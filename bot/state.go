package bot

import (
	"fmt"
	"html"
	"strings"
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

// startAutoRefresh begins a background goroutine that periodically re-fetches
// tail output and edits the Telegram message in place.
func (b *Bot) startAutoRefresh(chatID int64, messageID int, agentName string, verbose bool) {
	// Stop any existing auto-refresh first
	b.states.stopAutoRefresh(chatID)

	ar := &AutoRefreshState{
		AgentName: agentName,
		MessageID: messageID,
		ChatID:    chatID,
		Verbose:   verbose,
		Ticker:    time.NewTicker(5 * time.Second),
		Stop:      make(chan struct{}),
		Done:      make(chan struct{}),
	}

	s := b.states.get(chatID)
	b.states.mu.Lock()
	s.AutoRefresh = ar
	b.states.mu.Unlock()

	go b.autoRefreshLoop(ar)
}

func (b *Bot) autoRefreshLoop(ar *AutoRefreshState) {
	defer close(ar.Done)
	for {
		select {
		case <-ar.Stop:
			return
		case <-ar.Ticker.C:
			resp := b.tailAgent(ar.AgentName, ar.Verbose)
			if strings.Contains(resp.Text, "Agent not found") {
				// Agent disconnected — update message and stop
				kb := lsInlineKeyboard(nil)
				disconnected := response{
					Text:   fmt.Sprintf("<i>Agent <code>%s</code> disconnected.</i>", html.EscapeString(ar.AgentName)),
					Inline: &kb,
				}
				b.editMessage(ar.ChatID, ar.MessageID, disconnected)
				return
			}
			// Override the keyboard to show "Stop refresh" instead of "Start"
			kb := tailInlineKeyboard(ar.AgentName, ar.Verbose, true)
			resp.Inline = &kb
			err := b.editMessage(ar.ChatID, ar.MessageID, resp)
			// Silently ignore "message is not modified" errors
			if err != nil && !strings.Contains(err.Error(), "message is not modified") {
				return
			}
		}
	}
}
