```
▗▄▄▄▖▗▄▖  ▗▄▄▖▗▖ ▗▖▗▖  ▗▖ ▗▄▖  ▗▄▄▖▗▄▄▄▖▗▄▄▄▖▗▄▄▖
  █ ▐▌ ▐▌▐▌   ▐▌▗▞▘▐▛▚▞▜▌▐▌ ▐▌▐▌     █  ▐▌   ▐▌ ▐▌
  █ ▐▛▀▜▌ ▝▀▚▖▐▛▚▖ ▐▌  ▐▌▐▛▀▜▌ ▝▀▚▖  █  ▐▛▀▀▘▐▛▀▚▖
  █ ▐▌ ▐▌▗▄▄▞▘▐▌ ▐▌▐▌  ▐▌▐▌ ▐▌▗▄▄▞▘  █  ▐▙▄▄▖▐▌ ▐▌
```

Monitor and control your [Claude Code](https://docs.anthropic.com/en/docs/claude-code) and [Codex](https://openai.com/index/introducing-codex/) agent sessions remotely through Telegram.

Taskmaster discovers running AI agents on your machine, lets you view their conversation history, and even inject input into their TTY — all from your phone.

## Features

- **Agent Discovery** — Automatically finds running Claude Code and Codex processes via `ps` and `lsof`
- **Live Status** — See which agents are working, waiting for input, or idle
- **Session Tailing** — Read the last messages from any agent's conversation
- **Remote Input** — Send text to an agent's TTY as if you typed it locally
- **Deterministic Naming** — Each agent gets a memorable name like `ace-owl` or `taskmaster-fox`

## Commands

| Command | Description |
|---------|-------------|
| `ls` | List all active agents grouped by working directory |
| `tail [name]` | Show the last 10 messages from an agent |
| `tail -v [name]` | Verbose tail including tool use, thinking blocks, and results |
| `echo [name] [message]` | Inject text into an agent's TTY |

### Status Icons

| Icon | Meaning |
|------|---------|
| 🟢 | Working — actively processing |
| 🟡 | Waiting — finished speaking, awaiting user input |
| ⚪ | Idle — processed last input, quiet |
| 🔴 | Unknown — no session file found |

> **Note:** For best results, run your AI agents inside [tmux](https://github.com/tmux/tmux). Taskmaster uses `tmux send-keys` to deliver input to agents, which is more reliable than direct TTY writing. Without tmux, the `echo` command falls back to the Claude CLI's `--resume` flag, which works but is less seamless.

## Getting Started

### Prerequisites

- Go 1.25+
- A [Telegram Bot Token](https://core.telegram.org/bots#how-do-i-create-a-bot) (via @BotFather)
- Your Telegram user ID

### Install

```sh
go install github.com/jpoz/taskmaster@latest
```

Or build from source:

```sh
git clone https://github.com/jpoz/taskmaster.git
cd taskmaster
make build
```

### Run

```sh
taskmaster
```

On first run, Taskmaster will prompt you for your Telegram bot token and user ID. Config is saved to `~/.config/taskmaster/config.json`.

## Development

```sh
make dev      # Start with live reload (air)
make build    # Build the binary
make test     # Run tests
make lint     # Run linters
make clean    # Remove build artifacts
```

## How It Works

Taskmaster is stateless — it discovers agents fresh on every command by scanning running processes. No background daemon required.

1. **Discovery** — Scans `ps` output for `claude` and `codex` processes, resolves working directories and TTYs via `lsof`
2. **Session Parsing** — Reads Claude Code JSONL logs (`~/.claude/projects/`) or Codex SQLite databases (`~/.codex/`) to extract conversation history
3. **Status Detection** — Determines agent state from session file recency, TCP connections, and conversation flow
4. **TTY Writing** — Injects input by writing directly to `/dev/<tty>` device files with PID ownership verification

## Project Structure

```
├── main.go           Entry point, config loading, banner
├── agent/
│   ├── discovery.go      Process scanning and agent resolution
│   ├── session_claude.go Claude Code JSONL parsing
│   ├── session_codex.go  Codex SQLite + JSONL parsing
│   └── naming.go         Deterministic name generation
├── bot/
│   └── bot.go            Telegram bot, command routing, formatting
├── config/
│   └── config.go         JSON config, interactive setup
├── tty/
│   └── writer.go         TTY device writing with sanitization
└── wordlist/
    └── words.go          Embedded word list for agent names
```
