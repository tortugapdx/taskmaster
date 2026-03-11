package bot

import (
	"fmt"
	"strings"

	"github.com/jpoz/taskmaster/agent"
	"github.com/jpoz/taskmaster/tty"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api    *tgbotapi.BotAPI
	userID int64
}

func New(token string, userID int64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}
	return &Bot{api: api, userID: userID}, nil
}

func (b *Bot) Run() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		if update.Message.From.ID != b.userID {
			continue
		}

		text := strings.TrimSpace(update.Message.Text)
		reply := b.handleCommand(text)
		b.send(update.Message.Chat.ID, reply)
	}
	return nil
}

func (b *Bot) handleCommand(text string) string {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "Send: ls, tail [id], or echo [id] [msg]"
	}

	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "ls":
		return b.handleLs()
	case "tail":
		return b.handleTail(parts[1:])
	case "echo":
		return b.handleEcho(parts[1:])
	default:
		return "Unknown command. Send: ls, tail [id], or echo [id] [msg]"
	}
}

func (b *Bot) handleLs() string {
	agents, err := agent.DiscoverAgents()
	if err != nil {
		return fmt.Sprintf("Error discovering agents: %v", err)
	}
	if len(agents) == 0 {
		return "No active agents found."
	}

	var sb strings.Builder
	for _, a := range agents {
		fmt.Fprintf(&sb, "%-16s %-8s %s  [%s]\n", a.Name, a.Type, a.CWD, a.Status())
	}
	return sb.String()
}

func (b *Bot) handleTail(args []string) string {
	if len(args) == 0 {
		return "Usage: tail [-v] <id>"
	}

	verbose := false
	id := args[0]
	if id == "-v" {
		verbose = true
		if len(args) < 2 {
			return "Usage: tail -v <id>"
		}
		id = args[1]
	}

	agents, err := agent.DiscoverAgents()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	a := findAgent(agents, id)
	if a == nil {
		return fmt.Sprintf("Unknown agent: %s. Use ls to see active agents.", id)
	}

	if a.SessionPath == "" {
		return fmt.Sprintf("No session data available for %s.", id)
	}

	fetchCount := 50
	if verbose {
		fetchCount = 10
	}

	var msgs []agent.Message
	switch a.Type {
	case agent.TypeClaude:
		msgs, err = agent.ParseClaudeMessages(a.SessionPath, fetchCount)
	case agent.TypeCodex:
		msgs, err = agent.ParseCodexMessages(a.SessionPath, fetchCount)
	}
	if err != nil {
		return fmt.Sprintf("Could not read session for %s.", id)
	}

	if !verbose {
		var filtered []agent.Message
		for _, m := range msgs {
			if !m.IsToolUse && !m.IsThinking {
				filtered = append(filtered, m)
			}
		}
		msgs = filtered
	}
	if len(msgs) > 10 {
		msgs = msgs[len(msgs)-10:]
	}

	return formatMessages(msgs, a.Type)
}

func (b *Bot) handleEcho(args []string) string {
	if len(args) < 2 {
		return "Usage: echo <id> <message>"
	}

	id := args[0]
	msg := strings.Join(args[1:], " ")

	agents, err := agent.DiscoverAgents()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	a := findAgent(agents, id)
	if a == nil {
		return fmt.Sprintf("Unknown agent: %s. Use ls to see active agents.", id)
	}

	if a.TTY == "" || a.TTY == "??" {
		return fmt.Sprintf("Agent %s has no TTY attached.", id)
	}

	if err := tty.WriteToTTY(a.PID, a.TTY, msg); err != nil {
		if strings.Contains(err.Error(), "no longer running") {
			return fmt.Sprintf("Agent %s is no longer running.", id)
		}
		return fmt.Sprintf("Error writing to %s: %v", id, err)
	}

	return fmt.Sprintf("Sent to %s: %s", id, msg)
}

func findAgent(agents []agent.Agent, id string) *agent.Agent {
	for i := range agents {
		if agents[i].Name == id {
			return &agents[i]
		}
	}
	return nil
}

func formatMessages(msgs []agent.Message, agentType agent.AgentType) string {
	if len(msgs) == 0 {
		return "(no messages)"
	}

	assistantLabel := "[claude]"
	if agentType == agent.TypeCodex {
		assistantLabel = "[codex]"
	}

	var sb strings.Builder
	for _, m := range msgs {
		label := "[you]"
		if m.Role == "assistant" {
			if m.IsThinking {
				label = "[thinking]"
			} else if m.IsToolUse {
				label = "[tool]"
			} else {
				label = assistantLabel
			}
		}

		content := m.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}

		fmt.Fprintf(&sb, "%s %s\n\n", label, content)
	}
	return sb.String()
}

func (b *Bot) send(chatID int64, text string) {
	if len(text) > 4000 {
		text = text[:4000] + "\n..."
	}
	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)
}
