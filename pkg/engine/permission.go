package engine

import (
	"encoding/json"
	"errors"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
	"github.com/yang-bin-free/claude-phone/pkg/provider"
	"github.com/yang-bin-free/claude-phone/pkg/session"
)

func (e *Engine) requestPermissionChange(sess *session.Session, permission string) error {
	adapter, ok := e.providers.Get(sess.Provider)
	if !ok || !adapter.Descriptor().Available {
		return errProviderNotAvailable
	}
	if !provider.SupportsPermission(adapter.Descriptor(), permission) {
		return errInvalidPermission
	}
	if permission == sess.Permission {
		e.broadcastPermission(sess, permission, false)
		return nil
	}
	e.mu.Lock()
	if e.busy[sess.ID] {
		e.pendingPermissions[sess.ID] = permission
		e.mu.Unlock()
		e.broadcastPermission(sess, permission, true)
		return nil
	}
	e.mu.Unlock()
	return e.applyPermissionChange(sess, permission)
}

func (e *Engine) applyPermissionChange(sess *session.Session, permission string) error {
	adapter, ok := e.providers.Get(sess.Provider)
	if !ok || !adapter.Descriptor().Available {
		return errProviderNotAvailable
	}
	e.mu.RLock()
	oldProcess := e.procs[sess.ID]
	e.mu.RUnlock()
	oldPermission := sess.Permission
	if oldProcess != nil {
		_ = oldProcess.Stop()
	}
	newProcess := e.newSessionProcess(adapter, sess, permission)
	if err := newProcess.Start(); err != nil {
		if oldProcess != nil {
			rollback := e.newSessionProcess(adapter, sess, oldPermission)
			if rollbackErr := rollback.Start(); rollbackErr == nil {
				e.mu.Lock()
				e.procs[sess.ID] = rollback
				e.mu.Unlock()
			} else {
				return errors.Join(err, rollbackErr)
			}
		}
		return err
	}
	e.mu.Lock()
	e.procs[sess.ID] = newProcess
	e.mu.Unlock()
	sess.SetPermission(permission)
	if err := e.history.UpdateSession(sess); err != nil {
		return err
	}
	e.broadcastPermission(sess, permission, false)
	return nil
}

func (e *Engine) newSessionProcess(adapter provider.Adapter, sess *session.Session, permission string) provider.Process {
	proc := adapter.NewProcess(provider.SessionConfig{
		Cwd: sess.Cwd, SessionID: sess.ID, Permission: permission, Model: sess.Model,
		AddDirs: []string{sess.Cwd}, Resume: e.sessionExists(sess.Cwd, sess.ID),
		AllowedTools: e.permissions.AllowedTools(),
	})
	proc.OnOutput(func(payload []byte) { e.handleProcOutput(sess, proc, payload) })
	return proc
}

func (e *Engine) broadcastPermission(sess *session.Session, permission string, pending bool) {
	payload, err := json.Marshal(protocol.PermissionChangedMsg{
		Type: protocol.TypePermissionChanged, SessionID: sess.ID,
		PermissionMode: permission, Pending: pending,
	})
	if err == nil {
		sess.Broadcast(payload)
	}
}

func (e *Engine) takePendingPermission(sessionID string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	permission, ok := e.pendingPermissions[sessionID]
	if ok {
		delete(e.pendingPermissions, sessionID)
	}
	return permission, ok
}
