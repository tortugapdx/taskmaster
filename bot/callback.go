package bot

import (
	"fmt"
	"html"
	"strings"

	"github.com/jpoz/taskmaster/agent"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleCallback(cq *tgbotapi.CallbackQuery) {
	chatID := cq.Message.Chat.ID

	// Stop any running auto-refresh on any callback
	b.states.stopAutoRefresh(chatID)

	// Clear echo state on any callback
	b.states.clearEcho(chatID)

	action, args := parseCallback(cq.Data)

	var resp response
	switch action {
	case "ls":
		resp = b.handleLs()
	case "t":
		if len(args) > 0 {
			resp = b.tailAgent(args[0], false)
		} else {
			resp = b.tailAgentPicker()
		}
	case "r":
		if len(args) >= 1 {
			name := args[0]
			verbose := len(args) >= 2 && args[1] == "v"
			resp = b.tailAgent(name, verbose)
		} else {
			resp = b.tailAgentPicker()
		}
	case "e":
		if len(args) > 0 {
			agentName := args[0]
			agents, err := agent.DiscoverAgents()
			if err != nil {
				resp = response{Text: fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))}
				break
			}
			a := findAgent(agents, agentName)
			if a == nil {
				resp = response{Text: fmt.Sprintf("Agent not found: <code>%s</code>", html.EscapeString(agentName))}
				break
			}
			state := b.states.get(chatID)
			state.AwaitingEchoFrom = a.Name
			kb := echoPromptKeyboard()
			resp = response{
				Text:   fmt.Sprintf("💬 <b>Send to %s</b>\nType your message below:", html.EscapeString(a.Name)),
				Inline: &kb,
			}
		} else {
			resp = b.echoAgentPicker()
		}
	case "a":
		if len(args) >= 1 {
			name := args[0]
			verbose := len(args) >= 2 && args[1] == "v"
			b.startAutoRefresh(chatID, cq.Message.MessageID, name, verbose)
			callback := tgbotapi.NewCallback(cq.ID, "Auto-refresh started")
			b.api.Send(callback)
			return
		}
		resp = b.tailAgentPicker()
	case "x":
		resp = b.handleLs()
	default:
		resp = response{Text: "Unknown action."}
	}

	// Answer the callback to dismiss the spinner
	callback := tgbotapi.NewCallback(cq.ID, "")
	b.api.Send(callback)

	// Edit the existing message in place (natural inline-button UX).
	// For echo prompt, send a new message since it's a new conversational context.
	if action == "e" && len(args) > 0 {
		b.send(chatID, resp)
	} else {
		err := b.editMessage(chatID, cq.Message.MessageID, resp)
		if err != nil && strings.Contains(err.Error(), "message is not modified") {
			// Content unchanged, ignore
		} else if err != nil {
			// Edit failed (e.g., message too old), fall back to new message
			b.send(chatID, resp)
		}
	}
}
