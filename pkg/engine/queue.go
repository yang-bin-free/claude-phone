package engine

import (
	"encoding/json"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
	"github.com/yang-bin-free/claude-phone/pkg/provider"
	"github.com/yang-bin-free/claude-phone/pkg/session"
)

type queuedPrompt struct {
	ID      string
	Content string
}

func (e *Engine) handleProcOutput(sess *session.Session, proc claudeProc, payload []byte) {
	if currentSession, ok := e.manager.Get(sess.ID); !ok || currentSession != sess {
		return
	}
	e.mu.RLock()
	currentProcess, registered := e.procs[sess.ID]
	e.mu.RUnlock()
	if registered && currentProcess != proc {
		return
	}
	e.recordActivity(sess.ID)
	if identity, ok := proc.(provider.SessionIdentity); ok {
		previousID := sess.ProviderSessionIdentity()
		providerID := identity.ProviderSessionID()
		if providerID != "" && providerID != previousID && sess.CompareAndSwapProviderSessionID(previousID, providerID) {
			if err := e.updateSession(sess); err != nil {
				sess.CompareAndSwapProviderSessionID(providerID, previousID)
				problem, _ := json.Marshal(protocol.NewError("ENGINE_ERROR", "无法保存引擎会话标识: "+err.Error()))
				sess.Broadcast(problem)
			}
		}
	}
	translated := e.translateOutput(sess, payload)
	for _, message := range translated {
		_ = e.history.Append(sess.ID, message)
		sess.Broadcast(message)
		var head struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(message, &head) == nil && head.Type == protocol.TypeDone {
			if permission, ok := e.takePendingPermission(sess.ID); ok {
				go func() {
					if err := e.applyPermissionChange(sess, permission); err != nil {
						problem, _ := json.Marshal(protocol.NewError("ENGINE_ERROR", err.Error()))
						sess.Broadcast(problem)
					}
					e.mu.RLock()
					nextProcess := e.procs[sess.ID]
					e.mu.RUnlock()
					e.advanceQueue(sess, nextProcess)
				}()
			} else {
				e.advanceQueue(sess, proc)
			}
		}
	}
}

func (e *Engine) translateOutput(sess *session.Session, payload []byte) [][]byte {
	if adapter, ok := e.providers.Get(sess.Provider); ok {
		if translator, ok := adapter.(provider.OutputTranslator); ok {
			return translator.TranslateOutput(payload)
		}
	}
	return translateClaudeOutput(payload)
}

func (e *Engine) advanceQueue(sess *session.Session, proc claudeProc) {
	e.mu.Lock()
	queue := e.queues[sess.ID]
	if len(queue) == 0 {
		e.busy[sess.ID] = false
		e.mu.Unlock()
		return
	}
	next := queue[0]
	e.queues[sess.ID] = queue[1:]
	e.mu.Unlock()
	notice, _ := json.Marshal(protocol.DequeuedMsg{Type: protocol.TypeDequeued, MsgID: next.ID})
	sess.Broadcast(notice)
	if err := proc.Send(next.Content); err != nil {
		e.mu.Lock()
		e.busy[sess.ID] = false
		e.mu.Unlock()
		problem, _ := json.Marshal(protocol.NewError("ENGINE_ERROR", err.Error()))
		sess.Broadcast(problem)
	}
}
