package engine

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/yang-bin-free/claude-phone/pkg/protocol"
	"github.com/yang-bin-free/claude-phone/pkg/session"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

type client struct {
	deviceID string
	conn     *websocket.Conn
	mu       chan struct{}
}

func (c *client) writeJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.write(b)
}

func (c *client) write(b []byte) error {
	c.mu <- struct{}{}
	defer func() { <-c.mu }()
	return c.conn.WriteMessage(websocket.TextMessage, b)
}

func (e *Engine) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	cl, err := e.authenticate(conn)
	if err != nil {
		_ = conn.WriteJSON(protocol.NewError(protocol.CodeDeviceNotAuthorized, err.Error()))
		return
	}
	e.addClient(cl)
	defer e.removeClient(cl.deviceID)

	_ = cl.writeJSON(protocol.HelloMsg{
		Type:            protocol.TypeHello,
		AgentVersion:    e.cfg.AgentVersion,
		ClaudeVersion:   e.cfg.ClaudeVersion,
		ProtocolVersion: protocol.ProtocolVersion,
	})

	var currentSession string
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		env, err := protocol.ParseEnvelope(payload)
		if err != nil {
			_ = cl.writeJSON(protocol.NewError("BAD_REQUEST", err.Error()))
			continue
		}
		switch env.Type {
		case protocol.TypeControl:
			id, err := e.handleControl(cl, env.Raw)
			if err != nil {
				_ = cl.writeJSON(errorFor(err))
				continue
			}
			if id != "" {
				currentSession = id
			}
		case protocol.TypeText, protocol.TypeVoice:
			if err := e.handleText(currentSession, env.Raw); err != nil {
				_ = cl.writeJSON(errorFor(err))
			}
		default:
			_ = cl.writeJSON(protocol.NewError("UNKNOWN_MESSAGE", "unknown message type"))
		}
	}
}

func (e *Engine) authenticate(conn *websocket.Conn) (*client, error) {
	_, payload, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	env, err := protocol.ParseEnvelope(payload)
	if err != nil {
		return nil, err
	}
	if env.Type != protocol.TypeAuth {
		return nil, errors.New("first message must be auth")
	}
	var msg protocol.AuthMsg
	if err := json.Unmarshal(env.Raw, &msg); err != nil {
		return nil, err
	}
	if msg.DeviceToken == "" {
		return nil, errors.New("missing device token")
	}
	if len(e.cfg.DeviceTokens) > 0 {
		if _, ok := e.cfg.DeviceTokens[msg.DeviceToken]; !ok {
			return nil, errors.New("device token not authorized")
		}
	}
	return &client{deviceID: msg.DeviceToken, conn: conn, mu: make(chan struct{}, 1)}, nil
}

func (e *Engine) addClient(cl *client) {
	e.mu.Lock()
	e.clients[cl.deviceID] = cl
	e.mu.Unlock()
}

func (e *Engine) removeClient(deviceID string) {
	e.mu.Lock()
	delete(e.clients, deviceID)
	for _, s := range e.manager.List() {
		s.Unsubscribe(deviceID)
	}
	e.mu.Unlock()
}

func (e *Engine) send(deviceID string, payload []byte) {
	e.mu.RLock()
	cl := e.clients[deviceID]
	e.mu.RUnlock()
	if cl != nil {
		_ = cl.write(payload)
	}
}

func (e *Engine) handleControl(cl *client, raw []byte) (string, error) {
	var msg protocol.ControlMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		return "", err
	}
	switch msg.Action {
	case protocol.ActionCreateSession:
		return e.createSession(cl, msg)
	case protocol.ActionSelectSession, protocol.ActionJoinSession:
		s, ok := e.manager.Get(msg.SessionID)
		if !ok {
			return "", session.ErrSessionNotFound
		}
		s.Subscribe(cl.deviceID)
		return s.ID, nil
	case protocol.ActionListSessions:
		return "", cl.writeJSON(e.sessionList())
	default:
		return "", nil
	}
}

func (e *Engine) createSession(cl *client, msg protocol.ControlMsg) (string, error) {
	cwd := msg.WorkingDir
	if cwd == "" {
		cwd = e.cfg.DefaultWorkingDir
	}
	permission := msg.PermissionMode
	if permission == "" {
		permission = e.cfg.DefaultPermission
	}
	name := msg.Name
	if name == "" {
		name = "Claude Session"
	}
	s, err := e.manager.Create(name, cwd, permission, cl.deviceID)
	if err != nil {
		return "", err
	}
	s.SetSender(e.send)

	proc := e.factory(session.ClaudeConfig{
		Bin:        e.cfg.ClaudeBin,
		Cwd:        cwd,
		SessionID:  s.ID,
		Permission: permission,
		AddDirs:    []string{cwd},
	})
	proc.OnOutput(func(payload []byte) { s.Broadcast(payload) })
	if err := proc.Start(); err != nil {
		e.manager.Remove(s.ID)
		return "", err
	}
	e.mu.Lock()
	e.procs[s.ID] = proc
	e.mu.Unlock()

	if err := cl.writeJSON(protocol.SessionCreatedMsg{
		Type:      protocol.TypeSessionCreated,
		SessionID: s.ID,
		Name:      s.Name,
		Cwd:       s.Cwd,
	}); err != nil {
		return "", err
	}
	return s.ID, nil
}

func (e *Engine) handleText(sessionID string, raw []byte) error {
	if sessionID == "" {
		return session.ErrSessionNotFound
	}
	var msg protocol.TextMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		return err
	}
	e.mu.RLock()
	proc := e.procs[sessionID]
	e.mu.RUnlock()
	if proc == nil {
		return session.ErrSessionNotFound
	}
	return proc.Send(msg.Content)
}

func (e *Engine) sessionList() protocol.SessionListMsg {
	sessions := e.manager.List()
	infos := make([]protocol.SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		infos = append(infos, protocol.SessionInfo{
			SessionID:   s.ID,
			Name:        s.Name,
			Status:      s.Status,
			Owner:       s.Owner,
			Subscribers: s.Subscribers(),
			CreatedAt:   s.CreatedAt,
		})
	}
	return protocol.SessionListMsg{Type: protocol.TypeSessionList, Sessions: infos}
}

func errorFor(err error) protocol.ErrorMsg {
	switch {
	case errors.Is(err, session.ErrSessionLimit):
		return protocol.NewError(protocol.CodeSessionLimitReached, err.Error())
	case errors.Is(err, session.ErrSessionNotFound):
		return protocol.NewError(protocol.CodeSessionNotFound, err.Error())
	default:
		return protocol.NewError("ENGINE_ERROR", err.Error())
	}
}
