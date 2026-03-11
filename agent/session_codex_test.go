package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCodexMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	lines := `{"timestamp":"2026-03-10T19:30:47.691Z","type":"session_meta","payload":{"id":"abc123"}}
{"timestamp":"2026-03-10T19:30:47.756Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"fix the bug"}]}}
{"timestamp":"2026-03-10T19:31:00.000Z","type":"event_msg","payload":{"type":"agent_message","agent_message":"I found the issue and fixed it."}}
{"timestamp":"2026-03-10T19:31:05.000Z","type":"response_item","payload":{"type":"function_call","name":"apply_patch","arguments":"{}"}}
`
	os.WriteFile(path, []byte(lines), 0644)

	msgs, err := ParseCodexMessages(path, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d: %+v", len(msgs), msgs)
	}

	if msgs[0].Role != "user" || msgs[0].Content != "fix the bug" {
		t.Errorf("msg[0] = %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "I found the issue and fixed it." {
		t.Errorf("msg[1] = %+v", msgs[1])
	}
	if !msgs[2].IsToolUse || msgs[2].Content != "[tool: apply_patch]" {
		t.Errorf("msg[2] should be tool_use: %+v", msgs[2])
	}
}

func TestParseCodexMessages_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(path, []byte(""), 0644)

	msgs, err := ParseCodexMessages(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages from empty file, got %d", len(msgs))
	}
}

func TestCodexSessionState_AgentMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	lines := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}
{"type":"event_msg","payload":{"type":"agent_message","agent_message":"hi"}}
`
	os.WriteFile(path, []byte(lines), 0644)

	state, err := CodexSessionState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.LastEntryType != "assistant" {
		t.Errorf("LastEntryType = %q, want %q", state.LastEntryType, "assistant")
	}
	if state.PendingToolUse {
		t.Error("PendingToolUse should be false for agent_message")
	}
}

func TestCodexSessionState_PendingFunctionCall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	lines := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"fix it"}]}}
{"type":"response_item","payload":{"type":"function_call","name":"apply_patch","arguments":"{}"}}
`
	os.WriteFile(path, []byte(lines), 0644)

	state, err := CodexSessionState(path)
	if err != nil {
		t.Fatal(err)
	}
	if !state.PendingToolUse {
		t.Error("PendingToolUse should be true when last entry is function_call")
	}
}
