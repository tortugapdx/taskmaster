package tty

import "testing"

func TestSendResult(t *testing.T) {
	r := &SendResult{Method: "tmux"}
	if r.Method != "tmux" {
		t.Errorf("got %q, want tmux", r.Method)
	}
}
