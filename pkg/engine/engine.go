package engine

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/yang-bin-free/claude-phone/pkg/session"
)

type claudeProc interface {
	OnOutput(session.OutputFunc)
	Start() error
	Send(string) error
	Stop() error
}

type ClaudeFactory func(session.ClaudeConfig) claudeProc

type Engine struct {
	cfg     Config
	manager *session.Manager
	factory ClaudeFactory
	server  *http.Server

	mu          sync.RWMutex
	resumeMu    sync.Mutex
	clients     map[string]*client
	procs       map[string]claudeProc
	projects    *projectStore
	devices     *deviceStore
	history     *historyStore
	permissions *permissionStore
	startedAt   time.Time
	configMu    sync.RWMutex
	runtime     runtimeConfig
	stopWatch   chan struct{}
	closeOnce   sync.Once
	power       powerInhibitor
}

func New(cfg Config) *Engine {
	cfg = cfg.withDefaults()
	e := &Engine{
		cfg:         cfg,
		manager:     session.NewManager(session.ManagerConfig{MaxConcurrent: cfg.MaxConcurrentSession}),
		clients:     map[string]*client{},
		procs:       map[string]claudeProc{},
		projects:    newProjectStore(cfg.DataDir),
		devices:     newDeviceStore(cfg.DataDir),
		history:     newHistoryStore(cfg.DataDir),
		permissions: newPermissionStore(cfg.DataDir),
		startedAt:   time.Now(),
		runtime:     runtimeConfig{DefaultWorkingDir: cfg.DefaultWorkingDir, DefaultPermission: cfg.DefaultPermission, MaxConcurrentSessions: cfg.MaxConcurrentSession},
		stopWatch:   make(chan struct{}),
		power:       newPowerInhibitor(),
	}
	e.factory = func(c session.ClaudeConfig) claudeProc { return session.NewClaudeProc(c) }
	if persisted, err := e.history.Restore(); err == nil {
		for _, sess := range persisted {
			sess.SetSender(e.send)
			sess.Unsubscribe(sess.Owner)
			e.manager.Restore(sess)
		}
	}
	_ = e.reloadRuntimeConfig()
	go e.watchRuntimeConfig()
	return e
}

func (e *Engine) SetClaudeFactory(factory ClaudeFactory) {
	if factory != nil {
		e.factory = factory
	}
}

func (e *Engine) Manager() *session.Manager {
	return e.manager
}

func (e *Engine) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", e.HandleWS)
	mux.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(e.Status())
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}

func (e *Engine) ListenAndServe() error {
	e.server = &http.Server{Addr: e.cfg.Addr, Handler: e.Handler()}
	err := e.server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (e *Engine) Serve(ln net.Listener) error {
	e.server = &http.Server{Handler: e.Handler()}
	err := e.server.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (e *Engine) Close() error {
	e.closeOnce.Do(func() { close(e.stopWatch) })
	e.mu.Lock()
	for _, proc := range e.procs {
		_ = proc.Stop()
	}
	e.mu.Unlock()
	_ = e.power.Release()
	if e.server != nil {
		return e.server.Close()
	}
	return nil
}
