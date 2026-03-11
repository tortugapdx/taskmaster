package agent

// Message is a human-readable conversation message extracted from a session.
type Message struct {
	Role       string // "user" or "assistant"
	Content    string // text content
	IsToolUse  bool   // true if this is a tool_use block
	IsThinking bool   // true if this is a thinking block
}
