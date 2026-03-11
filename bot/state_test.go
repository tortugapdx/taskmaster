package bot

import (
	"testing"
	"time"
)

func TestStateManager_GetOrCreate(t *testing.T) {
	sm := newStateManager()
	s1 := sm.get(123)
	if s1.AwaitingEchoFrom != "" {
		t.Error("new state should have empty AwaitingEchoFrom")
	}
	s2 := sm.get(123)
	s2.AwaitingEchoFrom = "ace-fox"
	if sm.get(123).AwaitingEchoFrom != "ace-fox" {
		t.Error("should return same state object")
	}
}

func TestStateManager_ClearEcho(t *testing.T) {
	sm := newStateManager()
	s := sm.get(123)
	s.AwaitingEchoFrom = "ace-fox"
	sm.clearEcho(123)
	if sm.get(123).AwaitingEchoFrom != "" {
		t.Error("clearEcho should reset AwaitingEchoFrom")
	}
}

func TestStateManager_DifferentChats(t *testing.T) {
	sm := newStateManager()
	sm.get(1).AwaitingEchoFrom = "a"
	sm.get(2).AwaitingEchoFrom = "b"
	if sm.get(1).AwaitingEchoFrom != "a" {
		t.Error("chat 1 state should be independent")
	}
	if sm.get(2).AwaitingEchoFrom != "b" {
		t.Error("chat 2 state should be independent")
	}
}

func TestStateManager_StopAutoRefresh_NoOp(t *testing.T) {
	sm := newStateManager()
	sm.stopAutoRefresh(123)
}

func TestStateManager_StopAutoRefresh_StopsGoroutine(t *testing.T) {
	sm := newStateManager()
	s := sm.get(123)
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	s.AutoRefresh = &AutoRefreshState{
		AgentName: "test",
		MessageID: 1,
		ChatID:    123,
		Ticker:    time.NewTicker(time.Hour),
		Stop:      stopCh,
		Done:      doneCh,
	}
	go func() {
		<-stopCh
		close(doneCh)
	}()

	sm.stopAutoRefresh(123)
	if sm.get(123).AutoRefresh != nil {
		t.Error("AutoRefresh should be nil after stop")
	}
}
