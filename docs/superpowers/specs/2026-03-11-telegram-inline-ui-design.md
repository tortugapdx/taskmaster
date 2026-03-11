# Telegram Tap-Friendly UI

Replace typed commands with a persistent reply keyboard and contextual inline buttons so the bot can be operated entirely by tapping on mobile.

## Current State

The bot accepts three text commands: `ls`, `tail [id]`, `echo [id] [msg]`. All require typing. The update loop in `bot.go` only handles `Message` updates and ignores everything else. The library is `go-telegram-bot-api/v5` at v5.5.1.

## Design

### Reply Keyboard

A persistent reply keyboard with three buttons replaces the phone keyboard:

```
[ ls ] [ tail ] [ echo ]
```

Set via `tgbotapi.ReplyKeyboardMarkup` with `ResizeKeyboard: true`. (The `IsPersistent` field is not available in v5.5.1 of the Go library; the keyboard will still persist because `OneTimeKeyboard` defaults to `false`.)

**Important constraint:** Telegram's `reply_markup` accepts either a `ReplyKeyboardMarkup` or an `InlineKeyboardMarkup`, not both on the same message. Strategy:

- The reply keyboard is sent **once** on `/start` (or the very first message) in a welcome message with no inline buttons.
- All subsequent messages use `InlineKeyboardMarkup` only. The reply keyboard persists from the initial send because `OneTimeKeyboard` is `false` — Telegram keeps it visible until explicitly removed.
- If the reply keyboard disappears (e.g., user switches chats on some clients), sending `/start` or any unrecognized text re-sends it.

The reply keyboard sends plain text (`ls`, `tail`, `echo`), which the existing command router already handles. A `/start` case is added to the command router to send the welcome message with the reply keyboard.

### Inline Keyboards Per Response

Each response includes contextual inline buttons for follow-up actions.

**`ls` response:**
- One `[📜 tail <name>]` button per agent (max 10; if more, show first 10)
- One `[🔄 Refresh]` button
- Callback data: `t:<name>`, `ls`

**`tail` agent picker** (when `tail` is tapped without a target):
- One button per agent showing status icon, name, and project basename
- Callback data: `t:<name>`

**`tail` output** (conversation history, starts in compact mode):
- Row 1: `[🔄 Refresh]` `[🔍 Verbose]` (toggles to `[📄 Compact]`)
- Row 2: `[💬 Echo]` `[📋 Back to ls]`
- Row 3: `[⏱️ Auto-refresh (5s)]` (toggles to `[⏹️ Stop refresh]`)
- Callback data: `r:<name>:<v|c>`, `e:<name>`, `ls`, `a:<name>:<v|c>`

**`echo` agent picker:**
- Same as tail picker plus `[❌ Cancel]`
- Callback data: `e:<name>`, `x`

**`echo` prompt** (after picking agent):
- Just `[❌ Cancel]`
- Callback data: `x`

### Callback Data Format

All callback data follows `action:arg1:arg2`. Single-letter action prefixes keep data compact. Telegram's limit is 64 bytes.

**Agent name length:** Agent names are `<dirBasename>-<word>` where the word is 3 characters but the directory basename is unbounded. To stay within 64 bytes, agent names in callback data are truncated to 50 characters. The existing `findAgent()` uses exact match and must be updated to use `strings.HasPrefix` so truncated callback names still resolve. Name collisions from truncation are extremely unlikely given the 50-char budget.

| Callback Data | Meaning |
|---|---|
| `ls` | Refresh agent list |
| `t:<name>` | Show tail for agent (compact) |
| `r:<name>:<v\|c>` | Refresh tail (verbose or compact) |
| `e:<name>` | Start echo flow / pick agent for echo |
| `a:<name>:<v\|c>` | Toggle auto-refresh on/off |
| `x` | Cancel / stop — always clears echo state AND stops auto-refresh |

### Callback Router

The `Run()` loop gains a branch for `update.CallbackQuery != nil`. Auth check: `update.CallbackQuery.From.ID != b.userID` rejects unauthorized users, mirroring the existing message auth. A callback router parses the action prefix and dispatches to handler functions. Each callback is answered with `tgbotapi.NewCallback()` to dismiss the loading spinner on the button.

### Chat State

```go
type ChatState struct {
    AwaitingEchoFrom string           // agent name, empty = not waiting
    AutoRefresh      *AutoRefreshState
}

type AutoRefreshState struct {
    AgentName string
    MessageID int
    ChatID    int64
    Verbose   bool
    Ticker    *time.Ticker
    Stop      chan struct{}
    Done      chan struct{} // closed when goroutine exits
}
```

A `map[int64]*ChatState` keyed by chat ID, protected by a `sync.Mutex`.

**Known limitation:** State is in-memory only and lost on bot restart. A user in the middle of an echo flow will have their next message routed to the command handler instead. This is acceptable for a personal bot.

### Echo Two-Step Flow

1. User taps `💬 echo` (reply keyboard) or `e:<name>` (inline button) → bot discovers agents, shows agent picker with inline buttons
2. User taps agent → bot sets `AwaitingEchoFrom`, replies "Type your message:" with `[❌ Cancel]`
3. User types free text → bot checks `AwaitingEchoFrom` first, before command routing. If set, calls `tty.SendToAgent()` (re-discovering agents to get full Agent struct), clears state, confirms delivery
4. Any callback (including cancel) clears `AwaitingEchoFrom`. Commands typed as text (`ls`, `tail`, `echo`) also clear it and are routed normally — echo-await does not swallow commands.

### Auto-Refresh

1. User taps `[⏱️ Auto-refresh (5s)]` on a tail output
2. Bot starts a goroutine with `time.NewTicker(5 * time.Second)`
3. Each tick: re-discover agents, re-parse session, call `tgbotapi.NewEditMessageText()` to update the existing message in place. The edit sets `ParseMode: tgbotapi.ModeHTML` and re-attaches the inline keyboard (with the stop button). If the message content is unchanged, Telegram returns a "message is not modified" error — this is silently ignored.
4. Button toggles to `[⏹️ Stop refresh]` (callback data: `x`)
5. Stops when:
   - User taps `[⏹️ Stop refresh]` (`x` callback)
   - User taps any other callback button on the auto-refreshing message (Refresh, Verbose, Echo, Back to ls) — the auto-refresh stops first, then the tapped action executes normally as a one-shot
   - User sends any text message (command or otherwise)
   - Agent is no longer found during a tick (edit message to say "agent disconnected", stop ticker)
6. One auto-refresh per chat at a time — starting a new one closes the old `Stop` channel and waits on `Done` before proceeding
7. 5s interval = 12 edits/min, well within Telegram's ~30/min rate limit

### Handler Refactoring

Handlers currently return a `string`. They change to return a `response` struct:

```go
type response struct {
    Text   string
    Inline *tgbotapi.InlineKeyboardMarkup
}
```

The `send()` method is updated to accept a `response` and attach the inline keyboard if present. `ParseMode: tgbotapi.ModeHTML` is always set on both `send()` and `editMessage()`. `helpText()` returns a `response` with no inline keyboard.

For auto-refresh, a separate `editMessage()` method handles `tgbotapi.NewEditMessageText` with `ParseMode` and inline keyboard attachment.

## File Changes

| File | Change |
|---|---|
| `bot/bot.go` | Add `CallbackQuery` branch to `Run()`, add `/start` handler with reply keyboard, refactor `send()` to use `response` struct, update handler return types, change `findAgent()` to use `strings.HasPrefix` for truncated callback names |
| `bot/keyboard.go` | New — reply keyboard builder, inline keyboard builders per response type, callback data formatting/parsing helpers |
| `bot/callback.go` | New — callback router with auth check, dispatches to handler functions |
| `bot/state.go` | New — `ChatState`, state map with mutex, echo flow logic, auto-refresh goroutine lifecycle (start/stop/swap) |

No changes to `agent/`, `tty/`, or `config/`.

## Error Handling

- **Auth on callbacks**: `CallbackQuery.From.ID` checked against `b.userID`, unauthorized queries silently ignored
- **Stale agents**: Callback for an agent no longer running → answer with alert text "Agent not found", no crash
- **Auto-refresh agent gone**: Stop ticker, edit message to "Agent disconnected" with `[📋 Back to ls]` button
- **"Message not modified" error**: Silently ignored during auto-refresh ticks when content hasn't changed
- **Stale message callbacks**: Old inline buttons from before a restart work fine — they trigger fresh agent discovery, and if the agent is gone, the stale-agent path handles it
- **Echo state cleanup**: Cleared on echo completion, cancel callback, any other callback, or any recognized command text
- **Agent picker cap**: `ls` and picker responses cap at 10 agents to avoid unwieldy inline keyboards

## Testing Strategy

- **`bot/callback_test.go`**: Callback data parse/format round-trip, action routing for each prefix
- **`bot/state_test.go`**: Echo flow state transitions (set → clear on send, clear on cancel, clear on command), auto-refresh start/stop/swap lifecycle
- **`bot/keyboard_test.go`**: Inline keyboard builders produce correct button counts and callback data for various agent lists (0, 1, 10+)
