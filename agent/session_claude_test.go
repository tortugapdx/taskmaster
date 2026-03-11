package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindClaudeSession(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, "-Users-test-myproject")
	os.MkdirAll(projDir, 0755)

	old := filepath.Join(projDir, "old-session-id.jsonl")
	os.WriteFile(old, []byte(`{"type":"user","message":{"role":"user","content":"hello"}}`+"\n"), 0644)
	pastTime := mustParseTime(t, "2024-01-01T00:00:00Z")
	os.Chtimes(old, pastTime, pastTime)

	newer := filepath.Join(projDir, "new-session-id.jsonl")
	os.WriteFile(newer, []byte(`{"type":"user","message":{"role":"user","content":"world"}}`+"\n"), 0644)

	sessionID, sessionPath, err := FindClaudeSession(dir, "/Users/test/myproject")
	if err != nil {
		t.Fatal(err)
	}
	if sessionID != "new-session-id" {
		t.Errorf("got session %q, want %q", sessionID, "new-session-id")
	}
	if sessionPath != newer {
		t.Errorf("got path %q, want %q", sessionPath, newer)
	}
}

func TestParseClaudeMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	lines := `{"type":"user","message":{"role":"user","content":"hello there"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"hi back"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"let me think"},{"type":"tool_use","name":"Read","input":{}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"x","content":"file data"}]}}
`
	os.WriteFile(path, []byte(lines), 0644)

	msgs, err := ParseClaudeMessages(path, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d: %+v", len(msgs), msgs)
	}

	if msgs[0].Role != "user" || msgs[0].Content != "hello there" {
		t.Errorf("msg[0] = %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi back" {
		t.Errorf("msg[1] = %+v", msgs[1])
	}
	if !msgs[2].IsThinking {
		t.Errorf("msg[2] should be thinking: %+v", msgs[2])
	}
	if !msgs[3].IsToolUse || msgs[3].Content != "[tool: Read]" {
		t.Errorf("msg[3] should be tool_use: %+v", msgs[3])
	}
	if !msgs[4].IsToolUse {
		t.Errorf("msg[4] should be tool_result: %+v", msgs[4])
	}
}

func TestParseClaudeMessages_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(path, []byte(""), 0644)

	msgs, err := ParseClaudeMessages(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages from empty file, got %d", len(msgs))
	}
}

func TestEncodeCWD(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/Users/jpoz/Developer/ace", "-Users-jpoz-Developer-ace"},
		{"/", "-"},
	}
	for _, tt := range tests {
		got := encodeCWD(tt.input)
		if got != tt.want {
			t.Errorf("encodeCWD(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClaudeLastEntryType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	lines := `{"type":"user","message":{"role":"user","content":"hello"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"hi"}]}}
`
	os.WriteFile(path, []byte(lines), 0644)

	entryType, err := ClaudeLastEntryType(path)
	if err != nil {
		t.Fatal(err)
	}
	if entryType != "assistant" {
		t.Errorf("got %q, want %q", entryType, "assistant")
	}
}

func mustParseTime(t *testing.T, s string) (tm time.Time) {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}
