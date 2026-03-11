package tty

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SendViaCLI sends a message to a Claude Code session using the CLI.
// This resumes the session in print mode, which appends to the conversation
// but runs as a separate process from the interactive instance.
func SendViaCLI(sessionID string, cwd string, msg string) error {
	if sessionID == "" {
		return fmt.Errorf("no session ID available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sanitized := SanitizeMessage(msg)

	cmd := exec.CommandContext(ctx, "claude", "--resume", sessionID, "--print", sanitized)
	cmd.Dir = cwd

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("claude CLI: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
