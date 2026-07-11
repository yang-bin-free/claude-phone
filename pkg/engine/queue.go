package engine

import (
	"encoding/json"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
	"github.com/yang-bin-free/claude-phone/pkg/session"
)

type queuedPrompt struct {
	ID      string
	Content string
}

func (e *Engine) handleProcOutput(sess *session.Session, proc claudeProc, payload []byte) {
	e.recordActivity(sess.ID)
	_ = e.history.Append(sess.ID, payload)
	sess.Broadcast(payload)
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(payload, &head) != nil || head.Type != protocol.TypeDone {
		return
	}
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
