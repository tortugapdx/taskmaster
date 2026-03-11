package tty

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var ansiEscape = regexp.MustCompile(`\x1b(?:\[[0-9;]*[a-zA-Z]|\][^\x07]*\x07)`)

func SanitizeMessage(msg string) string {
	msg = ansiEscape.ReplaceAllString(msg, "")
	var b strings.Builder
	for _, r := range msg {
		if r == '\n' || r == '\t' || r >= 32 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func DevicePath(tty string) string {
	if tty == "??" || tty == "-" || tty == "" {
		return ""
	}
	return "/dev/" + tty
}

func VerifyPIDOwnsTTY(pid int, tty string) error {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "tty=").Output()
	if err != nil {
		return fmt.Errorf("process %d no longer running", pid)
	}
	currentTTY := strings.TrimSpace(string(out))
	if currentTTY != tty {
		return fmt.Errorf("process %d TTY changed from %s to %s", pid, tty, currentTTY)
	}
	return nil
}

