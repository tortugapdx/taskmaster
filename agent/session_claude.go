package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

func encodeCWD(cwd string) string {
	return strings.ReplaceAll(cwd, "/", "-")
}

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
	Name     string `json:"name"`
	Content  string `json:"content"`
}

func ParseClaudeMessages(path string, n int) ([]Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

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

	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all, scanner.Err()
}

func parseClaudeContent(role string, raw json.RawMessage) []Message {
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		if str == "" || strings.HasPrefix(str, "<local-command-caveat>") || strings.HasPrefix(str, "<command-name>") {
			return nil
		}
		return []Message{{Role: role, Content: str}}
	}

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

func ClaudeSessionState(path string) (SessionState, error) {
	f, err := os.Open(path)
	if err != nil {
		return SessionState{}, err
	}
	defer f.Close()

	var state SessionState
	pendingToolUse := false

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		var entry claudeJSONLEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		if entry.Type == "assistant" {
			state.LastEntryType = "assistant"
			pendingToolUse = entry.Message != nil && claudeHasToolUse(entry.Message)
		} else if entry.Type == "user" {
			state.LastEntryType = "user"
			// A user entry following an assistant clears the pending flag
			// (tool_result comes as a "user" type entry).
			pendingToolUse = false
		}
	}

	state.PendingToolUse = pendingToolUse
	return state, scanner.Err()
}

// claudeHasToolUse reports whether an assistant message's content array
// contains at least one tool_use block.
func claudeHasToolUse(raw json.RawMessage) bool {
	var msg claudeMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return false
	}
	var blocks []claudeContentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return false
	}
	for _, b := range blocks {
		if b.Type == "tool_use" {
			return true
		}
	}
	return false
}
