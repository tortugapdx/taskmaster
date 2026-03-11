# Telegram Tap-Friendly UI Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace typed Telegram commands with a persistent reply keyboard and contextual inline buttons so the bot can be operated entirely by tapping on mobile.

**Architecture:** Add three new files to the `bot/` package — `keyboard.go` (keyboard builders + callback data helpers), `callback.go` (callback query router), and `state.go` (chat state + auto-refresh lifecycle). Refactor `bot.go` handlers to return a `response` struct carrying text + optional inline keyboard. The update loop gains a `CallbackQuery` branch alongside the existing `Message` branch.

**Tech Stack:** Go, `go-telegram-bot-api/v5` v5.5.1 (existing dependency)

**Spec:** `docs/superpowers/specs/2026-03-11-telegram-inline-ui-design.md`

---

## Chunk 1: Foundation (keyboard helpers, callback parsing, response struct)

### Task 1: Callback data formatting and parsing helpers

**Files:**
- Create: `bot/keyboard.go`
- Create: `bot/keyboard_test.go`

- [ ] **Step 1: Write tests for callback data formatting and parsing**

```go
// bot/keyboard_test.go
package bot

import (
	"fmt"
	"testing"
)

func TestFormatCallback(t *testing.T) {
	tests := []struct {
		action string
		args   []string
		want   string
	}{
		{"ls", nil, "ls"},
		{"t", []string{"ace-fox"}, "t:ace-fox"},
		{"r", []string{"ace-fox", "v"}, "r:ace-fox:v"},
		{"x", nil, "x"},
	}
	for _, tt := range tests {
		got := formatCallback(tt.action, tt.args...)
		if got != tt.want {
			t.Errorf("formatCallback(%q, %v) = %q, want %q", tt.action, tt.args, got, tt.want)
		}
	}
}

func TestFormatCallback_TruncatesLongNames(t *testing.T) {
	longName := "my-very-long-project-directory-name-that-exceeds-fifty-chars-fox"
	cb := formatCallback("t", longName)
	if len(cb) > 64 {
		t.Errorf("callback data exceeds 64 bytes: %d bytes: %q", len(cb), cb)
	}
}

func TestParseCallback(t *testing.T) {
	tests := []struct {
		data       string
		wantAction string
		wantArgs   []string
	}{
		{"ls", "ls", nil},
		{"t:ace-fox", "t", []string{"ace-fox"}},
		{"r:ace-fox:v", "r", []string{"ace-fox", "v"}},
		{"x", "x", nil},
	}
	for _, tt := range tests {
		action, args := parseCallback(tt.data)
		if action != tt.wantAction {
			t.Errorf("parseCallback(%q) action = %q, want %q", tt.data, action, tt.wantAction)
		}
		if len(args) != len(tt.wantArgs) {
			t.Errorf("parseCallback(%q) args len = %d, want %d", tt.data, len(args), len(tt.wantArgs))
			continue
		}
		for i, a := range args {
			if a != tt.wantArgs[i] {
				t.Errorf("parseCallback(%q) args[%d] = %q, want %q", tt.data, i, a, tt.wantArgs[i])
			}
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/jpoz/Developer/taskmaster && go test ./bot/ -run "TestFormatCallback|TestParseCallback" -v`
Expected: FAIL — `formatCallback` and `parseCallback` not defined

- [ ] **Step 3: Implement callback data helpers**

```go
// bot/keyboard.go
package bot

import "strings"

const maxCallbackData = 64
const maxNameLen = 50

// formatCallback builds a callback data string like "action:arg1:arg2".
// Agent names are truncated to maxNameLen to stay within Telegram's 64-byte limit.
func formatCallback(action string, args ...string) string {
	if len(args) == 0 {
		return action
	}
	for i, a := range args {
		if len(a) > maxNameLen {
			args[i] = a[:maxNameLen]
		}
	}
	return action + ":" + strings.Join(args, ":")
}

// parseCallback splits callback data into action and arguments.
func parseCallback(data string) (string, []string) {
	parts := strings.SplitN(data, ":", 3)
	if len(parts) == 1 {
		return parts[0], nil
	}
	return parts[0], parts[1:]
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/jpoz/Developer/taskmaster && go test ./bot/ -run "TestFormatCallback|TestParseCallback" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bot/keyboard.go bot/keyboard_test.go
git commit -m "feat(bot): add callback data format/parse helpers"
```

### Task 2: Inline keyboard builders

**Files:**
- Modify: `bot/keyboard.go`
- Modify: `bot/keyboard_test.go`

- [ ] **Step 1: Write tests for keyboard builders**

```go
// Append to bot/keyboard_test.go
func TestReplyKeyboard(t *testing.T) {
	kb := replyKeyboard()
	if len(kb.Keyboard) != 1 {
		t.Fatalf("expected 1 row, got %d", len(kb.Keyboard))
	}
	if len(kb.Keyboard[0]) != 3 {
		t.Fatalf("expected 3 buttons, got %d", len(kb.Keyboard[0]))
	}
	if kb.Keyboard[0][0].Text != "ls" {
		t.Errorf("first button = %q, want %q", kb.Keyboard[0][0].Text, "ls")
	}
	if !kb.ResizeKeyboard {
		t.Error("ResizeKeyboard should be true")
	}
}

func TestLsKeyboard(t *testing.T) {
	names := []string{"ace-fox", "web-owl"}
	kb := lsInlineKeyboard(names)
	// One row per agent + one refresh row
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(kb.InlineKeyboard))
	}
	// First row: tail buttons for each agent
	if len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 2 buttons in first row, got %d", len(kb.InlineKeyboard[0]))
	}
	if *kb.InlineKeyboard[0][0].CallbackData != "t:ace-fox" {
		t.Errorf("first button callback = %q, want %q", *kb.InlineKeyboard[0][0].CallbackData, "t:ace-fox")
	}
	// Last row: refresh
	lastRow := kb.InlineKeyboard[len(kb.InlineKeyboard)-1]
	if *lastRow[0].CallbackData != "ls" {
		t.Errorf("refresh button callback = %q, want %q", *lastRow[0].CallbackData, "ls")
	}
}

func TestLsKeyboard_CapsAt10(t *testing.T) {
	names := make([]string, 15)
	for i := range names {
		names[i] = fmt.Sprintf("agent-%d", i)
	}
	kb := lsInlineKeyboard(names)
	// First row has at most 10 agent buttons, second row is refresh
	agentButtons := 0
	for _, row := range kb.InlineKeyboard[:len(kb.InlineKeyboard)-1] {
		agentButtons += len(row)
	}
	if agentButtons > 10 {
		t.Errorf("expected at most 10 agent buttons, got %d", agentButtons)
	}
}

func TestAgentPickerKeyboard(t *testing.T) {
	agents := []agentInfo{
		{name: "ace-fox", icon: "🟢", project: "taskmaster"},
		{name: "web-owl", icon: "🟡", project: "webapp"},
	}
	kb := agentPickerKeyboard(agents, "t")
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(kb.InlineKeyboard))
	}
	if *kb.InlineKeyboard[0][0].CallbackData != "t:ace-fox" {
		t.Errorf("callback = %q, want %q", *kb.InlineKeyboard[0][0].CallbackData, "t:ace-fox")
	}
}

func TestTailKeyboard(t *testing.T) {
	kb := tailInlineKeyboard("ace-fox", false, false)
	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(kb.InlineKeyboard))
	}
	// Row 1: Refresh + Verbose
	if len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("row 1: expected 2 buttons, got %d", len(kb.InlineKeyboard[0]))
	}
	// Row 2: Echo + Back
	if len(kb.InlineKeyboard[1]) != 2 {
		t.Fatalf("row 2: expected 2 buttons, got %d", len(kb.InlineKeyboard[1]))
	}
	// Row 3: Auto-refresh
	if len(kb.InlineKeyboard[2]) != 1 {
		t.Fatalf("row 3: expected 1 button, got %d", len(kb.InlineKeyboard[2]))
	}
}

func TestTailKeyboard_VerboseToggle(t *testing.T) {
	compact := tailInlineKeyboard("ace-fox", false, false)
	verbose := tailInlineKeyboard("ace-fox", true, false)
	compactBtn := compact.InlineKeyboard[0][1].Text
	verboseBtn := verbose.InlineKeyboard[0][1].Text
	if compactBtn == verboseBtn {
		t.Error("verbose toggle button text should differ between modes")
	}
}

func TestTailKeyboard_AutoRefreshToggle(t *testing.T) {
	normal := tailInlineKeyboard("ace-fox", false, false)
	refreshing := tailInlineKeyboard("ace-fox", false, true)
	normalBtn := normal.InlineKeyboard[2][0].Text
	refreshBtn := refreshing.InlineKeyboard[2][0].Text
	if normalBtn == refreshBtn {
		t.Error("auto-refresh button text should differ when active")
	}
}

func TestEchoPickerKeyboard(t *testing.T) {
	agents := []agentInfo{
		{name: "ace-fox", icon: "🟢", project: "taskmaster"},
	}
	kb := echoPickerKeyboard(agents)
	// 1 agent row + 1 cancel row
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(kb.InlineKeyboard))
	}
	lastRow := kb.InlineKeyboard[len(kb.InlineKeyboard)-1]
	if *lastRow[0].CallbackData != "x" {
		t.Errorf("cancel button callback = %q, want %q", *lastRow[0].CallbackData, "x")
	}
}

func TestEchoPromptKeyboard(t *testing.T) {
	kb := echoPromptKeyboard()
	if len(kb.InlineKeyboard) != 1 {
		t.Fatalf("expected 1 row, got %d", len(kb.InlineKeyboard))
	}
	if *kb.InlineKeyboard[0][0].CallbackData != "x" {
		t.Errorf("cancel button callback = %q, want %q", *kb.InlineKeyboard[0][0].CallbackData, "x")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/jpoz/Developer/taskmaster && go test ./bot/ -run "TestReplyKeyboard|TestLsKeyboard|TestAgentPicker|TestTailKeyboard|TestEchoPicker|TestEchoPrompt" -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement keyboard builders**

Replace the import in `bot/keyboard.go` with the full import block, then add the following code after the existing `parseCallback` function:

```go
import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jpoz/taskmaster/agent"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type agentInfo struct {
	name    string
	icon    string
	project string
}

func agentInfoList(agents []agent.Agent) []agentInfo {
	var list []agentInfo
	for _, a := range agents {
		list = append(list, agentInfo{
			name:    a.Name,
			icon:    statusToIcon(a.Status()),
			project: filepath.Base(a.CWD),
		})
	}
	return list
}

func replyKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("ls"),
			tgbotapi.NewKeyboardButton("tail"),
			tgbotapi.NewKeyboardButton("echo"),
		),
	)
}

func lsInlineKeyboard(names []string) tgbotapi.InlineKeyboardMarkup {
	if len(names) > 10 {
		names = names[:10]
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	// Agent tail buttons — 3 per row
	var row []tgbotapi.InlineKeyboardButton
	for i, name := range names {
		cb := formatCallback("t", name)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("📜 %s", name), cb))
		if (i+1)%3 == 0 || i == len(names)-1 {
			rows = append(rows, row)
			row = nil
		}
	}
	// Refresh row
	cb := formatCallback("ls")
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", cb),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func agentPickerKeyboard(agents []agentInfo, action string) tgbotapi.InlineKeyboardMarkup {
	if len(agents) > 10 {
		agents = agents[:10]
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, a := range agents {
		cb := formatCallback(action, a.name)
		label := fmt.Sprintf("%s %s — %s", a.icon, a.name, a.project)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, cb),
		))
	}
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func tailInlineKeyboard(name string, verbose bool, autoRefreshing bool) tgbotapi.InlineKeyboardMarkup {
	mode := "c"
	if verbose {
		mode = "v"
	}
	toggleMode := "v"
	toggleLabel := "🔍 Verbose"
	if verbose {
		toggleMode = "c"
		toggleLabel = "📄 Compact"
	}

	row1 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", formatCallback("r", name, mode)),
		tgbotapi.NewInlineKeyboardButtonData(toggleLabel, formatCallback("r", name, toggleMode)),
	)
	row2 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("💬 Echo", formatCallback("e", name)),
		tgbotapi.NewInlineKeyboardButtonData("📋 Back to ls", formatCallback("ls")),
	)

	autoLabel := "⏱️ Auto-refresh (5s)"
	autoCB := formatCallback("a", name, mode)
	if autoRefreshing {
		autoLabel = "⏹️ Stop refresh"
		autoCB = formatCallback("x")
	}
	row3 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(autoLabel, autoCB),
	)

	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{row1, row2, row3},
	}
}

func echoPickerKeyboard(agents []agentInfo) tgbotapi.InlineKeyboardMarkup {
	kb := agentPickerKeyboard(agents, "e")
	kb.InlineKeyboard = append(kb.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", formatCallback("x")),
	))
	return kb
}

func echoPromptKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", formatCallback("x")),
			),
		},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/jpoz/Developer/taskmaster && go test ./bot/ -run "TestReplyKeyboard|TestLsKeyboard|TestAgentPicker|TestTailKeyboard|TestEchoPicker|TestEchoPrompt" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bot/keyboard.go bot/keyboard_test.go
git commit -m "feat(bot): add reply and inline keyboard builders"
```

### Task 3: Chat state management

**Files:**
- Create: `bot/state.go`
- Create: `bot/state_test.go`

- [ ] **Step 1: Write tests for chat state**

```go
// bot/state_test.go
package bot

import (
	"testing"
	"time"
)

func TestStateManager_GetOrCreate(t *testing.T) {
	sm := newStateManager()
	s1 := sm.get(123)
	if s1.AwaitingEchoFrom != "" {
		t.Error("new state should have empty AwaitingEchoFrom")
	}
	// Same chat returns same state
	s2 := sm.get(123)
	s2.AwaitingEchoFrom = "ace-fox"
	if sm.get(123).AwaitingEchoFrom != "ace-fox" {
		t.Error("should return same state object")
	}
}

func TestStateManager_ClearEcho(t *testing.T) {
	sm := newStateManager()
	s := sm.get(123)
	s.AwaitingEchoFrom = "ace-fox"
	sm.clearEcho(123)
	if sm.get(123).AwaitingEchoFrom != "" {
		t.Error("clearEcho should reset AwaitingEchoFrom")
	}
}

func TestStateManager_DifferentChats(t *testing.T) {
	sm := newStateManager()
	sm.get(1).AwaitingEchoFrom = "a"
	sm.get(2).AwaitingEchoFrom = "b"
	if sm.get(1).AwaitingEchoFrom != "a" {
		t.Error("chat 1 state should be independent")
	}
	if sm.get(2).AwaitingEchoFrom != "b" {
		t.Error("chat 2 state should be independent")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/jpoz/Developer/taskmaster && go test ./bot/ -run "TestStateManager" -v`
Expected: FAIL — `newStateManager` not defined

- [ ] **Step 3: Implement state manager**

```go
// bot/state.go
package bot

import (
	"sync"
	"time"
)

// ChatState holds per-chat conversational state.
type ChatState struct {
	AwaitingEchoFrom string
	AutoRefresh      *AutoRefreshState
}

// AutoRefreshState tracks an active auto-refresh goroutine.
type AutoRefreshState struct {
	AgentName string
	MessageID int
	ChatID    int64
	Verbose   bool
	Ticker    *time.Ticker
	Stop      chan struct{}
	Done      chan struct{}
}

type stateManager struct {
	mu     sync.Mutex
	states map[int64]*ChatState
}

func newStateManager() *stateManager {
	return &stateManager{
		states: make(map[int64]*ChatState),
	}
}

func (sm *stateManager) get(chatID int64) *ChatState {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.states[chatID]
	if !ok {
		s = &ChatState{}
		sm.states[chatID] = s
	}
	return s
}

func (sm *stateManager) clearEcho(chatID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s, ok := sm.states[chatID]; ok {
		s.AwaitingEchoFrom = ""
	}
}

// stopAutoRefresh stops any running auto-refresh for the chat and waits for the
// goroutine to exit. Must NOT be called while holding sm.mu.
func (sm *stateManager) stopAutoRefresh(chatID int64) {
	sm.mu.Lock()
	s, ok := sm.states[chatID]
	if !ok || s.AutoRefresh == nil {
		sm.mu.Unlock()
		return
	}
	ar := s.AutoRefresh
	s.AutoRefresh = nil
	sm.mu.Unlock()

	ar.Ticker.Stop()
	close(ar.Stop)
	<-ar.Done
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/jpoz/Developer/taskmaster && go test ./bot/ -run "TestStateManager" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bot/state.go bot/state_test.go
git commit -m "feat(bot): add chat state manager with echo and auto-refresh tracking"
```

## Chunk 2: Refactor bot.go handlers and update loop

### Task 4: Refactor handler return types to response struct

**Files:**
- Modify: `bot/bot.go`
- Modify: `bot/bot_test.go`

- [ ] **Step 1: Update existing tests to work with new response type**

Update `bot/bot_test.go` — the `handleCommand` function will now return a `response` struct. Update `TestHandleCommand_Unknown` and `TestFindAgent` (findAgent changes to prefix match):

```go
// In TestHandleCommand_Unknown, change:
//   reply := b.handleCommand("blah")
//   if !strings.Contains(reply, "Unknown command") {
// To:
//   reply := b.handleCommand("blah")
//   if !strings.Contains(reply.Text, "Unknown command") {

// In TestFindAgent, add a prefix-match test case:
//   if a := findAgent(agents, "ace-fo"); a == nil || a.Name != "ace-fox" {
//       t.Error("should find ace-fox by prefix")
//   }
```

Also update `TestFormatMessages_*` tests — those call `formatMessages` directly which still returns a string, so no change needed there.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/jpoz/Developer/taskmaster && go test ./bot/ -v`
Expected: FAIL — `handleCommand` still returns `string`

- [ ] **Step 3: Refactor bot.go**

Changes to `bot/bot.go`:

1. Add `response` struct
2. Change `handleCommand` to return `response`
3. Change `helpText` to return `response`
4. Change `handleLs` to return `response` with `lsInlineKeyboard`
5. Change `handleTail` to return `response` with agent picker or `tailInlineKeyboard`
6. Change `handleEcho` to return `response` (text commands still work for backwards compat)
7. Add `/start` case to command router
8. Change `findAgent` to use `strings.HasPrefix`
9. Refactor `send` to accept `response` and attach inline keyboard
10. Add `sendWithReplyKeyboard` for the `/start` welcome message
11. Add `editMessage` for auto-refresh
12. Update `Run()` loop to handle `CallbackQuery` updates
13. Add `states` field to `Bot` struct

Full replacement for `bot/bot.go`:

```go
package bot

import (
	"fmt"
	"html"
	"os"
	"strings"
	"time"

	"github.com/jpoz/taskmaster/agent"
	"github.com/jpoz/taskmaster/tty"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type response struct {
	Text   string
	Inline *tgbotapi.InlineKeyboardMarkup
}

type Bot struct {
	api    *tgbotapi.BotAPI
	userID int64
	states *stateManager
}

func New(token string, userID int64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}
	return &Bot{api: api, userID: userID, states: newStateManager()}, nil
}

func (b *Bot) Run() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	fmt.Printf("Bot connected as @%s\n", b.api.Self.UserName)

	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinIdx := 0
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r%s Listening...", spinner[spinIdx%len(spinner)])
				spinIdx++
			}
		}
	}()

	for update := range updates {
		if update.CallbackQuery != nil {
			if update.CallbackQuery.From.ID != b.userID {
				continue
			}
			b.handleCallback(update.CallbackQuery)
			continue
		}

		if update.Message == nil {
			continue
		}
		if update.Message.From.ID != b.userID {
			continue
		}

		chatID := update.Message.Chat.ID
		text := strings.TrimSpace(update.Message.Text)

		// Stop any running auto-refresh on text input
		b.states.stopAutoRefresh(chatID)

		// Check if we're awaiting echo text
		state := b.states.get(chatID)
		if state.AwaitingEchoFrom != "" && text != "ls" && text != "tail" && text != "echo" && text != "/start" {
			reply := b.handleEchoText(chatID, text)
			b.send(chatID, reply)
			continue
		}

		// Clear echo state if user types a command
		b.states.clearEcho(chatID)

		reply := b.handleCommand(text)
		if text == "/start" {
			b.sendWithReplyKeyboard(chatID, reply)
		} else {
			b.send(chatID, reply)
		}
	}

	close(stop)
	fmt.Fprintf(os.Stderr, "\r")
	return nil
}

func (b *Bot) handleCommand(text string) response {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return helpText()
	}

	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "/start":
		return response{Text: "👋 <b>Taskmaster</b> ready.\nUse the buttons below to get started."}
	case "ls":
		return b.handleLs()
	case "tail":
		return b.handleTail(parts[1:])
	case "echo":
		return b.handleEcho(parts[1:])
	default:
		return response{Text: "Unknown command.\n\n" + helpText().Text}
	}
}

func helpText() response {
	return response{
		Text: "<b>Commands:</b>\n" +
			"<code>ls</code> — list active agents\n" +
			"<code>tail [id]</code> — recent messages\n" +
			"<code>tail -v [id]</code> — verbose (incl. tool/thinking)\n" +
			"<code>echo [id] [msg]</code> — send text to agent",
	}
}

func (b *Bot) handleLs() response {
	agents, err := agent.DiscoverAgents()
	if err != nil {
		return response{Text: fmt.Sprintf("Error discovering agents: %v", html.EscapeString(err.Error()))}
	}
	if len(agents) == 0 {
		return response{Text: "<i>No active agents found.</i>"}
	}

	// Group agents by working directory.
	groups := make(map[string][]agent.Agent)
	var order []string
	for _, a := range agents {
		if _, seen := groups[a.CWD]; !seen {
			order = append(order, a.CWD)
		}
		groups[a.CWD] = append(groups[a.CWD], a)
	}

	var sb strings.Builder
	var names []string
	sb.WriteString("<b>Active Agents</b>\n\n")
	for _, cwd := range order {
		fmt.Fprintf(&sb, "📁 <code>%s</code>\n", html.EscapeString(cwd))
		for _, a := range groups[cwd] {
			status := a.Status()
			names = append(names, a.Name)
			fmt.Fprintf(&sb, "  %s <b>%s</b> <i>%s</i> <code>%s</code>\n",
				statusToIcon(status),
				html.EscapeString(a.Name),
				html.EscapeString(string(status)),
				html.EscapeString(string(a.Type)),
			)
		}
		sb.WriteString("\n")
	}

	kb := lsInlineKeyboard(names)
	return response{Text: sb.String(), Inline: &kb}
}

func (b *Bot) handleTail(args []string) response {
	if len(args) == 0 {
		// No agent specified — show picker
		return b.tailAgentPicker()
	}

	verbose := false
	id := args[0]
	if id == "-v" {
		verbose = true
		if len(args) < 2 {
			return b.tailAgentPicker()
		}
		id = args[1]
	}

	return b.tailAgent(id, verbose)
}

func (b *Bot) tailAgentPicker() response {
	agents, err := agent.DiscoverAgents()
	if err != nil {
		return response{Text: fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))}
	}
	if len(agents) == 0 {
		return response{Text: "<i>No active agents found.</i>"}
	}
	infos := agentInfoList(agents)
	kb := agentPickerKeyboard(infos, "t")
	return response{Text: "<b>Which agent?</b>", Inline: &kb}
}

func (b *Bot) tailAgent(id string, verbose bool) response {
	agents, err := agent.DiscoverAgents()
	if err != nil {
		return response{Text: fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))}
	}

	a := findAgent(agents, id)
	if a == nil {
		return response{Text: fmt.Sprintf("Agent not found: <code>%s</code>", html.EscapeString(id))}
	}

	if a.SessionPath == "" {
		return response{Text: fmt.Sprintf("No session data available for <code>%s</code>.", html.EscapeString(a.Name))}
	}

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
		return response{Text: fmt.Sprintf("Could not read session for %s.", a.Name)}
	}

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

	kb := tailInlineKeyboard(a.Name, verbose, false)
	return response{Text: formatMessages(msgs, a.Type), Inline: &kb}
}

func (b *Bot) handleEcho(args []string) response {
	if len(args) < 2 {
		// No args or just agent name — show picker
		return b.echoAgentPicker()
	}

	id := args[0]
	msg := strings.Join(args[1:], " ")

	agents, err := agent.DiscoverAgents()
	if err != nil {
		return response{Text: fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))}
	}

	a := findAgent(agents, id)
	if a == nil {
		return response{Text: fmt.Sprintf("Unknown agent: <code>%s</code>. Use <code>ls</code> to see active agents.", html.EscapeString(id))}
	}

	result, err := tty.SendToAgent(a.PID, a.TTY, a.SessionID, a.CWD, msg)
	if err != nil {
		if strings.Contains(err.Error(), "no longer running") {
			return response{Text: fmt.Sprintf("Agent <code>%s</code> is no longer running.", html.EscapeString(id))}
		}
		return response{Text: fmt.Sprintf("Error sending to <code>%s</code>: %v", html.EscapeString(id), html.EscapeString(err.Error()))}
	}

	return response{Text: fmt.Sprintf("Sent to <b>%s</b> via %s: <code>%s</code>", html.EscapeString(a.Name), result.Method, html.EscapeString(msg))}
}

func (b *Bot) echoAgentPicker() response {
	agents, err := agent.DiscoverAgents()
	if err != nil {
		return response{Text: fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))}
	}
	if len(agents) == 0 {
		return response{Text: "<i>No active agents found.</i>"}
	}
	infos := agentInfoList(agents)
	kb := echoPickerKeyboard(infos)
	return response{Text: "<b>Send to which agent?</b>", Inline: &kb}
}

func (b *Bot) handleEchoText(chatID int64, text string) response {
	state := b.states.get(chatID)
	agentName := state.AwaitingEchoFrom
	b.states.clearEcho(chatID)

	agents, err := agent.DiscoverAgents()
	if err != nil {
		return response{Text: fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))}
	}

	a := findAgent(agents, agentName)
	if a == nil {
		return response{Text: fmt.Sprintf("Agent <code>%s</code> is no longer available.", html.EscapeString(agentName))}
	}

	result, err := tty.SendToAgent(a.PID, a.TTY, a.SessionID, a.CWD, text)
	if err != nil {
		return response{Text: fmt.Sprintf("Error sending to <code>%s</code>: %v", html.EscapeString(agentName), html.EscapeString(err.Error()))}
	}

	return response{Text: fmt.Sprintf("Sent to <b>%s</b> via %s: <code>%s</code>", html.EscapeString(agentName), result.Method, html.EscapeString(text))}
}

func statusToIcon(s agent.Status) string {
	switch s {
	case agent.StatusWorking:
		return "🟢"
	case agent.StatusWaiting:
		return "🟡"
	case agent.StatusIdle:
		return "⚪"
	default:
		return "🔴"
	}
}

func findAgent(agents []agent.Agent, id string) *agent.Agent {
	// Exact match first
	for i := range agents {
		if agents[i].Name == id {
			return &agents[i]
		}
	}
	// Prefix match for truncated callback data
	for i := range agents {
		if strings.HasPrefix(agents[i].Name, id) {
			return &agents[i]
		}
	}
	return nil
}

func formatMessages(msgs []agent.Message, agentType agent.AgentType) string {
	if len(msgs) == 0 {
		return "<i>No messages.</i>"
	}

	assistantLabel := "🤖 claude"
	if agentType == agent.TypeCodex {
		assistantLabel = "🤖 codex"
	}

	var sb strings.Builder
	for _, m := range msgs {
		label := "👤 you"
		if m.Role == "assistant" {
			if m.IsThinking {
				label = "💭 thinking"
			} else if m.IsToolUse {
				label = "🔧 tool"
			} else {
				label = assistantLabel
			}
		}

		content := m.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		content = html.EscapeString(content)

		fmt.Fprintf(&sb, "<b>%s</b>\n%s\n\n", label, content)
	}
	return sb.String()
}

func (b *Bot) send(chatID int64, r response) {
	text := r.Text
	if len(text) > 4000 {
		text = text[:4000] + "\n..."
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	if r.Inline != nil {
		msg.ReplyMarkup = r.Inline
	}
	b.api.Send(msg)
}

func (b *Bot) sendWithReplyKeyboard(chatID int64, r response) {
	text := r.Text
	if len(text) > 4000 {
		text = text[:4000] + "\n..."
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	kb := replyKeyboard()
	msg.ReplyMarkup = kb
	b.api.Send(msg)
}

func (b *Bot) editMessage(chatID int64, messageID int, r response) error {
	text := r.Text
	if len(text) > 4000 {
		text = text[:4000] + "\n..."
	}
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	if r.Inline != nil {
		edit.ReplyMarkup = r.Inline
	}
	_, err := b.api.Send(edit)
	return err
}
```

- [ ] **Step 4: Update tests for new signatures**

In `bot/bot_test.go`, update:
- `TestHandleCommand_Unknown`: check `reply.Text` instead of `reply`
- `TestFindAgent`: add prefix-match case

```go
func TestHandleCommand_Unknown(t *testing.T) {
	b := &Bot{states: newStateManager()}
	reply := b.handleCommand("blah")
	if !strings.Contains(reply.Text, "Unknown command") {
		t.Errorf("expected unknown command response, got: %s", reply.Text)
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

func TestHandleCommand_Start(t *testing.T) {
	b := &Bot{states: newStateManager()}
	reply := b.handleCommand("/start")
	if !strings.Contains(reply.Text, "Taskmaster") {
		t.Errorf("expected welcome text, got: %s", reply.Text)
	}
}

func TestHandleCommand_TailNoPicker(t *testing.T) {
	b := &Bot{states: newStateManager()}
	// tail with no args should return a response (picker attempt, will show "no agents" in test)
	reply := b.handleCommand("tail")
	// Should not be a usage error — should attempt picker
	if strings.Contains(reply.Text, "Usage") {
		t.Error("tail with no args should show picker, not usage")
	}
}

func TestHandleCommand_EchoNoPicker(t *testing.T) {
	b := &Bot{states: newStateManager()}
	// echo with no args should return a response (picker attempt)
	reply := b.handleCommand("echo")
	if strings.Contains(reply.Text, "Usage") {
		t.Error("echo with no args should show picker, not usage")
	}
}
```

- [ ] **Step 5: Run all tests**

Run: `cd /Users/jpoz/Developer/taskmaster && go test ./bot/ -v`
Expected: PASS (all existing + updated tests)

- [ ] **Step 6: Commit**

```bash
git add bot/bot.go bot/bot_test.go
git commit -m "refactor(bot): change handlers to return response struct with inline keyboards"
```

## Chunk 3: Callback router and auto-refresh

### Task 5: Callback router

**Files:**
- Create: `bot/callback.go`
- Create: `bot/callback_test.go`

- [ ] **Step 1: Write tests for callback routing**

```go
// bot/callback_test.go
package bot

import "testing"

func TestCallbackRouting_ParsesActions(t *testing.T) {
	tests := []struct {
		data       string
		wantAction string
	}{
		{"ls", "ls"},
		{"t:ace-fox", "t"},
		{"r:ace-fox:v", "r"},
		{"e:ace-fox", "e"},
		{"a:ace-fox:c", "a"},
		{"x", "x"},
	}
	for _, tt := range tests {
		action, _ := parseCallback(tt.data)
		if action != tt.wantAction {
			t.Errorf("parseCallback(%q) action = %q, want %q", tt.data, action, tt.wantAction)
		}
	}
}
```

- [ ] **Step 2: Implement callback router**

```go
// bot/callback.go
package bot

import (
	"fmt"
	"html"
	"strings"

	"github.com/jpoz/taskmaster/agent"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleCallback(cq *tgbotapi.CallbackQuery) {
	chatID := cq.Message.Chat.ID

	// Stop any running auto-refresh on any callback
	b.states.stopAutoRefresh(chatID)

	// Clear echo state on any callback
	b.states.clearEcho(chatID)

	action, args := parseCallback(cq.Data)

	var resp response
	switch action {
	case "ls":
		resp = b.handleLs()
	case "t":
		if len(args) > 0 {
			resp = b.tailAgent(args[0], false)
		} else {
			resp = b.tailAgentPicker()
		}
	case "r":
		if len(args) >= 1 {
			name := args[0]
			verbose := len(args) >= 2 && args[1] == "v"
			resp = b.tailAgent(name, verbose)
		} else {
			resp = b.tailAgentPicker()
		}
	case "e":
		if len(args) > 0 {
			// Agent selected for echo — enter awaiting state
			agentName := args[0]
			agents, err := agent.DiscoverAgents()
			if err != nil {
				resp = response{Text: fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))}
				break
			}
			a := findAgent(agents, agentName)
			if a == nil {
				resp = response{Text: fmt.Sprintf("Agent not found: <code>%s</code>", html.EscapeString(agentName))}
				break
			}
			state := b.states.get(chatID)
			state.AwaitingEchoFrom = a.Name
			kb := echoPromptKeyboard()
			resp = response{
				Text:   fmt.Sprintf("💬 <b>Send to %s</b>\nType your message below:", html.EscapeString(a.Name)),
				Inline: &kb,
			}
		} else {
			resp = b.echoAgentPicker()
		}
	case "a":
		if len(args) >= 1 {
			name := args[0]
			verbose := len(args) >= 2 && args[1] == "v"
			b.startAutoRefresh(chatID, cq.Message.MessageID, name, verbose)
			// Answer callback only, don't send new message
			callback := tgbotapi.NewCallback(cq.ID, "Auto-refresh started")
			b.api.Send(callback)
			return
		}
		resp = b.tailAgentPicker()
	case "x":
		// Cancel — already cleared echo and stopped auto-refresh above
		resp = b.handleLs()
	default:
		resp = response{Text: "Unknown action."}
	}

	// Answer the callback to dismiss the spinner
	callback := tgbotapi.NewCallback(cq.ID, "")
	b.api.Send(callback)

	// Edit the existing message in place (natural inline-button UX).
	// For echo prompt, send a new message since it's a new conversational context.
	if action == "e" && len(args) > 0 {
		b.send(chatID, resp)
	} else {
		err := b.editMessage(chatID, cq.Message.MessageID, resp)
		if err != nil && strings.Contains(err.Error(), "message is not modified") {
			// Content unchanged, ignore
		} else if err != nil {
			// Edit failed (e.g., message too old), fall back to new message
			b.send(chatID, resp)
		}
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/jpoz/Developer/taskmaster && go test ./bot/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add bot/callback.go bot/callback_test.go
git commit -m "feat(bot): add callback query router for inline button handling"
```

### Task 6: Auto-refresh implementation

**Files:**
- Modify: `bot/state.go`
- Modify: `bot/callback.go`
- Modify: `bot/state_test.go`

- [ ] **Step 1: Write tests for auto-refresh lifecycle**

```go
// Append to bot/state_test.go

func TestStateManager_StopAutoRefresh_NoOp(t *testing.T) {
	sm := newStateManager()
	// Should not panic when no auto-refresh is running
	sm.stopAutoRefresh(123)
}

func TestStateManager_StopAutoRefresh_StopsGoroutine(t *testing.T) {
	sm := newStateManager()
	s := sm.get(123)
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	s.AutoRefresh = &AutoRefreshState{
		AgentName: "test",
		MessageID: 1,
		ChatID:    123,
		Ticker:    time.NewTicker(time.Hour), // won't actually tick
		Stop:      stopCh,
		Done:      doneCh,
	}
	// Simulate goroutine
	go func() {
		<-stopCh
		close(doneCh)
	}()

	sm.stopAutoRefresh(123)
	// If we get here, Done was closed and we didn't deadlock
	if sm.get(123).AutoRefresh != nil {
		t.Error("AutoRefresh should be nil after stop")
	}
}
```

- [ ] **Step 2: Run tests to verify they pass** (the state tests should pass with existing code)

Run: `cd /Users/jpoz/Developer/taskmaster && go test ./bot/ -run "TestStateManager" -v`
Expected: PASS

- [ ] **Step 3: Add `startAutoRefresh` method to bot**

Add to `bot/state.go`:

```go
// startAutoRefresh begins a background goroutine that periodically re-fetches
// tail output and edits the Telegram message in place.
func (b *Bot) startAutoRefresh(chatID int64, messageID int, agentName string, verbose bool) {
	// Stop any existing auto-refresh first
	b.states.stopAutoRefresh(chatID)

	ar := &AutoRefreshState{
		AgentName: agentName,
		MessageID: messageID,
		ChatID:    chatID,
		Verbose:   verbose,
		Ticker:    time.NewTicker(5 * time.Second),
		Stop:      make(chan struct{}),
		Done:      make(chan struct{}),
	}

	s := b.states.get(chatID)
	b.states.mu.Lock()
	s.AutoRefresh = ar
	b.states.mu.Unlock()

	go b.autoRefreshLoop(ar)
}

func (b *Bot) autoRefreshLoop(ar *AutoRefreshState) {
	defer close(ar.Done)
	for {
		select {
		case <-ar.Stop:
			return
		case <-ar.Ticker.C:
			resp := b.tailAgent(ar.AgentName, ar.Verbose)
			if strings.Contains(resp.Text, "Agent not found") {
				// Agent disconnected — update message and stop
				kb := lsInlineKeyboard(nil)
				disconnected := response{
					Text:   fmt.Sprintf("<i>Agent <code>%s</code> disconnected.</i>", html.EscapeString(ar.AgentName)),
					Inline: &kb,
				}
				b.editMessage(ar.ChatID, ar.MessageID, disconnected)
				return
			}
			// Override the keyboard to show "Stop refresh" instead of "Start"
			kb := tailInlineKeyboard(ar.AgentName, ar.Verbose, true)
			resp.Inline = &kb
			err := b.editMessage(ar.ChatID, ar.MessageID, resp)
			// Silently ignore "message is not modified" errors
			if err != nil && !strings.Contains(err.Error(), "message is not modified") {
				return
			}
		}
	}
}
```

Replace the import block in `state.go` with:

```go
import (
	"fmt"
	"html"
	"strings"
	"sync"
	"time"
)
```

Note: `startAutoRefresh` is a method on `Bot` not `stateManager` because it needs access to `b.tailAgent` and `b.editMessage`. But it uses `stateManager` internally. The `stateManager.mu` lock is used carefully — acquired to read/write state, released before channel operations.

- [ ] **Step 4: Run all tests**

Run: `cd /Users/jpoz/Developer/taskmaster && go test ./bot/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bot/state.go bot/state_test.go bot/callback.go
git commit -m "feat(bot): add auto-refresh goroutine for live tail updates"
```

### Task 7: Final integration — verify build and run all tests

**Files:** (no changes — verification only)

- [ ] **Step 1: Run full test suite**

Run: `cd /Users/jpoz/Developer/taskmaster && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 2: Verify build**

Run: `cd /Users/jpoz/Developer/taskmaster && go build ./...`
Expected: Clean build, no errors

- [ ] **Step 3: Run vet and check for issues**

Run: `cd /Users/jpoz/Developer/taskmaster && go vet ./...`
Expected: No issues
