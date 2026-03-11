# Taskmaster Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI that runs a Telegram bot to remotely monitor and control active Claude Code and Codex agent sessions.

**Architecture:** Stateless agent discovery via `ps` + `lsof`, with JSONL/SQLite session parsing for conversation data. Telegram long-polling bot with three commands (`ls`, `tail`, `echo`). TTY injection for sending input to agents.

**Tech Stack:** Go 1.25, `github.com/go-telegram-bot-api/telegram-bot-api/v5`, `modernc.org/sqlite`

---

## File Structure

```
taskmaster/
├── main.go                  # CLI entrypoint: load config, start bot
├── go.mod
├── config/
│   └── config.go            # Load/save ~/.config/taskmaster/config.json, interactive setup
├── agent/
│   ├── agent.go             # Agent struct, status detection, AgentType enum
│   ├── discovery.go         # ps scanning, lsof CWD extraction, orchestration
│   ├── naming.go            # Deterministic <dir>-<word> ID from session ID hash
│   ├── session_claude.go    # Claude Code JSONL parsing (find session file, parse messages)
│   ├── session_codex.go     # Codex SQLite + JSONL parsing (query threads, parse messages)
│   └── session.go           # Shared Message type and session interface
├── bot/
│   └── bot.go               # Telegram long-polling, command routing (ls/tail/echo)
├── tty/
│   └── writer.go            # TTY device writing with safety checks
└── wordlist/
    └── words.go             # Embedded 256-word list for naming
```

---

## Chunk 1: Foundation — Go module, config, wordlist, naming

### Task 1: Go module initialization

**Files:**
- Create: `go.mod`
- Create: `main.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/jpoz/Developer/taskmaster
go mod init github.com/jpoz/taskmaster
```

- [ ] **Step 2: Create minimal main.go**

```go
package main

import "fmt"

func main() {
	fmt.Println("taskmaster")
}
```

- [ ] **Step 3: Verify it compiles**

```bash
go build -o taskmaster .
./taskmaster
```
Expected: prints "taskmaster"

- [ ] **Step 4: Commit**

```bash
git add go.mod main.go
git commit -m "init: go module and minimal main"
```

---

### Task 2: Config loading and interactive setup

**Files:**
- Create: `config/config.go`
- Create: `config/config_test.go`

- [ ] **Step 1: Write failing test for config loading**

`config/config_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"telegram_bot_token":"tok123","telegram_user_id":42}`), 0600)

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramBotToken != "tok123" {
		t.Errorf("got token %q, want %q", cfg.TelegramBotToken, "tok123")
	}
	if cfg.TelegramUserID != 42 {
		t.Errorf("got user ID %d, want %d", cfg.TelegramUserID, 42)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/config.json")
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestConfig_IsComplete(t *testing.T) {
	if (Config{}).IsComplete() {
		t.Error("empty config should not be complete")
	}
	if (Config{TelegramBotToken: "tok"}).IsComplete() {
		t.Error("config with only token should not be complete")
	}
	if (Config{TelegramUserID: 1}).IsComplete() {
		t.Error("config with only user ID should not be complete")
	}
	if !(Config{TelegramBotToken: "tok", TelegramUserID: 1}).IsComplete() {
		t.Error("full config should be complete")
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := Config{TelegramBotToken: "tok456", TelegramUserID: 99}
	if err := cfg.SaveToFile(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TelegramBotToken != "tok456" || loaded.TelegramUserID != 99 {
		t.Errorf("round-trip failed: got %+v", loaded)
	}

	// Check file permissions
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("config file perm = %o, want 0600", info.Mode().Perm())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./config/ -v
```
Expected: FAIL — package doesn't exist yet

- [ ] **Step 3: Implement config.go**

`config/config.go`:
```go
package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	TelegramBotToken string `json:"telegram_bot_token"`
	TelegramUserID   int64  `json:"telegram_user_id"`
}

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "taskmaster", "config.json")
}

func LoadFromFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) SaveToFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (c Config) IsComplete() bool {
	return c.TelegramBotToken != "" && c.TelegramUserID != 0
}

func InteractiveSetup(r *bufio.Reader) (Config, error) {
	fmt.Println("Welcome to Taskmaster!")
	fmt.Println()

	fmt.Print("Enter your Telegram Bot Token (from @BotFather): ")
	token, err := r.ReadString('\n')
	if err != nil {
		return Config{}, err
	}
	token = strings.TrimSpace(token)

	fmt.Print("Enter your Telegram User ID: ")
	idStr, err := r.ReadString('\n')
	if err != nil {
		return Config{}, err
	}
	idStr = strings.TrimSpace(idStr)

	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return Config{}, fmt.Errorf("invalid user ID: %w", err)
	}

	return Config{
		TelegramBotToken: token,
		TelegramUserID:   userID,
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./config/ -v
```
Expected: all 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add config/
git commit -m "feat: config loading, saving, and interactive setup"
```

---

### Task 3: Wordlist and deterministic naming

**Files:**
- Create: `wordlist/words.go`
- Create: `agent/naming.go`
- Create: `agent/naming_test.go`

- [ ] **Step 1: Create the embedded wordlist**

`wordlist/words.go`:
```go
package wordlist

// Words is a 256-word list of short, memorable animal/nature words for agent naming.
// All entries are unique.
var Words = [256]string{
	"ant", "ape", "asp", "bat", "bee", "boa", "bug", "cat",
	"cod", "cow", "cub", "dab", "doe", "dog", "eel", "elk",
	"emu", "ewe", "fly", "fox", "gar", "gnu", "hen", "hog",
	"jay", "kea", "kid", "koi", "lam", "leo", "moa", "owl",
	"pig", "pug", "ram", "rat", "ray", "roc", "roe", "roo",
	"sal", "yak", "wol", "tit", "tur", "tui", "tad", "sow",
	"alb", "auk", "axo", "bai", "bar", "bas", "bir", "boo",
	"buc", "buf", "bul", "bun", "cab", "cam", "cap", "car",
	"chi", "cic", "cla", "cob", "con", "cor", "cra", "cro",
	"cur", "dac", "dar", "dee", "din", "dol", "don", "dor",
	"dov", "dra", "dro", "duc", "dug", "dun", "eag", "ear",
	"ech", "eld", "eri", "fal", "fan", "fer", "fin", "fla",
	"flo", "fri", "gal", "gaz", "gec", "ger", "goa", "goo",
	"gor", "gos", "gro", "gru", "gui", "gul", "gup", "ham",
	"har", "haw", "hed", "her", "hip", "hor", "hum", "hye",
	"ibi", "imp", "jab", "jac", "jag", "jel", "jer", "kan",
	"kes", "kin", "kit", "koa", "koo", "kud", "lac", "lar",
	"lem", "lyn", "lin", "lio", "liz", "lla", "lob", "loo",
	"lor", "mac", "mag", "man", "mar", "mee", "min", "mol",
	"mon", "moo", "mos", "mou", "mul", "mun", "mus", "mut",
	"nar", "new", "nig", "oct", "oka", "ori", "osp", "ost",
	"ott", "pan", "par", "pea", "pel", "pen", "per", "phe",
	"pho", "pir", "pla", "pol", "por", "pos", "pra", "puf",
	"pum", "pyt", "qua", "rab", "rac", "rai", "rap", "rav",
	"rhi", "rob", "ros", "sab", "san", "sca", "sea", "ser",
	"sha", "she", "shr", "sil", "ska", "slo", "sna", "sni",
	"spa", "spi", "squ", "sta", "sto", "stu", "sun", "swa",
	"swi", "tap", "tar", "ter", "tig", "toa", "tor", "tre",
	"tro", "tun", "ung", "uru", "val", "vip", "vul", "wal",
	"war", "wea", "wha", "wil", "wis", "wom", "woo", "wre",
	"xen", "zeb", "zer", "zor", "sku", "coy", "myr", "aye",
	"cas", "dik", "fos", "nab", "orb", "pik", "vet", "wag",
}
```

- [ ] **Step 2: Write failing test for naming**

`agent/naming_test.go`:
```go
package agent

import "testing"

func TestGenerateName(t *testing.T) {
	// Same session ID should always produce the same name
	name1 := GenerateName("ace", "abc-123-session")
	name2 := GenerateName("ace", "abc-123-session")
	if name1 != name2 {
		t.Errorf("names not stable: %q vs %q", name1, name2)
	}

	// Format should be <dir>-<word>
	if len(name1) < 4 { // minimum: "x-yy"
		t.Errorf("name too short: %q", name1)
	}

	// Different session IDs should (very likely) produce different names
	name3 := GenerateName("ace", "def-456-session")
	if name1 == name3 {
		t.Errorf("different sessions got same name: %q", name1)
	}

	// Different dirs should produce different names
	name4 := GenerateName("venture", "abc-123-session")
	if name1 == name4 {
		t.Errorf("different dirs got same name: %q vs %q", name1, name4)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./agent/ -run TestGenerateName -v
```
Expected: FAIL — function doesn't exist

- [ ] **Step 4: Implement naming.go**

`agent/naming.go`:
```go
package agent

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/jpoz/taskmaster/wordlist"
)

func GenerateName(dir string, sessionID string) string {
	h := sha256.Sum256([]byte(sessionID))
	idx := binary.BigEndian.Uint16(h[:2]) % uint16(len(wordlist.Words))
	return fmt.Sprintf("%s-%s", dir, wordlist.Words[idx])
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./agent/ -run TestGenerateName -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add wordlist/ agent/naming.go agent/naming_test.go
git commit -m "feat: wordlist and deterministic agent naming"
```

---

## Chunk 2: Agent struct, session parsing (Claude Code + Codex)

### Task 4: Agent struct and status detection

**Files:**
- Create: `agent/agent.go`
- Create: `agent/agent_test.go`

- [ ] **Step 1: Write failing test for status detection**

`agent/agent_test.go`:
```go
package agent

import (
	"testing"
	"time"
)

func TestAgent_Status_Working(t *testing.T) {
	a := Agent{
		SessionModTime: time.Now(), // modified just now
		LastEntryType:  "assistant",
	}
	if s := a.Status(); s != StatusWorking {
		t.Errorf("got %q, want %q", s, StatusWorking)
	}
}

func TestAgent_Status_Waiting(t *testing.T) {
	a := Agent{
		SessionModTime: time.Now().Add(-30 * time.Second), // modified 30s ago
		LastEntryType:  "assistant",
	}
	if s := a.Status(); s != StatusWaiting {
		t.Errorf("got %q, want %q", s, StatusWaiting)
	}
}

func TestAgent_Status_Idle(t *testing.T) {
	a := Agent{
		SessionModTime: time.Now().Add(-30 * time.Second),
		LastEntryType:  "user",
	}
	if s := a.Status(); s != StatusIdle {
		t.Errorf("got %q, want %q", s, StatusIdle)
	}
}

func TestAgent_Status_Unknown(t *testing.T) {
	a := Agent{
		SessionModTime: time.Time{}, // zero value — no session
	}
	if s := a.Status(); s != StatusUnknown {
		t.Errorf("got %q, want %q", s, StatusUnknown)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./agent/ -run TestAgent_Status -v
```
Expected: FAIL

- [ ] **Step 3: Implement agent.go**

`agent/agent.go`:
```go
package agent

import "time"

type AgentType string

const (
	TypeClaude AgentType = "claude"
	TypeCodex  AgentType = "codex"
)

type Status string

const (
	StatusWorking Status = "working"
	StatusWaiting Status = "waiting"
	StatusIdle    Status = "idle"
	StatusUnknown Status = "unknown"
)

type Agent struct {
	Name           string
	Type           AgentType
	CWD            string
	PID            int
	TTY            string
	SessionID      string
	SessionPath    string
	SessionModTime time.Time
	LastEntryType  string // "user" or "assistant" (tool_results arrive as type:"user")
}

func (a Agent) Status() Status {
	if a.SessionModTime.IsZero() {
		return StatusUnknown
	}
	if time.Since(a.SessionModTime) < 5*time.Second {
		return StatusWorking
	}
	if a.LastEntryType == "assistant" {
		return StatusWaiting
	}
	return StatusIdle
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./agent/ -run TestAgent_Status -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add agent/agent.go agent/agent_test.go
git commit -m "feat: agent struct with status detection"
```

---

### Task 5: Shared session types

**Files:**
- Create: `agent/session.go`

- [ ] **Step 1: Create session.go with shared Message type**

`agent/session.go`:
```go
package agent

// Message is a human-readable conversation message extracted from a session.
type Message struct {
	Role    string // "user" or "assistant"
	Content string // text content
	IsToolUse  bool // true if this is a tool_use block
	IsThinking bool // true if this is a thinking block
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./agent/
```
Expected: success

- [ ] **Step 3: Commit**

```bash
git add agent/session.go
git commit -m "feat: shared session message type"
```

---

### Task 6: Claude Code session parsing

**Files:**
- Create: `agent/session_claude.go`
- Create: `agent/session_claude_test.go`

- [ ] **Step 1: Write test fixtures and failing tests**

`agent/session_claude_test.go`:
```go
package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindClaudeSession(t *testing.T) {
	// Set up fake ~/.claude/projects/<encoded-cwd>/ with two session files
	dir := t.TempDir()
	projDir := filepath.Join(dir, "-Users-test-myproject")
	os.MkdirAll(projDir, 0755)

	// Older session
	old := filepath.Join(projDir, "old-session-id.jsonl")
	os.WriteFile(old, []byte(`{"type":"user","message":{"role":"user","content":"hello"}}`+"\n"), 0644)
	// Touch it in the past
	pastTime := mustParseTime(t, "2024-01-01T00:00:00Z")
	os.Chtimes(old, pastTime, pastTime)

	// Newer session
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

	// msg[0]: user text
	if msgs[0].Role != "user" || msgs[0].Content != "hello there" {
		t.Errorf("msg[0] = %+v", msgs[0])
	}
	// msg[1]: assistant text
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi back" {
		t.Errorf("msg[1] = %+v", msgs[1])
	}
	// msg[2]: thinking block
	if !msgs[2].IsThinking {
		t.Errorf("msg[2] should be thinking: %+v", msgs[2])
	}
	// msg[3]: tool_use block
	if !msgs[3].IsToolUse || msgs[3].Content != "[tool: Read]" {
		t.Errorf("msg[3] should be tool_use: %+v", msgs[3])
	}
	// msg[4]: tool_result
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./agent/ -run TestFindClaude -v
go test ./agent/ -run TestParseClaude -v
go test ./agent/ -run TestClaudeLast -v
```
Expected: FAIL

- [ ] **Step 3: Implement session_claude.go**

`agent/session_claude.go`:
```go
package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindClaudeSession finds the most recently modified .jsonl session file
// for the given CWD in the Claude projects directory.
// claudeDir is the base projects directory (e.g., ~/.claude/projects).
func FindClaudeSession(claudeDir string, cwd string) (sessionID string, sessionPath string, err error) {
	encoded := encodeCWD(cwd)
	projDir := filepath.Join(claudeDir, encoded)

	entries, err := os.ReadDir(projDir)
	if err != nil {
		return "", "", fmt.Errorf("reading project dir %s: %w", projDir, err)
	}

	var bestPath string
	var bestTime int64

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().UnixNano() > bestTime {
			bestTime = info.ModTime().UnixNano()
			bestPath = filepath.Join(projDir, e.Name())
		}
	}

	if bestPath == "" {
		return "", "", fmt.Errorf("no session files found in %s", projDir)
	}

	base := filepath.Base(bestPath)
	sessionID = strings.TrimSuffix(base, ".jsonl")
	return sessionID, bestPath, nil
}

// encodeCWD converts a path to Claude's encoded format: /Users/foo/bar -> -Users-foo-bar
func encodeCWD(cwd string) string {
	return strings.ReplaceAll(cwd, "/", "-")
}

// claudeJSONLEntry represents a single line in a Claude Code JSONL file.
type claudeJSONLEntry struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message"`
}

type claudeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type claudeContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Thinking string `json:"thinking"`
	Name     string `json:"name"`    // for tool_use
	Content  string `json:"content"` // for tool_result (when string)
}

// ParseClaudeMessages reads the last N messages from a Claude Code JSONL file.
func ParseClaudeMessages(path string, n int) ([]Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB line buffer

	for scanner.Scan() {
		line := scanner.Bytes()
		var entry claudeJSONLEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.Type != "user" && entry.Type != "assistant" {
			continue
		}
		if entry.Message == nil {
			continue
		}

		var msg claudeMessage
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		msgs := parseClaudeContent(msg.Role, msg.Content)
		all = append(all, msgs...)
	}

	// Return last N
	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all, scanner.Err()
}

func parseClaudeContent(role string, raw json.RawMessage) []Message {
	// Try as string first (user messages are often plain strings)
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		if str == "" || strings.HasPrefix(str, "<local-command-caveat>") || strings.HasPrefix(str, "<command-name>") {
			return nil // skip meta/command messages
		}
		return []Message{{Role: role, Content: str}}
	}

	// Try as array of content blocks
	var blocks []claudeContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil
	}

	var msgs []Message
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				msgs = append(msgs, Message{Role: role, Content: b.Text})
			}
		case "tool_use":
			msgs = append(msgs, Message{Role: role, Content: fmt.Sprintf("[tool: %s]", b.Name), IsToolUse: true})
		case "thinking":
			if b.Thinking != "" {
				msgs = append(msgs, Message{Role: role, Content: b.Thinking, IsThinking: true})
			}
		case "tool_result":
			content := "[tool_result]"
			if b.Content != "" {
				content = b.Content
			}
			msgs = append(msgs, Message{Role: role, Content: content, IsToolUse: true})
		}
	}
	return msgs
}

// ClaudeLastEntryType returns the type of the last user/assistant entry in the JSONL.
func ClaudeLastEntryType(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lastType string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		var entry claudeJSONLEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Type == "user" || entry.Type == "assistant" {
			lastType = entry.Type
		}
	}
	return lastType, scanner.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./agent/ -run "TestFindClaude|TestParseClaude|TestClaudeLast" -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add agent/session_claude.go agent/session_claude_test.go
git commit -m "feat: claude code session finding and JSONL parsing"
```

---

### Task 7: Codex session parsing

**Files:**
- Create: `agent/session_codex.go`
- Create: `agent/session_codex_test.go`

- [ ] **Step 1: Write failing tests**

`agent/session_codex_test.go`:
```go
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

	// msg[0]: user message
	if msgs[0].Role != "user" || msgs[0].Content != "fix the bug" {
		t.Errorf("msg[0] = %+v", msgs[0])
	}
	// msg[1]: assistant text
	if msgs[1].Role != "assistant" || msgs[1].Content != "I found the issue and fixed it." {
		t.Errorf("msg[1] = %+v", msgs[1])
	}
	// msg[2]: tool use
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

func TestCodexLastEntryType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	lines := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}
{"type":"event_msg","payload":{"type":"agent_message","agent_message":"hi"}}
`
	os.WriteFile(path, []byte(lines), 0644)

	entryType, err := CodexLastEntryType(path)
	if err != nil {
		t.Fatal(err)
	}
	if entryType != "assistant" {
		t.Errorf("got %q, want %q", entryType, "assistant")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./agent/ -run TestParseCodex -v
go test ./agent/ -run TestCodexLast -v
```
Expected: FAIL

- [ ] **Step 3: Implement session_codex.go**

`agent/session_codex.go`:
```go
package agent

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// CodexThread holds metadata from the Codex threads table.
type CodexThread struct {
	ID          string
	RolloutPath string
	CWD         string
}

// FindCodexSession queries the Codex SQLite DB for the most recent thread matching the CWD.
func FindCodexSession(codexDir string, cwd string) (CodexThread, error) {
	dbPath := filepath.Join(codexDir, "state_5.sqlite")
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return CodexThread{}, fmt.Errorf("opening codex db: %w", err)
	}
	defer db.Close()

	var thread CodexThread
	err = db.QueryRowContext(context.Background(),
		`SELECT id, rollout_path, cwd FROM threads WHERE cwd = ? ORDER BY updated_at DESC LIMIT 1`,
		cwd,
	).Scan(&thread.ID, &thread.RolloutPath, &thread.CWD)
	if err != nil {
		return CodexThread{}, fmt.Errorf("querying codex threads: %w", err)
	}
	return thread, nil
}

// codexJSONLEntry represents a line in a Codex session JSONL file.
type codexJSONLEntry struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type codexResponsePayload struct {
	Type    string              `json:"type"`
	Role    string              `json:"role"`
	Content []codexContentBlock `json:"content"`
	Name    string              `json:"name"` // for function_call
}

type codexContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type codexEventPayload struct {
	Type         string `json:"type"`
	AgentMessage string `json:"agent_message"`
}

// ParseCodexMessages reads the last N messages from a Codex JSONL session file.
func ParseCodexMessages(path string, n int) ([]Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		var entry codexJSONLEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		switch entry.Type {
		case "response_item":
			var payload codexResponsePayload
			if err := json.Unmarshal(entry.Payload, &payload); err != nil {
				continue
			}
			if payload.Role == "user" {
				for _, block := range payload.Content {
					if block.Type == "input_text" && block.Text != "" {
						all = append(all, Message{Role: "user", Content: block.Text})
					}
				}
			} else if payload.Type == "function_call" {
				all = append(all, Message{
					Role:      "assistant",
					Content:   fmt.Sprintf("[tool: %s]", payload.Name),
					IsToolUse: true,
				})
			}
		case "event_msg":
			var payload codexEventPayload
			if err := json.Unmarshal(entry.Payload, &payload); err != nil {
				continue
			}
			if payload.Type == "agent_message" && payload.AgentMessage != "" {
				all = append(all, Message{Role: "assistant", Content: payload.AgentMessage})
			}
		}
	}

	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all, scanner.Err()
}

// CodexLastEntryType returns the role of the last meaningful entry in a Codex JSONL.
func CodexLastEntryType(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lastType string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		var entry codexJSONLEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		switch entry.Type {
		case "response_item":
			var payload codexResponsePayload
			if err := json.Unmarshal(entry.Payload, &payload); err != nil {
				continue
			}
			if payload.Role == "user" {
				lastType = "user"
			} else if payload.Type == "function_call" {
				lastType = "assistant"
			}
		case "event_msg":
			var payload codexEventPayload
			if err := json.Unmarshal(entry.Payload, &payload); err != nil {
				continue
			}
			if payload.Type == "agent_message" {
				lastType = "assistant"
			}
		}
	}
	return lastType, scanner.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./agent/ -run "TestParseCodex|TestCodexLast" -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add agent/session_codex.go agent/session_codex_test.go
git commit -m "feat: codex session parsing from SQLite and JSONL"
```

---

## Chunk 3: Agent discovery, TTY writer

### Task 8: Agent discovery (ps + lsof orchestration)

**Files:**
- Create: `agent/discovery.go`
- Create: `agent/discovery_test.go`

- [ ] **Step 1: Write failing test for process parsing**

`agent/discovery_test.go`:
```go
package agent

import "testing"

func TestParsePsLine(t *testing.T) {
	tests := []struct {
		line    string
		pid     int
		tty     string
		args    string
	}{
		// Claude Code: comm is a version number, but args contains "claude"
		{" 10695 ttys018  claude --dangerously-skip-permissions", 10695, "ttys018", "claude --dangerously-skip-permissions"},
		// Codex
		{" 2569 ??       codex app-server --analytics-default-enabled", 2569, "??", "codex app-server --analytics-default-enabled"},
	}
	for _, tt := range tests {
		pid, tty, args, err := parsePsLine(tt.line)
		if err != nil {
			t.Fatalf("parsePsLine(%q): %v", tt.line, err)
		}
		if pid != tt.pid {
			t.Errorf("pid = %d, want %d", pid, tt.pid)
		}
		if tty != tt.tty {
			t.Errorf("tty = %q, want %q", tty, tt.tty)
		}
		if args != tt.args {
			t.Errorf("args = %q, want %q", args, tt.args)
		}
	}
}

func TestClassifyArgs(t *testing.T) {
	tests := []struct {
		args     string
		wantType AgentType
		wantSkip bool
	}{
		{"claude --dangerously-skip-permissions", TypeClaude, false},
		{"codex", TypeCodex, false},
		{"/Applications/Claude.app/Contents/Frameworks/Squirrel.framework/Resources/ShipIt", "", true},
		{"codex app-server --analytics-default-enabled", "", true},
		{"/Applications/Codex.app/Contents/MacOS/Codex", "", true},
	}
	for _, tt := range tests {
		agentType, skip := classifyArgs(tt.args)
		if skip != tt.wantSkip {
			t.Errorf("classifyArgs(%q) skip = %v, want %v", tt.args, skip, tt.wantSkip)
		}
		if !skip && agentType != tt.wantType {
			t.Errorf("classifyArgs(%q) type = %q, want %q", tt.args, agentType, tt.wantType)
		}
	}
}

func TestParseLsofLine(t *testing.T) {
	// Format from: lsof -a -d cwd -p PIDs
	line := "2.1.72  10695 jpoz  cwd    DIR   1,16     2176 162287637 /Users/jpoz/Developer/ace"
	pid, cwd, err := parseLsofLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if pid != 10695 {
		t.Errorf("pid = %d, want 10695", pid)
	}
	if cwd != "/Users/jpoz/Developer/ace" {
		t.Errorf("cwd = %q", cwd)
	}
}

// classifyArgs tests are in TestClassifyArgs above
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./agent/ -run "TestParsePs|TestParseLsof|TestClassify" -v
```
Expected: FAIL

- [ ] **Step 3: Implement discovery.go**

`agent/discovery.go`:
```go
package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type processInfo struct {
	PID       int
	TTY       string
	Args      string
	AgentType AgentType
}

// DiscoverAgents finds all active Claude Code and Codex agent processes,
// resolves their CWDs, and matches them with session data.
func DiscoverAgents() ([]Agent, error) {
	procs, err := findAgentProcesses()
	if err != nil {
		return nil, err
	}
	if len(procs) == 0 {
		return nil, nil
	}

	cwds, err := resolveCWDs(procs)
	if err != nil {
		return nil, err
	}

	home, _ := os.UserHomeDir()
	claudeProjectsDir := filepath.Join(home, ".claude", "projects")
	codexDir := filepath.Join(home, ".codex")

	var agents []Agent
	for _, p := range procs {
		cwd, ok := cwds[p.PID]
		if !ok {
			continue
		}

		a := Agent{
			Type: p.AgentType,
			CWD:  cwd,
			PID:  p.PID,
			TTY:  p.TTY,
		}

		dirBase := filepath.Base(cwd)

		switch a.Type {
		case TypeClaude:
			sessionID, sessionPath, err := FindClaudeSession(claudeProjectsDir, cwd)
			if err == nil {
				a.SessionID = sessionID
				a.SessionPath = sessionPath
				info, _ := os.Stat(sessionPath)
				if info != nil {
					a.SessionModTime = info.ModTime()
				}
				a.LastEntryType, _ = ClaudeLastEntryType(sessionPath)
			}
			if a.SessionID == "" {
				a.SessionID = fmt.Sprintf("pid-%d", p.PID)
			}
			a.Name = GenerateName(dirBase, a.SessionID)

		case TypeCodex:
			thread, err := FindCodexSession(codexDir, cwd)
			if err == nil {
				a.SessionID = thread.ID
				a.SessionPath = thread.RolloutPath
				info, _ := os.Stat(thread.RolloutPath)
				if info != nil {
					a.SessionModTime = info.ModTime()
				}
				a.LastEntryType, _ = CodexLastEntryType(thread.RolloutPath)
			}
			if a.SessionID == "" {
				a.SessionID = fmt.Sprintf("pid-%d", p.PID)
			}
			a.Name = GenerateName(dirBase, a.SessionID)
		}

		agents = append(agents, a)
	}

	return agents, nil
}

func findAgentProcesses() ([]processInfo, error) {
	// Use pid,tty,args — we classify by args, not comm, because
	// Claude Code's binary name is its version number (e.g. "2.1.72")
	out, err := exec.Command("ps", "-eo", "pid,tty,args").Output()
	if err != nil {
		return nil, fmt.Errorf("running ps: %w", err)
	}

	var procs []processInfo
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "PID") {
			continue
		}
		// Quick pre-filter: line must mention claude or codex somewhere
		if !strings.Contains(line, "claude") && !strings.Contains(line, "codex") {
			continue
		}

		pid, tty, args, err := parsePsLine(line)
		if err != nil {
			continue
		}

		agentType, skip := classifyArgs(args)
		if skip {
			continue
		}

		procs = append(procs, processInfo{PID: pid, TTY: tty, Args: args, AgentType: agentType})
	}
	return procs, nil
}

// parsePsLine parses a line from `ps -eo pid,tty,args`.
// Returns PID, TTY, and the full args string.
func parsePsLine(line string) (pid int, tty string, args string, err error) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return 0, "", "", fmt.Errorf("too few fields: %q", line)
	}
	pid, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, "", "", fmt.Errorf("parsing pid: %w", err)
	}
	tty = fields[1]
	args = strings.Join(fields[2:], " ")
	return pid, tty, args, nil
}

// classifyArgs determines if a process is a Claude/Codex agent or should be skipped.
// It examines the full args string to handle cases where the binary name is a
// version number (e.g. Claude Code binary is named "2.1.72").
func classifyArgs(args string) (agentType AgentType, skip bool) {
	skipPatterns := []string{"ShipIt", "app-server", "Codex.app", "Claude.app"}
	for _, pat := range skipPatterns {
		if strings.Contains(args, pat) {
			return "", true
		}
	}

	// The first token in args is the command/binary path.
	// For Claude Code: "claude ..." or "/Users/.../.claude/local/2.1.72 ..."
	// For Codex: "codex ..." or "/path/to/codex ..."
	firstToken := strings.Fields(args)[0]
	base := filepath.Base(firstToken)

	if base == "claude" || strings.Contains(firstToken, ".claude/local/") {
		return TypeClaude, false
	}
	if base == "codex" || strings.Contains(firstToken, "codex") {
		return TypeCodex, false
	}

	return "", true // unknown process, skip
}

func resolveCWDs(procs []processInfo) (map[int]string, error) {
	pids := make([]string, len(procs))
	for i, p := range procs {
		pids[i] = strconv.Itoa(p.PID)
	}

	out, err := exec.Command("lsof", "-a", "-d", "cwd", "-p", strings.Join(pids, ",")).Output()
	if err != nil {
		// lsof returns exit code 1 if some PIDs not found; check if we got output
		if len(out) == 0 {
			return nil, fmt.Errorf("lsof failed: %w", err)
		}
	}

	cwds := make(map[int]string)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "COMMAND") {
			continue
		}
		pid, cwd, err := parseLsofLine(line)
		if err != nil {
			continue
		}
		cwds[pid] = cwd
	}
	return cwds, nil
}

func parseLsofLine(line string) (pid int, cwd string, err error) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return 0, "", fmt.Errorf("too few fields: %q", line)
	}
	pid, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, "", fmt.Errorf("parsing pid: %w", err)
	}
	// lsof NAME field is everything from field index 8 onward
	cwd = strings.Join(fields[8:], " ")
	return pid, cwd, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./agent/ -run "TestParsePs|TestParseLsof|TestClassify" -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add agent/discovery.go agent/discovery_test.go
git commit -m "feat: agent discovery via ps and lsof"
```

---

### Task 9: TTY writer

**Files:**
- Create: `tty/writer.go`
- Create: `tty/writer_test.go`

- [ ] **Step 1: Write failing test**

`tty/writer_test.go`:
```go
package tty

import "testing"

func TestSanitizeMessage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello world"},
		{"hello\x1b[31mworld", "helloworld"},          // strip ANSI escape
		{"hello\x00world", "helloworld"},               // strip null byte
		{"hello\nworld", "hello\nworld"},               // preserve newline
		{"hello\tworld", "hello\tworld"},               // preserve tab
		{"\x1b]0;title\x07text", "text"},               // strip OSC sequence
	}
	for _, tt := range tests {
		got := SanitizeMessage(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeMessage(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTTYDevicePath(t *testing.T) {
	tests := []struct {
		tty  string
		want string
	}{
		{"ttys010", "/dev/ttys010"},
		{"ttys018", "/dev/ttys018"},
		{"??", ""},
	}
	for _, tt := range tests {
		got := DevicePath(tt.tty)
		if got != tt.want {
			t.Errorf("DevicePath(%q) = %q, want %q", tt.tty, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./tty/ -v
```
Expected: FAIL

- [ ] **Step 3: Implement writer.go**

`tty/writer.go`:
```go
package tty

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var ansiEscape = regexp.MustCompile(`\x1b(?:\[[0-9;]*[a-zA-Z]|\][^\x07]*\x07)`)

// SanitizeMessage strips ANSI escape sequences and control characters
// (except newline and tab) from the message.
func SanitizeMessage(msg string) string {
	msg = ansiEscape.ReplaceAllString(msg, "")
	var b strings.Builder
	for _, r := range msg {
		if r == '\n' || r == '\t' || r >= 32 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// DevicePath converts a TTY name from ps output to a device path.
// Returns empty string for detached processes (tty = "??").
func DevicePath(tty string) string {
	if tty == "??" || tty == "-" || tty == "" {
		return ""
	}
	return "/dev/" + tty
}

// VerifyPIDOwnsTTY checks that the given PID still owns the given TTY.
func VerifyPIDOwnsTTY(pid int, tty string) error {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "tty=").Output()
	if err != nil {
		return fmt.Errorf("process %d no longer running", pid)
	}
	currentTTY := strings.TrimSpace(string(out))
	if currentTTY != tty {
		return fmt.Errorf("process %d TTY changed from %s to %s", pid, tty, currentTTY)
	}
	return nil
}

// WriteToTTY writes a message to the given TTY device.
// It sanitizes the message, verifies PID ownership, and appends a newline.
func WriteToTTY(pid int, tty string, msg string) error {
	devPath := DevicePath(tty)
	if devPath == "" {
		return fmt.Errorf("no TTY attached")
	}

	if err := VerifyPIDOwnsTTY(pid, tty); err != nil {
		return err
	}

	sanitized := SanitizeMessage(msg)

	f, err := os.OpenFile(devPath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("opening TTY %s: %w", devPath, err)
	}
	defer f.Close()

	_, err = f.Write([]byte(sanitized + "\n"))
	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./tty/ -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tty/
git commit -m "feat: TTY writer with sanitization and PID verification"
```

---

## Chunk 4: Telegram bot and main entrypoint

### Task 10: Telegram bot with command routing

**Files:**
- Create: `bot/bot.go`

- [ ] **Step 1: Add telegram dependency**

```bash
cd /Users/jpoz/Developer/taskmaster
go get github.com/go-telegram-bot-api/telegram-bot-api/v5
```

- [ ] **Step 2: Implement bot.go**

`bot/bot.go`:
```go
package bot

import (
	"fmt"
	"strings"

	"github.com/jpoz/taskmaster/agent"
	"github.com/jpoz/taskmaster/tty"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api    *tgbotapi.BotAPI
	userID int64
}

func New(token string, userID int64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}
	return &Bot{api: api, userID: userID}, nil
}

func (b *Bot) Run() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		if update.Message.From.ID != b.userID {
			continue // silently ignore other users
		}

		text := strings.TrimSpace(update.Message.Text)
		reply := b.handleCommand(text)
		b.send(update.Message.Chat.ID, reply)
	}
	return nil
}

func (b *Bot) handleCommand(text string) string {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "Send: ls, tail [id], or echo [id] [msg]"
	}

	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "ls":
		return b.handleLs()
	case "tail":
		return b.handleTail(parts[1:])
	case "echo":
		return b.handleEcho(parts[1:])
	default:
		return "Unknown command. Send: ls, tail [id], or echo [id] [msg]"
	}
}

func (b *Bot) handleLs() string {
	agents, err := agent.DiscoverAgents()
	if err != nil {
		return fmt.Sprintf("Error discovering agents: %v", err)
	}
	if len(agents) == 0 {
		return "No active agents found."
	}

	var sb strings.Builder
	for _, a := range agents {
		fmt.Fprintf(&sb, "%-16s %-8s %s  [%s]\n", a.Name, a.Type, a.CWD, a.Status())
	}
	return sb.String()
}

func (b *Bot) handleTail(args []string) string {
	if len(args) == 0 {
		return "Usage: tail [-v] <id>"
	}

	verbose := false
	id := args[0]
	if id == "-v" {
		verbose = true
		if len(args) < 2 {
			return "Usage: tail -v <id>"
		}
		id = args[1]
	}

	agents, err := agent.DiscoverAgents()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	a := findAgent(agents, id)
	if a == nil {
		return fmt.Sprintf("Unknown agent: %s. Use ls to see active agents.", id)
	}

	if a.SessionPath == "" {
		return fmt.Sprintf("No session data available for %s.", id)
	}

	// Parse more messages than needed so we have enough after filtering
	fetchCount := 50
	if verbose {
		fetchCount = 10
	}

	var msgs []agent.Message
	switch a.Type {
	case agent.TypeClaude:
		msgs, err = agent.ParseClaudeMessages(a.SessionPath, fetchCount)
	case agent.TypeCodex:
		msgs, err = agent.ParseCodexMessages(a.SessionPath, fetchCount)
	}
	if err != nil {
		return fmt.Sprintf("Could not read session for %s.", id)
	}

	// Filter for display mode, then take last 10
	if !verbose {
		var filtered []agent.Message
		for _, m := range msgs {
			if !m.IsToolUse && !m.IsThinking {
				filtered = append(filtered, m)
			}
		}
		msgs = filtered
	}
	if len(msgs) > 10 {
		msgs = msgs[len(msgs)-10:]
	}

	return formatMessages(msgs, a.Type)
}

func (b *Bot) handleEcho(args []string) string {
	if len(args) < 2 {
		return "Usage: echo <id> <message>"
	}

	id := args[0]
	msg := strings.Join(args[1:], " ")

	agents, err := agent.DiscoverAgents()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	a := findAgent(agents, id)
	if a == nil {
		return fmt.Sprintf("Unknown agent: %s. Use ls to see active agents.", id)
	}

	if a.TTY == "" || a.TTY == "??" {
		return fmt.Sprintf("Agent %s has no TTY attached.", id)
	}

	if err := tty.WriteToTTY(a.PID, a.TTY, msg); err != nil {
		if strings.Contains(err.Error(), "no longer running") {
			return fmt.Sprintf("Agent %s is no longer running.", id)
		}
		return fmt.Sprintf("Error writing to %s: %v", id, err)
	}

	return fmt.Sprintf("Sent to %s: %s", id, msg)
}

func findAgent(agents []agent.Agent, id string) *agent.Agent {
	for i := range agents {
		if agents[i].Name == id {
			return &agents[i]
		}
	}
	return nil
}

func formatMessages(msgs []agent.Message, agentType agent.AgentType) string {
	if len(msgs) == 0 {
		return "(no messages)"
	}

	assistantLabel := "[claude]"
	if agentType == agent.TypeCodex {
		assistantLabel = "[codex]"
	}

	var sb strings.Builder
	for _, m := range msgs {
		label := "[you]"
		if m.Role == "assistant" {
			if m.IsThinking {
				label = "[thinking]"
			} else if m.IsToolUse {
				label = "[tool]"
			} else {
				label = assistantLabel
			}
		}

		content := m.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}

		fmt.Fprintf(&sb, "%s %s\n\n", label, content)
	}
	return sb.String()
}

func (b *Bot) send(chatID int64, text string) {
	// Telegram has a 4096 char limit per message
	if len(text) > 4000 {
		text = text[:4000] + "\n..."
	}
	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)
}
```

- [ ] **Step 3: Write bot unit tests**

`bot/bot_test.go`:
```go
package bot

import (
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
	if !contains(out, "[you] hello") {
		t.Errorf("missing user message in: %s", out)
	}
	if !contains(out, "[claude] hi there") {
		t.Errorf("missing assistant message in: %s", out)
	}
}

func TestFormatMessages_CodexLabel(t *testing.T) {
	msgs := []agent.Message{
		{Role: "assistant", Content: "done"},
	}
	out := formatMessages(msgs, agent.TypeCodex)
	if !contains(out, "[codex] done") {
		t.Errorf("expected [codex] label in: %s", out)
	}
}

func TestFormatMessages_Truncation(t *testing.T) {
	long := make([]byte, 600)
	for i := range long {
		long[i] = 'a'
	}
	msgs := []agent.Message{
		{Role: "user", Content: string(long)},
	}
	out := formatMessages(msgs, agent.TypeClaude)
	if len(out) > 520 { // 500 + "[you] " + "..." + newlines
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
	b := &Bot{} // api is nil but handleCommand doesn't use it directly
	reply := b.handleCommand("blah")
	if !contains(reply, "Unknown command") {
		t.Errorf("expected unknown command response, got: %s", reply)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./bot/ -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bot/
git commit -m "feat: telegram bot with ls, tail, echo command routing"
```

---

### Task 11: Wire up main.go

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Implement the full main.go**

`main.go`:
```go
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/jpoz/taskmaster/bot"
	"github.com/jpoz/taskmaster/config"
)

func main() {
	cfgPath := config.DefaultPath()

	cfg, err := config.LoadFromFile(cfgPath)
	if err != nil || !cfg.IsComplete() {
		reader := bufio.NewReader(os.Stdin)
		cfg, err = config.InteractiveSetup(reader)
		if err != nil {
			log.Fatalf("Setup failed: %v", err)
		}
		if err := cfg.SaveToFile(cfgPath); err != nil {
			log.Fatalf("Failed to save config: %v", err)
		}
		fmt.Printf("\nConfig saved to %s\n", cfgPath)
	}

	fmt.Println("Starting bot...")

	b, err := bot.New(cfg.TelegramBotToken, cfg.TelegramUserID)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	if err := b.Run(); err != nil {
		log.Fatalf("Bot error: %v", err)
	}
}
```

- [ ] **Step 2: Add sqlite dependency and tidy**

```bash
cd /Users/jpoz/Developer/taskmaster
go get modernc.org/sqlite
go mod tidy
```

- [ ] **Step 3: Verify it compiles**

```bash
go build -o taskmaster .
```
Expected: produces `taskmaster` binary

- [ ] **Step 4: Commit**

```bash
git add main.go go.mod go.sum
git commit -m "feat: wire up main entrypoint with config setup and bot start"
```

---

## Chunk 5: Integration testing and polish

### Task 12: Integration test for full discovery + formatting flow

**Files:**
- Create: `agent/integration_test.go`

- [ ] **Step 1: Write integration test that exercises discovery output formatting**

`agent/integration_test.go`:
```go
//go:build integration

package agent

import (
	"testing"
)

func TestDiscoverAgents_Live(t *testing.T) {
	agents, err := DiscoverAgents()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Found %d agents:", len(agents))
	for _, a := range agents {
		t.Logf("  %-16s %-8s %s [%s]", a.Name, a.Type, a.CWD, a.Status())
		t.Logf("    Session: %s", a.SessionPath)
		t.Logf("    TTY: %s  PID: %d", a.TTY, a.PID)
	}
}
```

- [ ] **Step 2: Run unit tests (all packages)**

```bash
go test ./... -v
```
Expected: all unit tests PASS

- [ ] **Step 3: Run integration test (optional, requires running agents)**

```bash
go test ./agent/ -tags=integration -run TestDiscoverAgents_Live -v
```
Expected: lists any running agents (informational)

- [ ] **Step 4: Commit**

```bash
git add agent/integration_test.go
git commit -m "test: add live integration test for agent discovery"
```

---

### Task 13: Add .gitignore and build verification

**Files:**
- Create: `.gitignore`

- [ ] **Step 1: Create .gitignore**

`.gitignore`:
```
taskmaster
*.exe
```

- [ ] **Step 2: Full build and test pass**

```bash
go build -o taskmaster .
go test ./... -v
```
Expected: clean build, all tests PASS

- [ ] **Step 3: Commit**

```bash
git add .gitignore
git commit -m "chore: add gitignore"
```
