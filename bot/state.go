package bot

import (
	"sync"
	"time"
)

type ChatState struct {
	AwaitingEchoFrom string
	AutoRefresh      *AutoRefreshState
}

type AutoRefreshState struct {
	AgentName string
	MessageID int
	ChatID    int64
	Verbose   bool
	Ticker    *time.Ticker
	Stop      chan struct{}
	Done      chan struct{}
}

type stateManager struct {
	mu     sync.Mutex
	states map[int64]*ChatState
}

func newStateManager() *stateManager {
	return &stateManager{
		states: make(map[int64]*ChatState),
	}
}

func (sm *stateManager) get(chatID int64) *ChatState {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.states[chatID]
	if !ok {
		s = &ChatState{}
		sm.states[chatID] = s
	}
	return s
}

func (sm *stateManager) clearEcho(chatID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s, ok := sm.states[chatID]; ok {
		s.AwaitingEchoFrom = ""
	}
}

func (sm *stateManager) stopAutoRefresh(chatID int64) {
	sm.mu.Lock()
	s, ok := sm.states[chatID]
	if !ok || s.AutoRefresh == nil {
		sm.mu.Unlock()
		return
	}
	ar := s.AutoRefresh
	s.AutoRefresh = nil
	sm.mu.Unlock()

	ar.Ticker.Stop()
	close(ar.Stop)
	<-ar.Done
}
