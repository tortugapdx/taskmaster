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

type CodexThread struct {
	ID          string
	RolloutPath string
	CWD         string
}

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

type codexJSONLEntry struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type codexResponsePayload struct {
	Type    string              `json:"type"`
	Role    string              `json:"role"`
	Content []codexContentBlock `json:"content"`
	Name    string              `json:"name"`
}

type codexContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type codexEventPayload struct {
	Type         string `json:"type"`
	AgentMessage string `json:"agent_message"`
}

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

func CodexSessionState(path string) (SessionState, error) {
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
				state.LastEntryType = "user"
				pendingToolUse = false
			} else if payload.Type == "function_call" {
				state.LastEntryType = "assistant"
				pendingToolUse = true
			}
		case "event_msg":
			var payload codexEventPayload
			if err := json.Unmarshal(entry.Payload, &payload); err != nil {
				continue
			}
			if payload.Type == "agent_message" {
				state.LastEntryType = "assistant"
				pendingToolUse = false
			}
		}
	}

	state.PendingToolUse = pendingToolUse
	return state, scanner.Err()
}
