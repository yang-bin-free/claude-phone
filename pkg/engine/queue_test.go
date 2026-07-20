package engine

import (
	"encoding/json"
	"testing"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
	"github.com/yang-bin-free/claude-phone/pkg/provider"
	"github.com/yang-bin-free/claude-phone/pkg/session"
)

func TestBusySessionDequeuesPromptAfterDone(t *testing.T) {
	e := New(Config{DataDir: t.TempDir()})
	defer e.Close()
	e.manager = session.NewManager(session.ManagerConfig{IDFunc: func() string { return "sess-q" }})
	sess, err := e.manager.Create("queue", ".", "default", "owner")
	if err != nil {
		t.Fatal(err)
	}
	proc := &stubClaudeProc{}
	e.procs[sess.ID] = proc
	e.busy[sess.ID] = true

	if _, err := e.handleText(sess.ID, []byte(`{"type":"text","content":"second"}`)); err != nil {
		t.Fatal(err)
	}
	if len(e.queues[sess.ID]) != 1 || len(proc.sent) != 0 {
		t.Fatalf("queue=%v sent=%v", e.queues[sess.ID], proc.sent)
	}
	e.handleProcOutput(sess, proc, []byte(`{"type":"done"}`))
	if len(e.queues[sess.ID]) != 0 || len(proc.sent) != 1 || proc.sent[0] != "second" {
		t.Fatalf("queue=%v sent=%v", e.queues[sess.ID], proc.sent)
	}
}

func TestHandleProcOutputUsesSelectedProviderTranslator(t *testing.T) {
	e := New(Config{DataDir: t.TempDir()})
	defer e.Close()
	e.providers = provider.NewRegistry(provider.NewCodexAdapter("codex", true, ""))
	sess := session.NewSession("codex-local", "Codex", ".", "owner")
	sess.Provider = provider.CodexID
	e.manager.Restore(sess)
	proc := &stubClaudeProc{}
	var got []byte
	sess.SetSender(func(_ string, payload []byte) { got = append([]byte(nil), payload...) })

	e.handleProcOutput(sess, proc, []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"hello from Codex"}}`))
	var message protocol.TokenMsg
	if err := json.Unmarshal(got, &message); err != nil {
		t.Fatalf("translated payload=%q err=%v", got, err)
	}
	if message.Type != protocol.TypeToken || message.Content != "hello from Codex" {
		t.Fatalf("message=%+v", message)
	}
}
