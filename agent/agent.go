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
