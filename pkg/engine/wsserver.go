package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yang-bin-free/claude-phone/pkg/protocol"
	"github.com/yang-bin-free/claude-phone/pkg/provider"
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
	defer e.removeClient(cl)

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
			id, err := e.handleControl(cl, currentSession, env.Raw)
			if err != nil {
				_ = cl.writeJSON(errorFor(err))
				continue
			}
			currentSession = id
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
	if !e.deviceAuthorized(msg.DeviceToken) {
		return nil, errors.New("device token not authorized")
	}
	return &client{deviceID: msg.DeviceToken, conn: conn, mu: make(chan struct{}, 1)}, nil
}

func (e *Engine) addClient(cl *client) {
	e.mu.Lock()
	e.clients[cl.deviceID] = cl
	e.mu.Unlock()
}

func (e *Engine) removeClient(cl *client) {
	e.mu.Lock()
	if e.clients[cl.deviceID] != cl {
		e.mu.Unlock()
		return
	}
	delete(e.clients, cl.deviceID)
	for _, s := range e.manager.List() {
		s.Unsubscribe(cl.deviceID)
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

func (e *Engine) handleControl(cl *client, currentSession string, raw []byte) (string, error) {
	var msg protocol.ControlMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		return currentSession, err
	}
	switch msg.Action {
	case protocol.ActionCreateSession:
		return e.createSession(cl, msg)
	case protocol.ActionSelectSession, protocol.ActionJoinSession:
		s, ok := e.manager.Get(msg.SessionID)
		if !ok {
			return currentSession, session.ErrSessionNotFound
		}
		if s.Status == "dormant" {
			if err := e.resumeSession(s); err != nil {
				return currentSession, err
			}
		}
		s.Subscribe(cl.deviceID)
		return s.ID, nil
	case protocol.ActionLeaveSession:
		s, ok := e.manager.Get(msg.SessionID)
		if !ok {
			return currentSession, session.ErrSessionNotFound
		}
		if s.Owner == cl.deviceID {
			return currentSession, session.ErrSessionNotOwner
		}
		s.Unsubscribe(cl.deviceID)
		if currentSession == s.ID {
			return "", nil
		}
		return currentSession, nil
	case protocol.ActionStopSession:
		s, ok := e.manager.Get(msg.SessionID)
		if !ok {
			return currentSession, session.ErrSessionNotFound
		}
		if s.Owner != cl.deviceID {
			return currentSession, session.ErrSessionNotOwner
		}
		if err := e.stopSession(s); err != nil {
			return currentSession, err
		}
		if currentSession == s.ID {
			return "", nil
		}
		return currentSession, nil
	case protocol.ActionSetPermission:
		s, ok := e.manager.Get(msg.SessionID)
		if !ok {
			return currentSession, session.ErrSessionNotFound
		}
		if s.Owner != cl.deviceID {
			return currentSession, session.ErrSessionNotOwner
		}
		return currentSession, e.requestPermissionChange(s, msg.PermissionMode)
	case protocol.ActionListSessions:
		limit := msg.Limit
		if limit <= 0 {
			limit = len(e.manager.List())
		}
		return currentSession, cl.writeJSON(e.sessionList(limit, msg.Offset))
	case protocol.ActionListProjects:
		projects, err := e.projects.List()
		if err != nil {
			return currentSession, err
		}
		items := make([]protocol.ProjectInfo, 0, len(projects))
		for _, project := range projects {
			items = append(items, protocol.ProjectInfo{Name: project.Name, Path: project.Path, Permission: project.Permission})
		}
		return currentSession, cl.writeJSON(protocol.ProjectListMsg{Type: protocol.TypeProjectList, Projects: items})
	case protocol.ActionListTemplates:
		templates, err := e.templates.List()
		if err != nil {
			return currentSession, err
		}
		return currentSession, cl.writeJSON(protocol.TemplateListMsg{Type: protocol.TypeTemplateList, Templates: templates})
	case protocol.ActionListProviders:
		return currentSession, cl.writeJSON(e.providerList())
	case protocol.ActionLoadHistory:
		sessionID := msg.SessionID
		if sessionID == "" {
			sessionID = currentSession
		}
		if _, ok := e.manager.Get(sessionID); !ok {
			return currentSession, session.ErrSessionNotFound
		}
		messages, err := e.history.Load(sessionID, msg.Limit)
		if err != nil {
			return currentSession, err
		}
		return currentSession, cl.writeJSON(protocol.HistoryMsg{Type: "history", SessionID: sessionID, Messages: messages})
	case protocol.ActionPing, protocol.ActionWaitLonger:
		return currentSession, cl.writeJSON(protocol.PongMsg{Type: protocol.TypePong})
	case protocol.ActionCancel, protocol.ActionForceKill:
		s, ok := e.manager.Get(msg.SessionID)
		if !ok && currentSession != "" {
			s, ok = e.manager.Get(currentSession)
		}
		if !ok {
			return currentSession, session.ErrSessionNotFound
		}
		if s.Owner != cl.deviceID {
			return currentSession, session.ErrSessionNotOwner
		}
		if err := e.stopSession(s); err != nil {
			return currentSession, err
		}
		return "", nil
	default:
		return currentSession, errors.New("unsupported control action")
	}
}

func (e *Engine) stopSession(s *session.Session) error {
	e.mu.RLock()
	proc := e.procs[s.ID]
	e.mu.RUnlock()
	if proc != nil {
		if err := proc.Stop(); err != nil {
			return err
		}
	}
	s.SetStatus("stopped")
	b, err := json.Marshal(protocol.SessionStoppedMsg{Type: protocol.TypeSessionStopped, SessionID: s.ID})
	if err != nil {
		return err
	}
	s.Broadcast(b)
	e.mu.Lock()
	delete(e.procs, s.ID)
	delete(e.activity, s.ID)
	delete(e.healthState, s.ID)
	delete(e.queues, s.ID)
	delete(e.busy, s.ID)
	idle := len(e.procs) == 0
	e.mu.Unlock()
	if idle {
		_ = e.power.Release()
	}
	e.manager.Remove(s.ID)
	return nil
}

func (e *Engine) createSession(cl *client, msg protocol.ControlMsg) (string, error) {
	if msg.RequestID != "" {
		e.createMu.Lock()
		defer e.createMu.Unlock()
	}
	providerID := provider.NormalizeID(msg.Provider)
	adapter, ok := e.providers.Get(providerID)
	if !ok || !adapter.Descriptor().Available {
		return "", errProviderNotAvailable
	}
	runtime := e.runtimeConfig()
	cwd := msg.WorkingDir
	if cwd == "" {
		cwd = runtime.DefaultWorkingDir
	} else if cwd != runtime.DefaultWorkingDir {
		projects, err := e.projects.List()
		if err != nil {
			return "", err
		}
		allowed := false
		for _, project := range projects {
			if project.Path == cwd {
				allowed = true
				if msg.PermissionMode == "" && project.Permission != "" {
					msg.PermissionMode = project.Permission
				}
				break
			}
		}
		if !allowed {
			return "", errors.New("working directory is not an authorized project")
		}
	}
	permission := msg.PermissionMode
	if permission == "" {
		permission = runtime.DefaultPermission
	}
	if !provider.SupportsPermission(adapter.Descriptor(), permission) {
		return "", errInvalidPermission
	}
	name := msg.Name
	if name == "" {
		name = "New Session"
	}
	signature := strings.Join([]string{providerID, msg.Model, cwd, permission, name}, "\x00")
	if msg.RequestID != "" {
		key := cl.deviceID + "\x00" + msg.RequestID
		e.mu.RLock()
		prior, exists := e.createRequests[key]
		e.mu.RUnlock()
		if exists {
			if prior.signature != signature {
				return "", errors.New("requestId was already used with different session parameters")
			}
			return prior.message.SessionID, cl.writeJSON(prior.message)
		}
	}
	s, err := e.manager.Create(name, cwd, permission, cl.deviceID)
	if err != nil {
		return "", err
	}
	s.SetSender(e.send)
	s.Provider = providerID
	s.Model = msg.Model
	if err := e.history.CreateSession(s); err != nil {
		e.manager.Remove(s.ID)
		return "", err
	}

	proc := adapter.NewProcess(provider.SessionConfig{
		Cwd:          cwd,
		SessionID:    s.ID,
		Permission:   permission,
		AddDirs:      []string{cwd},
		AllowedTools: e.permissions.AllowedTools(),
	})
	proc.OnOutput(func(payload []byte) {
		e.handleProcOutput(s, proc, payload)
	})
	if err := proc.Start(); err != nil {
		e.manager.Remove(s.ID)
		return "", err
	}
	e.mu.Lock()
	e.procs[s.ID] = proc
	e.activity[s.ID] = time.Now()
	e.mu.Unlock()
	_ = e.power.Acquire()

	created := protocol.SessionCreatedMsg{
		Type: protocol.TypeSessionCreated, SessionID: s.ID, Name: s.Name, Cwd: s.Cwd,
		Provider: s.Provider, Model: s.Model, PermissionMode: s.Permission, RequestID: msg.RequestID,
	}
	if msg.RequestID != "" {
		e.mu.Lock()
		e.createRequests[cl.deviceID+"\x00"+msg.RequestID] = createResult{message: created, signature: signature}
		e.mu.Unlock()
	}
	if err := cl.writeJSON(created); err != nil {
		return "", err
	}
	return s.ID, nil
}

func (e *Engine) resumeSession(s *session.Session) error {
	e.resumeMu.Lock()
	defer e.resumeMu.Unlock()
	e.mu.RLock()
	existing := e.procs[s.ID]
	e.mu.RUnlock()
	if existing != nil {
		s.SetStatus("active")
		return nil
	}
	adapter, ok := e.providers.Get(s.Provider)
	if !ok || !adapter.Descriptor().Available {
		return errProviderNotAvailable
	}
	proc := adapter.NewProcess(provider.SessionConfig{
		Cwd: s.Cwd, SessionID: s.ID, Permission: s.Permission, Model: s.Model,
		AddDirs: []string{s.Cwd}, Resume: e.sessionExists(s.Cwd, s.ID),
		AllowedTools: e.permissions.AllowedTools(),
	})
	proc.OnOutput(func(payload []byte) {
		e.handleProcOutput(s, proc, payload)
	})
	if err := proc.Start(); err != nil {
		return err
	}
	e.mu.Lock()
	e.procs[s.ID] = proc
	e.activity[s.ID] = time.Now()
	e.mu.Unlock()
	_ = e.power.Acquire()
	s.SetStatus("active")
	return nil
}

func (e *Engine) handleText(sessionID string, raw []byte) error {
	if sessionID == "" {
		return session.ErrSessionNotFound
	}
	var msg protocol.TextMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		return err
	}
	if err := e.history.Append(sessionID, raw); err != nil {
		return err
	}
	e.mu.Lock()
	proc := e.procs[sessionID]
	if proc == nil {
		e.mu.Unlock()
		return session.ErrSessionNotFound
	}
	if e.busy[sessionID] {
		e.queueSeq++
		queued := queuedPrompt{ID: fmt.Sprintf("msg-%d", e.queueSeq), Content: msg.Content}
		e.queues[sessionID] = append(e.queues[sessionID], queued)
		position := len(e.queues[sessionID])
		e.mu.Unlock()
		if s, ok := e.manager.Get(sessionID); ok {
			payload, _ := json.Marshal(protocol.QueuedMsg{Type: protocol.TypeQueued, MsgID: queued.ID, Position: position})
			s.Broadcast(payload)
		}
		return nil
	}
	e.busy[sessionID] = true
	e.mu.Unlock()
	if err := proc.Send(msg.Content); err != nil {
		e.mu.Lock()
		e.busy[sessionID] = false
		e.mu.Unlock()
		return err
	}
	return nil
}

func (e *Engine) sessionList(limit, offset int) protocol.SessionListMsg {
	sessions := e.manager.List()
	if offset < 0 {
		offset = 0
	}
	if offset > len(sessions) {
		offset = len(sessions)
	}
	end := len(sessions)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	sessions = sessions[offset:end]
	infos := make([]protocol.SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		infos = append(infos, protocol.SessionInfo{
			SessionID:   s.ID,
			Name:        s.Name,
			Status:      s.Status,
			Owner:       s.Owner,
			Subscribers: s.Subscribers(),
			CreatedAt:   s.CreatedAt,
			Cwd:         s.Cwd,
			Provider:    provider.NormalizeID(s.Provider),
			Model:       s.Model,
			Permission:  s.Permission,
		})
	}
	return protocol.SessionListMsg{Type: protocol.TypeSessionList, Sessions: infos}
}

func (e *Engine) providerList() protocol.ProviderListMsg {
	descriptors := e.providers.Descriptors()
	items := make([]protocol.ProviderInfo, 0, len(descriptors))
	for _, descriptor := range descriptors {
		permissions := make([]protocol.ProviderPermission, 0, len(descriptor.Permissions))
		for _, option := range descriptor.Permissions {
			permissions = append(permissions, protocol.ProviderPermission{
				ID: option.ID, Label: option.Label, Description: option.Description,
				Dangerous: option.Dangerous, Mutable: option.Mutable,
			})
		}
		items = append(items, protocol.ProviderInfo{
			ID: descriptor.ID, Name: descriptor.Name, Available: descriptor.Available,
			UnavailableReason: descriptor.UnavailableReason, Permissions: permissions, Models: descriptor.Models,
		})
	}
	return protocol.ProviderListMsg{Type: protocol.TypeProviderList, Providers: items}
}

var (
	errProviderNotAvailable = errors.New("provider is not available")
	errInvalidPermission    = errors.New("permission mode is not supported by provider")
)

func errorFor(err error) protocol.ErrorMsg {
	switch {
	case errors.Is(err, session.ErrSessionLimit):
		return protocol.NewError(protocol.CodeSessionLimitReached, err.Error())
	case errors.Is(err, session.ErrSessionNotFound):
		return protocol.NewError(protocol.CodeSessionNotFound, err.Error())
	case errors.Is(err, session.ErrSessionNotOwner):
		return protocol.NewError(protocol.CodeSessionNotOwner, err.Error())
	case errors.Is(err, errProviderNotAvailable):
		return protocol.NewError(protocol.CodeProviderNotAvailable, err.Error())
	case errors.Is(err, errInvalidPermission):
		return protocol.NewError(protocol.CodeInvalidPermission, err.Error())
	default:
		return protocol.NewError("ENGINE_ERROR", err.Error())
	}
}
