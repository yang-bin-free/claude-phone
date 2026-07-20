package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/yang-bin-free/claude-phone/pkg/desktop"
	"github.com/yang-bin-free/claude-phone/pkg/engine"
	"github.com/yang-bin-free/claude-phone/pkg/protocol"
)

type managedEngine interface {
	Handler() http.Handler
	AdminHandler(string) http.Handler
	Status() engine.StatusReport
	AddProject(string) (protocol.ProjectInfo, error)
	Close() error
}

type appConfig struct {
	DesktopAddr       string
	ClaudeBin         string
	DefaultWorkdir    string
	DefaultPermission string
	DataDir           string
	AdminToken        string
}

type appDependencies struct {
	resolveClaude func(string) (string, error)
	detectVersion func(string) (string, error)
	newEngine     func(engine.Config) managedEngine
	listen        func(string, string) (net.Listener, error)
	serve         func(context.Context, net.Listener, http.Handler) error
}

type application struct {
	cfg  appConfig
	deps appDependencies

	ctx    context.Context
	cancel context.CancelFunc

	lifecycleMu sync.Mutex
	mu          sync.RWMutex
	engine      managedEngine
	status      desktop.AppStatus
	handler     http.Handler
	listener    net.Listener
	serverDone  chan error
	menuStates  chan desktop.MenuState
	closeOnce   sync.Once
	closeErr    error
}

func newApplication(parent context.Context, cfg appConfig, deps appDependencies) *application {
	if deps.resolveClaude == nil {
		deps.resolveClaude = desktop.ResolveClaudeBinary
	}
	if deps.detectVersion == nil {
		deps.detectVersion = engine.DetectClaudeVersion
	}
	if deps.newEngine == nil {
		deps.newEngine = func(cfg engine.Config) managedEngine { return engine.New(cfg) }
	}
	if deps.listen == nil {
		deps.listen = net.Listen
	}
	if deps.serve == nil {
		deps.serve = desktop.Serve
	}
	ctx, cancel := context.WithCancel(parent)
	app := &application{cfg: cfg, deps: deps, ctx: ctx, cancel: cancel, serverDone: make(chan error, 1), menuStates: make(chan desktop.MenuState, 1)}
	app.handler = desktop.NewHandler(desktop.HandlerOptions{
		EngineHandler: app.engineHandler,
		AdminHandler:  app.adminHandler,
		Status:        app.Status,
		AddProject:    app.addProject,
		AdminToken:    cfg.AdminToken,
	})
	return app
}

func (a *application) Start() error {
	a.lifecycleMu.Lock()
	if a.listener != nil {
		a.lifecycleMu.Unlock()
		return errors.New("desktop application already started")
	}
	listener, err := a.deps.listen("tcp", a.cfg.DesktopAddr)
	if err != nil {
		a.lifecycleMu.Unlock()
		return fmt.Errorf("listen for desktop UI: %w", err)
	}
	a.listener = listener
	a.lifecycleMu.Unlock()

	go func() { a.serverDone <- a.deps.serve(a.ctx, listener, a.handler) }()
	_ = a.Resume()
	go a.publishMenuState()
	return nil
}

func (a *application) Resume() error {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()

	a.mu.RLock()
	alreadyReady := a.engine != nil
	a.mu.RUnlock()
	if alreadyReady {
		return nil
	}

	bin, err := a.deps.resolveClaude(a.cfg.ClaudeBin)
	if err != nil {
		a.setUnavailable(err, false)
		a.emitMenuState()
		return err
	}
	version, err := a.deps.detectVersion(bin)
	if err != nil {
		a.setUnavailable(fmt.Errorf("Claude CLI check failed: %w", err), false)
		a.emitMenuState()
		return err
	}
	instance := a.deps.newEngine(engine.Config{
		Addr:              a.cfg.DesktopAddr,
		ClaudeBin:         bin,
		ClaudeVersion:     version,
		DefaultWorkingDir: a.cfg.DefaultWorkdir,
		DefaultPermission: a.cfg.DefaultPermission,
		DataDir:           a.cfg.DataDir,
		DeviceTokens:      map[string]string{"desktop-" + a.cfg.AdminToken: "Mac"},
	})
	a.mu.Lock()
	a.engine = instance
	a.status = desktop.AppStatus{Ready: true, ClaudeBin: bin, ClaudeVersion: version}
	a.mu.Unlock()
	a.emitMenuState()
	return nil
}

func (a *application) Pause() error {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	a.mu.Lock()
	instance := a.engine
	a.engine = nil
	a.status = desktop.AppStatus{Paused: true}
	a.mu.Unlock()
	if instance != nil {
		err := instance.Close()
		a.emitMenuState()
		return err
	}
	a.emitMenuState()
	return nil
}

func (a *application) Close() error {
	a.closeOnce.Do(func() {
		a.cancel()
		a.mu.Lock()
		instance := a.engine
		a.engine = nil
		a.mu.Unlock()
		if instance != nil {
			a.closeErr = instance.Close()
		}
		if a.listener != nil {
			if err := <-a.serverDone; a.closeErr == nil {
				a.closeErr = err
			}
		}
	})
	return a.closeErr
}

func (a *application) Handler() http.Handler { return a.handler }

func (a *application) MenuStates() <-chan desktop.MenuState { return a.menuStates }

func (a *application) BaseURL() string {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	if a.listener == nil {
		return ""
	}
	return "http://" + a.listener.Addr().String() + "/"
}

func (a *application) Status() desktop.AppStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

func (a *application) EngineStatus() engine.StatusReport {
	a.mu.RLock()
	instance := a.engine
	a.mu.RUnlock()
	if instance == nil {
		return engine.StatusReport{}
	}
	return instance.Status()
}

func (a *application) engineHandler() http.Handler {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.engine == nil {
		return nil
	}
	return a.engine.Handler()
}

func (a *application) adminHandler() http.Handler {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.engine == nil {
		return nil
	}
	return a.engine.AdminHandler(a.cfg.AdminToken)
}

func (a *application) addProject(path string) (protocol.ProjectInfo, error) {
	a.mu.RLock()
	instance := a.engine
	a.mu.RUnlock()
	if instance == nil {
		return protocol.ProjectInfo{}, errors.New("desktop engine unavailable")
	}
	return instance.AddProject(path)
}

func (a *application) setUnavailable(err error, paused bool) {
	a.mu.Lock()
	a.status = desktop.AppStatus{Paused: paused, Error: err.Error()}
	a.mu.Unlock()
}

func (a *application) publishMenuState() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	a.emitMenuState()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.emitMenuState()
		}
	}
}

func (a *application) emitMenuState() {
	status := a.Status()
	state := desktop.MenuState{
		Ready: status.Ready, Paused: status.Paused, StatusText: status.Error,
		Autostart: desktop.AutostartEnabled(),
	}
	if status.Ready {
		report := a.EngineStatus()
		state.Devices = len(report.ConnectedDevices)
		state.Sessions = len(report.Sessions)
	}
	select {
	case a.menuStates <- state:
	default:
		select {
		case <-a.menuStates:
		default:
		}
		select {
		case a.menuStates <- state:
		default:
		}
	}
}
