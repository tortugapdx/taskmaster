package agent

import "testing"

func TestParsePsLine(t *testing.T) {
	tests := []struct {
		line string
		pid  int
		tty  string
		args string
	}{
		{" 10695 ttys018  claude --dangerously-skip-permissions", 10695, "ttys018", "claude --dangerously-skip-permissions"},
		{" 2569 ??       codex app-server --analytics-default-enabled", 2569, "??", "codex app-server --analytics-default-enabled"},
	}
	for _, tt := range tests {
		pid, tty, args, err := parsePsLine(tt.line)
		if err != nil {
			t.Fatalf("parsePsLine(%q): %v", tt.line, err)
		}
		if pid != tt.pid {
			t.Errorf("pid = %d, want %d", pid, tt.pid)
		}
		if tty != tt.tty {
			t.Errorf("tty = %q, want %q", tty, tt.tty)
		}
		if args != tt.args {
			t.Errorf("args = %q, want %q", args, tt.args)
		}
	}
}

func TestClassifyArgs(t *testing.T) {
	tests := []struct {
		args     string
		wantType AgentType
		wantSkip bool
	}{
		{"claude --dangerously-skip-permissions", TypeClaude, false},
		{"codex", TypeCodex, false},
		{"/Applications/Claude.app/Contents/Frameworks/Squirrel.framework/Resources/ShipIt", "", true},
		{"codex app-server --analytics-default-enabled", "", true},
		{"/Applications/Codex.app/Contents/MacOS/Codex", "", true},
	}
	for _, tt := range tests {
		agentType, skip := classifyArgs(tt.args)
		if skip != tt.wantSkip {
			t.Errorf("classifyArgs(%q) skip = %v, want %v", tt.args, skip, tt.wantSkip)
		}
		if !skip && agentType != tt.wantType {
			t.Errorf("classifyArgs(%q) type = %q, want %q", tt.args, agentType, tt.wantType)
		}
	}
}

func TestParseLsofLine(t *testing.T) {
	line := "2.1.72  10695 jpoz  cwd    DIR   1,16     2176 162287637 /Users/jpoz/Developer/ace"
	pid, cwd, err := parseLsofLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if pid != 10695 {
		t.Errorf("pid = %d, want 10695", pid)
	}
	if cwd != "/Users/jpoz/Developer/ace" {
		t.Errorf("cwd = %q", cwd)
	}
}
