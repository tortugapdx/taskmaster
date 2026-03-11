package agent

import (
	"testing"
	"time"
)

func TestAgent_Status_Working(t *testing.T) {
	a := Agent{
		SessionModTime: time.Now(),
		LastEntryType:  "assistant",
	}
	if s := a.Status(); s != StatusWorking {
		t.Errorf("got %q, want %q", s, StatusWorking)
	}
}

func TestAgent_Status_Waiting(t *testing.T) {
	a := Agent{
		SessionModTime: time.Now().Add(-30 * time.Second),
		LastEntryType:  "assistant",
	}
	if s := a.Status(); s != StatusWaiting {
		t.Errorf("got %q, want %q", s, StatusWaiting)
	}
}

func TestAgent_Status_Idle(t *testing.T) {
	a := Agent{
		SessionModTime: time.Now().Add(-30 * time.Second),
		LastEntryType:  "user",
	}
	if s := a.Status(); s != StatusIdle {
		t.Errorf("got %q, want %q", s, StatusIdle)
	}
}

func TestAgent_Status_Unknown(t *testing.T) {
	a := Agent{
		SessionModTime: time.Time{},
	}
	if s := a.Status(); s != StatusUnknown {
		t.Errorf("got %q, want %q", s, StatusUnknown)
	}
}
