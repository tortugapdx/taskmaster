package tty

import "testing"

func TestSanitizeMessage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello world"},
		{"hello\x1b[31mworld", "helloworld"},
		{"hello\x00world", "helloworld"},
		{"hello\nworld", "hello\nworld"},
		{"hello\tworld", "hello\tworld"},
		{"\x1b]0;title\x07text", "text"},
	}
	for _, tt := range tests {
		got := SanitizeMessage(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeMessage(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTTYDevicePath(t *testing.T) {
	tests := []struct {
		tty  string
		want string
	}{
		{"ttys010", "/dev/ttys010"},
		{"ttys018", "/dev/ttys018"},
		{"??", ""},
	}
	for _, tt := range tests {
		got := DevicePath(tt.tty)
		if got != tt.want {
			t.Errorf("DevicePath(%q) = %q, want %q", tt.tty, got, tt.want)
		}
	}
}
