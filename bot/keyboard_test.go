package bot

import (
	"testing"
)

func TestFormatCallback(t *testing.T) {
	tests := []struct {
		action string
		args   []string
		want   string
	}{
		{"ls", nil, "ls"},
		{"t", []string{"ace-fox"}, "t:ace-fox"},
		{"r", []string{"ace-fox", "v"}, "r:ace-fox:v"},
		{"x", nil, "x"},
	}
	for _, tt := range tests {
		got := formatCallback(tt.action, tt.args...)
		if got != tt.want {
			t.Errorf("formatCallback(%q, %v) = %q, want %q", tt.action, tt.args, got, tt.want)
		}
	}
}

func TestFormatCallback_TruncatesLongNames(t *testing.T) {
	longName := "my-very-long-project-directory-name-that-exceeds-fifty-chars-fox"
	cb := formatCallback("t", longName)
	if len(cb) > 64 {
		t.Errorf("callback data exceeds 64 bytes: %d bytes: %q", len(cb), cb)
	}
}

func TestParseCallback(t *testing.T) {
	tests := []struct {
		data       string
		wantAction string
		wantArgs   []string
	}{
		{"ls", "ls", nil},
		{"t:ace-fox", "t", []string{"ace-fox"}},
		{"r:ace-fox:v", "r", []string{"ace-fox", "v"}},
		{"x", "x", nil},
	}
	for _, tt := range tests {
		action, args := parseCallback(tt.data)
		if action != tt.wantAction {
			t.Errorf("parseCallback(%q) action = %q, want %q", tt.data, action, tt.wantAction)
		}
		if len(args) != len(tt.wantArgs) {
			t.Errorf("parseCallback(%q) args len = %d, want %d", tt.data, len(args), len(tt.wantArgs))
			continue
		}
		for i, a := range args {
			if a != tt.wantArgs[i] {
				t.Errorf("parseCallback(%q) args[%d] = %q, want %q", tt.data, i, a, tt.wantArgs[i])
			}
		}
	}
}

