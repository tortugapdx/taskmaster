package bot

import (
	"fmt"
	"html"
	"os"
	"strings"
	"time"

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

	fmt.Printf("Bot connected as @%s\n", b.api.Self.UserName)

	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinIdx := 0
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r%s Listening...", spinner[spinIdx%len(spinner)])
				spinIdx++
			}
		}
	}()

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

	close(stop)
	fmt.Fprintf(os.Stderr, "\r")
	return nil
}

func (b *Bot) handleCommand(text string) string {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return helpText()
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
		return "Unknown command.\n\n" + helpText()
	}
}

func helpText() string {
	return "<b>Commands:</b>\n" +
		"<code>ls</code> — list active agents\n" +
		"<code>tail [id]</code> — recent messages\n" +
		"<code>tail -v [id]</code> — verbose (incl. tool/thinking)\n" +
		"<code>echo [id] [msg]</code> — send text to agent"
}

func (b *Bot) handleLs() string {
	agents, err := agent.DiscoverAgents()
	if err != nil {
		return fmt.Sprintf("Error discovering agents: %v", html.EscapeString(err.Error()))
	}
	if len(agents) == 0 {
		return "<i>No active agents found.</i>"
	}

	// Group agents by working directory.
	groups := make(map[string][]agent.Agent)
	var order []string
	for _, a := range agents {
		if _, seen := groups[a.CWD]; !seen {
			order = append(order, a.CWD)
		}
		groups[a.CWD] = append(groups[a.CWD], a)
	}

	var sb strings.Builder
	sb.WriteString("<b>Active Agents</b>\n\n")
	for _, cwd := range order {
		fmt.Fprintf(&sb, "📁 <code>%s</code>\n", html.EscapeString(cwd))
		for _, a := range groups[cwd] {
			status := a.Status()
			fmt.Fprintf(&sb, "  %s <b>%s</b> <i>%s</i> <code>%s</code>\n",
				statusToIcon(status),
				html.EscapeString(a.Name),
				html.EscapeString(string(status)),
				html.EscapeString(string(a.Type)),
			)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (b *Bot) handleTail(args []string) string {
	if len(args) == 0 {
		return "Usage: <code>tail [-v] &lt;id&gt;</code>"
	}

	verbose := false
	id := args[0]
	if id == "-v" {
		verbose = true
		if len(args) < 2 {
			return "Usage: <code>tail -v &lt;id&gt;</code>"
		}
		id = args[1]
	}

	agents, err := agent.DiscoverAgents()
	if err != nil {
		return fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))
	}

	a := findAgent(agents, id)
	if a == nil {
		return fmt.Sprintf("Unknown agent: <code>%s</code>. Use <code>ls</code> to see active agents.", html.EscapeString(id))
	}

	if a.SessionPath == "" {
		return fmt.Sprintf("No session data available for <code>%s</code>.", html.EscapeString(id))
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
		return "Usage: <code>echo &lt;id&gt; &lt;message&gt;</code>"
	}

	id := args[0]
	msg := strings.Join(args[1:], " ")

	agents, err := agent.DiscoverAgents()
	if err != nil {
		return fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))
	}

	a := findAgent(agents, id)
	if a == nil {
		return fmt.Sprintf("Unknown agent: <code>%s</code>. Use <code>ls</code> to see active agents.", html.EscapeString(id))
	}

	if a.TTY == "" || a.TTY == "??" {
		return fmt.Sprintf("Agent <code>%s</code> has no TTY attached.", html.EscapeString(id))
	}

	if err := tty.WriteToTTY(a.PID, a.TTY, msg); err != nil {
		if strings.Contains(err.Error(), "no longer running") {
			return fmt.Sprintf("Agent <code>%s</code> is no longer running.", html.EscapeString(id))
		}
		return fmt.Sprintf("Error writing to <code>%s</code>: %v", html.EscapeString(id), html.EscapeString(err.Error()))
	}

	return fmt.Sprintf("Sent to <b>%s</b>: <code>%s</code>", html.EscapeString(id), html.EscapeString(msg))
}

func statusToIcon(s agent.Status) string {
	switch s {
	case agent.StatusWorking:
		return "🟢"
	case agent.StatusWaiting:
		return "🟡"
	case agent.StatusIdle:
		return "⚪"
	default:
		return "🔴"
	}
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
		return "<i>No messages.</i>"
	}

	assistantLabel := "🤖 claude"
	if agentType == agent.TypeCodex {
		assistantLabel = "🤖 codex"
	}

	var sb strings.Builder
	for _, m := range msgs {
		label := "👤 you"
		if m.Role == "assistant" {
			if m.IsThinking {
				label = "💭 thinking"
			} else if m.IsToolUse {
				label = "🔧 tool"
			} else {
				label = assistantLabel
			}
		}

		content := m.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		content = html.EscapeString(content)

		fmt.Fprintf(&sb, "<b>%s</b>\n%s\n\n", label, content)
	}
	return sb.String()
}

func (b *Bot) send(chatID int64, text string) {
	if len(text) > 4000 {
		text = text[:4000] + "\n..."
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	b.api.Send(msg)
}
