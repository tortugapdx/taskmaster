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

// SessionState holds parsed state from a session's JSONL log.
type SessionState struct {
	LastEntryType  string // "user" or "assistant"
	PendingToolUse bool   // last assistant message had tool_use without a subsequent tool_result
}

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
	PendingToolUse bool   // last assistant issued tool_use with no tool_result yet
	HasActiveConns bool   // process has active TCP connections (likely API call)
}

func (a Agent) Status() Status {
	if a.SessionModTime.IsZero() {
		return StatusUnknown
	}

	// Recent file activity = definitely working.
	if time.Since(a.SessionModTime) < 5*time.Second {
		return StatusWorking
	}

	// Active network connections = talking to the API.
	if a.HasActiveConns {
		return StatusWorking
	}

	// Last assistant message had tool_use without tool_result = tool
	// executing or waiting for permission.
	if a.PendingToolUse {
		return StatusWorking
	}

	// Last entry was assistant text (no pending tool use) = waiting for user.
	if a.LastEntryType == "assistant" {
		return StatusWaiting
	}

	return StatusIdle
}
