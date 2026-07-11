package protocol

import (
	"encoding/json"
	"testing"
)

func TestParseEnvelope_Auth(t *testing.T) {
	raw := []byte(`{"type":"auth","deviceToken":"dt_abc","deviceName":"Pixel 8"}`)
	env, err := ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope error: %v", err)
	}
	if env.Type != TypeAuth {
		t.Fatalf("got type %q, want %q", env.Type, TypeAuth)
	}
	var a AuthMsg
	if err := json.Unmarshal(env.Raw, &a); err != nil {
		t.Fatalf("unmarshal AuthMsg: %v", err)
	}
	if a.DeviceToken != "dt_abc" || a.DeviceName != "Pixel 8" {
		t.Fatalf("bad AuthMsg: %+v", a)
	}
}

func TestParseControl_Action(t *testing.T) {
	raw := []byte(`{"type":"control","action":"create_session","name":"车险联调","workingDir":"/p","permissionMode":"bypassPermissions"}`)
	env, err := ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if env.Type != TypeControl {
		t.Fatalf("type=%q", env.Type)
	}
	var c ControlMsg
	if err := json.Unmarshal(env.Raw, &c); err != nil {
		t.Fatalf("unmarshal control: %v", err)
	}
	if c.Action != ActionCreateSession || c.Name != "车险联调" || c.WorkingDir != "/p" {
		t.Fatalf("bad control: %+v", c)
	}
}

func TestErrorMsg_Marshal(t *testing.T) {
	b, err := json.Marshal(NewError(CodeSessionNotFound, "会话不存在"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"type":"error","code":"SESSION_NOT_FOUND","message":"会话不存在"}`
	if string(b) != want {
		t.Fatalf("got %s want %s", b, want)
	}
}

func TestParseEnvelope_InvalidJSON(t *testing.T) {
	if _, err := ParseEnvelope([]byte("not json")); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseEnvelope_MissingType(t *testing.T) {
	env, err := ParseEnvelope([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Type != "" {
		t.Fatalf("want empty type, got %q", env.Type)
	}
}

func TestHelloMsg_RoundTrip(t *testing.T) {
	b, err := json.Marshal(HelloMsg{
		Type: TypeHello, AgentVersion: "0.1.0",
		ClaudeVersion: "2.1.186", ProtocolVersion: ProtocolVersion,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"type":"hello","agentVersion":"0.1.0","claudeVersion":"2.1.186","protocolVersion":"1"}`
	if string(b) != want {
		t.Fatalf("got %s want %s", b, want)
	}
}
