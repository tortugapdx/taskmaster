# Taskmaster Design Spec

A Go CLI that provides remote monitoring and control of Claude Code and Codex agents via a Telegram bot.

## Problem

When running multiple Claude Code or Codex sessions across terminals, there's no way to check on them remotely. You have to be at the machine to see what each agent is doing, approve prompts, or send messages.

## Solution

A single `taskmaster` binary that runs a Telegram bot. It scans the local system for active agent processes and lets you interact with them through three commands.

## Commands

### `ls`

Lists all active (running) Claude Code and Codex agents.

Output format:
```
taskmaster-owl  claude  /Users/jpoz/Developer/taskmaster  [working]
ace-fox         codex   /Users/jpoz/Developer/ace         [waiting]
venture-cat     claude  /Users/jpoz/Developer/venture     [idle]
```

Status detection:
- `[working]` — process CPU > ~5% (actively generating or running tools)
- `[waiting]` — low CPU, last JSONL entry is an assistant message (agent finished turn, waiting for user input or permission approval)
- `[idle]` — low CPU, conversation is quiet

### `tail [id]`

Shows the last 10 messages from an agent's conversation. Displays only human-readable content (user text and assistant text responses). Tool use and thinking blocks are omitted. Messages truncated at ~500 chars.

```
[you] can we make air NOT log all the "Watching" paths?

[claude] Set `main_only = true` in the `[log]` section...
```

### `tail -v [id]`

Verbose tail — includes everything: tool use, thinking blocks, tool results.

### `echo [id] [msg]`

Writes a message to an active agent by injecting text into its TTY device. The text appears in the agent's terminal as if typed by the user.

Works for both free-text input and prompt responses (e.g., `echo ace-fox y` to approve a permission).

Only works on active agents. User should `tail` after to see the response.

## Agent Naming

Each agent gets a stable, human-friendly ID: `<dir>-<word>`

- `<dir>` is the basename of the agent's working directory
- `<word>` is deterministically derived from a hash of the session ID, picked from an embedded wordlist
- The ID is stable for the lifetime of that session

If two agents share the same directory basename, they get different words (e.g., `ace-fox`, `ace-owl`).

## Agent Discovery

On each command (stateless, no background tracking):

1. Run `ps` to find processes matching `claude` or `codex` (excluding helpers like `ShipIt`, `app-server`, `Codex.app`)
2. For each process, extract PID, TTY, and CWD (via `lsof -p <pid>` cwd entry)
3. For Claude Code: find the most recently modified `.jsonl` in `~/.claude/projects/<encoded-cwd>/` to get the session ID
4. For Codex: query `~/.codex/state_5.sqlite` threads table matching by CWD, most recent `updated_at`
5. Generate the stable `<dir>-<word>` name from the session ID

## Session Data Sources

### Claude Code
- Session files: `~/.claude/projects/<encoded-cwd>/<session-id>.jsonl`
- JSONL format: each line is a JSON object with `type` field
  - `type: "user"` — user messages, `.message.content` is string or array
  - `type: "assistant"` — assistant messages, `.message.content` is array of `{type: "text", text: "..."}`, `{type: "tool_use", ...}`, `{type: "thinking", ...}`
- Encoded CWD format: path with `/` replaced by `-` (e.g., `-Users-jpoz-Developer-ace`)

### Codex
- Thread metadata: `~/.codex/state_5.sqlite` `threads` table (id, cwd, title, updated_at)
- Session files: `~/.codex/sessions/<year>/<month>/<day>/rollout-<date>-<thread-id>.jsonl`
- JSONL format: each line has `{type, payload, timestamp}`
  - `type: "response_item"` with `payload.role: "user"` — user messages
  - `type: "event_msg"` with `payload.type: "agent_message"` — assistant text messages
  - `type: "response_item"` with `payload.type: "function_call"` — tool use

## TTY Writing

For `echo`, write to the agent's TTY device:
```go
f, _ := os.OpenFile("/dev/ttys010", os.O_WRONLY, 0)
f.Write([]byte(msg + "\n"))
f.Close()
```

The TTY device path comes from `ps` output (e.g., `ttys010` -> `/dev/ttys010`).

## Config

Stored at `~/.config/taskmaster/config.json`:
```json
{
  "telegram_bot_token": "123:ABC...",
  "telegram_user_id": 12345678
}
```

On first run (or if config is missing/incomplete), interactive setup:
```
Welcome to Taskmaster!

Enter your Telegram Bot Token (from @BotFather): ▌
Enter your Telegram User ID: ▌

Config saved to ~/.config/taskmaster/config.json
Starting bot...
```

All Telegram messages from non-matching user IDs are silently ignored.

## Project Structure

```
taskmaster/
├── main.go
├── go.mod
├── config/
│   └── config.go          # Load/save config, interactive setup
├── agent/
│   ├── discovery.go        # Process scanning, CWD/TTY extraction
│   ├── session.go          # JSONL parsing for Claude Code + Codex
│   ├── naming.go           # Deterministic <dir>-<word> ID generation
│   └── agent.go            # Agent struct, status detection
├── bot/
│   └── bot.go              # Telegram long-polling, command routing
├── tty/
│   └── writer.go           # TTY device writing
└── wordlist/
    └── words.go            # Embedded word list for naming
```

## Dependencies

- `github.com/go-telegram-bot-api/telegram-bot-api/v5` — Telegram bot API
- `modernc.org/sqlite` — pure Go SQLite (no cgo)

## Security

- Telegram bot only responds to the configured user ID
- TTY writing is limited to devices owned by the current user
- Config file permissions should be 0600
