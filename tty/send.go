package tty

import "fmt"

// SendResult describes how a message was delivered.
type SendResult struct {
	Method string // "tmux" or "cli"
}

// SendToAgent tries to deliver a message to an agent. It attempts tmux
// send-keys first (works when the agent runs in a tmux pane), then falls
// back to the Claude CLI (resumes the session in print mode).
func SendToAgent(pid int, ttyName string, sessionID string, cwd string, msg string) (*SendResult, error) {
	// Strategy 1: tmux send-keys
	if ttyName != "" && ttyName != "??" {
		pane, err := FindTmuxPane(ttyName)
		if err != nil {
			return nil, fmt.Errorf("checking tmux: %w", err)
		}
		if pane != "" {
			if err := SendViaTmux(pane, msg); err != nil {
				return nil, err
			}
			return &SendResult{Method: "tmux"}, nil
		}
	}

	// Strategy 2: Claude CLI fallback
	if err := SendViaCLI(sessionID, cwd, msg); err != nil {
		return nil, fmt.Errorf("all delivery methods failed; cli: %w", err)
	}
	return &SendResult{Method: "cli"}, nil
}
