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
	if out == "(no messages)" {
		t.Error("expected messages, got empty")
	}
	if !strings.Contains(out, "[you] hello") {
		t.Errorf("missing user message in: %s", out)
	}
	if !strings.Contains(out, "[claude] hi there") {
		t.Errorf("missing assistant message in: %s", out)
	}
}

func TestFormatMessages_CodexLabel(t *testing.T) {
	msgs := []agent.Message{
		{Role: "assistant", Content: "done"},
	}
	out := formatMessages(msgs, agent.TypeCodex)
	if !strings.Contains(out, "[codex] done") {
		t.Errorf("expected [codex] label in: %s", out)
	}
}

func TestFormatMessages_Truncation(t *testing.T) {
	long := strings.Repeat("a", 600)
	msgs := []agent.Message{
		{Role: "user", Content: long},
	}
	out := formatMessages(msgs, agent.TypeClaude)
	if len(out) > 520 {
		t.Errorf("message not truncated, len = %d", len(out))
	}
}

func TestFormatMessages_Empty(t *testing.T) {
	out := formatMessages(nil, agent.TypeClaude)
	if out != "(no messages)" {
		t.Errorf("expected '(no messages)', got %q", out)
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
	if a := findAgent(agents, "nope"); a != nil {
		t.Error("should not find nope")
	}
}

func TestHandleCommand_Unknown(t *testing.T) {
	b := &Bot{}
	reply := b.handleCommand("blah")
	if !strings.Contains(reply, "Unknown command") {
		t.Errorf("expected unknown command response, got: %s", reply)
	}
}
