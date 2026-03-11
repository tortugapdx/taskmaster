package tty

import "testing"

func TestParseTmuxOutput(t *testing.T) {
	// Test the matching logic used by FindTmuxPane by simulating
	// what tmux list-panes returns. We test the parsing inline since
	// FindTmuxPane shells out to tmux which may not be available in CI.

	lines := []string{
		"/dev/ttys003 main:0.0",
		"/dev/ttys004 main:0.1",
		"/dev/ttys010 work:1.0",
	}

	tests := []struct {
		tty  string
		want string
	}{
		{"ttys003", "main:0.0"},
		{"ttys010", "work:1.0"},
		{"ttys099", ""},
		{"??", ""},
		{"", ""},
	}

	for _, tt := range tests {
		devPath := DevicePath(tt.tty)
		got := ""
		for _, line := range lines {
			if len(line) == 0 {
				continue
			}
			idx := indexOf(line, ' ')
			if idx < 0 {
				continue
			}
			if line[:idx] == devPath {
				got = line[idx+1:]
				break
			}
		}
		if got != tt.want {
			t.Errorf("match(%q) = %q, want %q", tt.tty, got, tt.want)
		}
	}
}

func indexOf(s string, b byte) int {
	for i := range len(s) {
		if s[i] == b {
			return i
		}
	}
	return -1
}
