package bot

import (
	"strings"
	"testing"

	"github.com/jpoz/taskmaster/agent"
)

func TestFormatMessages_NonVerbose(t *testing.T) {
	msgs := []agent.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	out := formatMessages(msgs, agent.TypeClaude)
	if strings.Contains(out, "No messages") {
		t.Error("expected messages, got empty")
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("missing user message in: %s", out)
	}
	if !strings.Contains(out, "hi there") {
		t.Errorf("missing assistant message in: %s", out)
	}
}

func TestFormatMessages_CodexLabel(t *testing.T) {
	msgs := []agent.Message{
		{Role: "assistant", Content: "done"},
	}
	out := formatMessages(msgs, agent.TypeCodex)
	if !strings.Contains(out, "codex") {
		t.Errorf("expected codex label in: %s", out)
	}
}

func TestFormatMessages_Truncation(t *testing.T) {
	long := strings.Repeat("a", 600)
	msgs := []agent.Message{
		{Role: "user", Content: long},
	}
	out := formatMessages(msgs, agent.TypeClaude)
	if strings.Contains(out, strings.Repeat("a", 600)) {
		t.Error("message not truncated")
	}
	if !strings.Contains(out, "...") {
		t.Error("expected truncation marker")
	}
}

func TestFormatMessages_Empty(t *testing.T) {
	out := formatMessages(nil, agent.TypeClaude)
	if !strings.Contains(out, "No messages") {
		t.Errorf("expected 'No messages' indicator, got %q", out)
	}
}

func TestFindAgent(t *testing.T) {
	agents := []agent.Agent{
		{Name: "ace-fox"},
		{Name: "venture-owl"},
	}
	if a := findAgent(agents, "ace-fox"); a == nil || a.Name != "ace-fox" {
		t.Error("should find ace-fox")
	}
	if a := findAgent(agents, "ace-fo"); a == nil || a.Name != "ace-fox" {
		t.Error("should find ace-fox by prefix")
	}
	if a := findAgent(agents, "nope"); a != nil {
		t.Error("should not find nope")
	}
}

func TestHandleCommand_Unknown(t *testing.T) {
	b := &Bot{states: newStateManager()}
	reply := b.handleCommand("blah")
	if !strings.Contains(reply.Text, "Unknown command") {
		t.Errorf("expected unknown command response, got: %s", reply.Text)
	}
}

func TestHandleCommand_Start(t *testing.T) {
	b := &Bot{states: newStateManager()}
	reply := b.handleCommand("/start")
	if !strings.Contains(reply.Text, "Taskmaster") {
		t.Errorf("expected welcome text, got: %s", reply.Text)
	}
}

func TestHandleCommand_TailNoPicker(t *testing.T) {
	b := &Bot{states: newStateManager()}
	reply := b.handleCommand("tail")
	if strings.Contains(reply.Text, "Usage") {
		t.Error("tail with no args should show picker, not usage")
	}
}

func TestHandleCommand_EchoNoPicker(t *testing.T) {
	b := &Bot{states: newStateManager()}
	reply := b.handleCommand("echo")
	if strings.Contains(reply.Text, "Usage") {
		t.Error("echo with no args should show picker, not usage")
	}
}

func TestStatusToIcon(t *testing.T) {
	tests := []struct {
		status agent.Status
		icon   string
	}{
		{agent.StatusWorking, "🟢"},
		{agent.StatusWaiting, "🟡"},
		{agent.StatusIdle, "⚪"},
		{agent.StatusUnknown, "🔴"},
	}
	for _, tt := range tests {
		got := statusToIcon(tt.status)
		if got != tt.icon {
			t.Errorf("statusToIcon(%q) = %q, want %q", tt.status, got, tt.icon)
		}
	}
}
