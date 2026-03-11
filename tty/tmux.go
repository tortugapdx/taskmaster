package tty

import (
	"fmt"
	"os/exec"
	"strings"
)

// FindTmuxPane checks if the given TTY belongs to a tmux pane.
// Returns the pane target (e.g., "main:0.2") or empty string if not in tmux.
func FindTmuxPane(ttyName string) (string, error) {
	devPath := DevicePath(ttyName)
	if devPath == "" {
		return "", nil
	}

	out, err := exec.Command("tmux", "list-panes", "-a", "-F", "#{pane_tty} #{session_name}:#{window_index}.#{pane_index}").Output()
	if err != nil {
		// tmux not running or not installed
		return "", nil
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 && parts[0] == devPath {
			return parts[1], nil
		}
	}

	return "", nil
}

// SendViaTmux sends a message to a tmux pane using send-keys.
// Uses -l for literal text to avoid key name interpretation, then sends Enter separately.
func SendViaTmux(pane string, msg string) error {
	sanitized := SanitizeMessage(msg)

	// Send the literal text
	cmd := exec.Command("tmux", "send-keys", "-t", pane, "-l", sanitized)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux send-keys (text): %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Send Enter to submit
	cmd = exec.Command("tmux", "send-keys", "-t", pane, "Enter")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux send-keys (enter): %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}
