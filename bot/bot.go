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

type response struct {
	Text   string
	Inline *tgbotapi.InlineKeyboardMarkup
}

type Bot struct {
	api    *tgbotapi.BotAPI
	userID int64
	states *stateManager
}

func New(token string, userID int64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}
	return &Bot{api: api, userID: userID, states: newStateManager()}, nil
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
		if update.CallbackQuery != nil {
			if update.CallbackQuery.From.ID != b.userID {
				continue
			}
			b.handleCallback(update.CallbackQuery)
			continue
		}

		if update.Message == nil {
			continue
		}
		if update.Message.From.ID != b.userID {
			continue
		}

		chatID := update.Message.Chat.ID
		text := strings.TrimSpace(update.Message.Text)

		// Stop any running auto-refresh on text input
		b.states.stopAutoRefresh(chatID)

		// Check if we're awaiting echo text
		state := b.states.get(chatID)
		if state.AwaitingEchoFrom != "" && text != "ls" && text != "tail" && text != "echo" && text != "/start" {
			reply := b.handleEchoText(chatID, text)
			b.send(chatID, reply)
			continue
		}

		// Clear echo state if user types a command
		b.states.clearEcho(chatID)

		reply := b.handleCommand(text)
		parts := strings.Fields(text)
		isKnownCmd := len(parts) > 0 && (parts[0] == "ls" || parts[0] == "tail" || parts[0] == "echo")
		if isKnownCmd {
			b.send(chatID, reply)
		} else {
			// /start, unrecognized text, and empty input re-send the reply keyboard
			b.sendWithReplyKeyboard(chatID, reply)
		}
	}

	close(stop)
	fmt.Fprintf(os.Stderr, "\r")
	return nil
}

func (b *Bot) handleCommand(text string) response {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return helpText()
	}

	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "/start":
		return response{Text: "👋 <b>Taskmaster</b> ready.\nUse the buttons below to get started."}
	case "ls":
		return b.handleLs()
	case "tail":
		return b.handleTail(parts[1:])
	case "echo":
		return b.handleEcho(parts[1:])
	default:
		return response{Text: "Unknown command.\n\n" + helpText().Text}
	}
}

func helpText() response {
	return response{
		Text: "<b>Commands:</b>\n" +
			"<code>ls</code> — list active agents\n" +
			"<code>tail [id]</code> — recent messages\n" +
			"<code>tail -v [id]</code> — verbose (incl. tool/thinking)\n" +
			"<code>echo [id] [msg]</code> — send text to agent",
	}
}

func (b *Bot) handleLs() response {
	agents, err := agent.DiscoverAgents()
	if err != nil {
		return response{Text: fmt.Sprintf("Error discovering agents: %v", html.EscapeString(err.Error()))}
	}
	if len(agents) == 0 {
		return response{Text: "<i>No active agents found.</i>"}
	}

	groups := make(map[string][]agent.Agent)
	var order []string
	for _, a := range agents {
		if _, seen := groups[a.CWD]; !seen {
			order = append(order, a.CWD)
		}
		groups[a.CWD] = append(groups[a.CWD], a)
	}

	var sb strings.Builder
	var names []string
	sb.WriteString("<b>Active Agents</b>\n\n")
	for _, cwd := range order {
		fmt.Fprintf(&sb, "📁 <code>%s</code>\n", html.EscapeString(cwd))
		for _, a := range groups[cwd] {
			status := a.Status()
			names = append(names, a.Name)
			fmt.Fprintf(&sb, "  %s <b>%s</b> <i>%s</i> <code>%s</code>\n",
				statusToIcon(status),
				html.EscapeString(a.Name),
				html.EscapeString(string(status)),
				html.EscapeString(string(a.Type)),
			)
		}
		sb.WriteString("\n")
	}

	kb := lsInlineKeyboard(names)
	return response{Text: sb.String(), Inline: &kb}
}

func (b *Bot) handleTail(args []string) response {
	if len(args) == 0 {
		return b.tailAgentPicker()
	}

	verbose := false
	id := args[0]
	if id == "-v" {
		verbose = true
		if len(args) < 2 {
			return b.tailAgentPicker()
		}
		id = args[1]
	}

	return b.tailAgent(id, verbose)
}

func (b *Bot) tailAgentPicker() response {
	agents, err := agent.DiscoverAgents()
	if err != nil {
		return response{Text: fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))}
	}
	if len(agents) == 0 {
		return response{Text: "<i>No active agents found.</i>"}
	}
	infos := agentInfoList(agents)
	kb := agentPickerKeyboard(infos, "t")
	return response{Text: "<b>Which agent?</b>", Inline: &kb}
}

func (b *Bot) tailAgent(id string, verbose bool) response {
	agents, err := agent.DiscoverAgents()
	if err != nil {
		return response{Text: fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))}
	}

	a := findAgent(agents, id)
	if a == nil {
		return response{Text: fmt.Sprintf("Agent not found: <code>%s</code>", html.EscapeString(id))}
	}

	if a.SessionPath == "" {
		return response{Text: fmt.Sprintf("No session data available for <code>%s</code>.", html.EscapeString(a.Name))}
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
		return response{Text: fmt.Sprintf("Could not read session for %s.", a.Name)}
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

	kb := tailInlineKeyboard(a.Name, verbose, false)
	return response{Text: formatMessages(msgs, a.Type), Inline: &kb}
}

func (b *Bot) handleEcho(args []string) response {
	if len(args) < 2 {
		return b.echoAgentPicker()
	}

	id := args[0]
	msg := strings.Join(args[1:], " ")

	agents, err := agent.DiscoverAgents()
	if err != nil {
		return response{Text: fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))}
	}

	a := findAgent(agents, id)
	if a == nil {
		return response{Text: fmt.Sprintf("Unknown agent: <code>%s</code>. Use <code>ls</code> to see active agents.", html.EscapeString(id))}
	}

	result, err := tty.SendToAgent(a.PID, a.TTY, a.SessionID, a.CWD, msg)
	if err != nil {
		if strings.Contains(err.Error(), "no longer running") {
			return response{Text: fmt.Sprintf("Agent <code>%s</code> is no longer running.", html.EscapeString(id))}
		}
		return response{Text: fmt.Sprintf("Error sending to <code>%s</code>: %v", html.EscapeString(id), html.EscapeString(err.Error()))}
	}

	return response{Text: fmt.Sprintf("Sent to <b>%s</b> via %s: <code>%s</code>", html.EscapeString(a.Name), result.Method, html.EscapeString(msg))}
}

func (b *Bot) echoAgentPicker() response {
	agents, err := agent.DiscoverAgents()
	if err != nil {
		return response{Text: fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))}
	}
	if len(agents) == 0 {
		return response{Text: "<i>No active agents found.</i>"}
	}
	infos := agentInfoList(agents)
	kb := echoPickerKeyboard(infos)
	return response{Text: "<b>Send to which agent?</b>", Inline: &kb}
}

func (b *Bot) handleEchoText(chatID int64, text string) response {
	state := b.states.get(chatID)
	agentName := state.AwaitingEchoFrom
	b.states.clearEcho(chatID)

	agents, err := agent.DiscoverAgents()
	if err != nil {
		return response{Text: fmt.Sprintf("Error: %v", html.EscapeString(err.Error()))}
	}

	a := findAgent(agents, agentName)
	if a == nil {
		return response{Text: fmt.Sprintf("Agent <code>%s</code> is no longer available.", html.EscapeString(agentName))}
	}

	result, err := tty.SendToAgent(a.PID, a.TTY, a.SessionID, a.CWD, text)
	if err != nil {
		return response{Text: fmt.Sprintf("Error sending to <code>%s</code>: %v", html.EscapeString(agentName), html.EscapeString(err.Error()))}
	}

	return response{Text: fmt.Sprintf("Sent to <b>%s</b> via %s: <code>%s</code>", html.EscapeString(agentName), result.Method, html.EscapeString(text))}
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
	for i := range agents {
		if strings.HasPrefix(agents[i].Name, id) {
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

func (b *Bot) send(chatID int64, r response) {
	text := r.Text
	if len(text) > 4000 {
		text = text[:4000] + "\n..."
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	if r.Inline != nil {
		msg.ReplyMarkup = r.Inline
	}
	b.api.Send(msg)
}

func (b *Bot) sendWithReplyKeyboard(chatID int64, r response) {
	text := r.Text
	if len(text) > 4000 {
		text = text[:4000] + "\n..."
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	kb := replyKeyboard()
	msg.ReplyMarkup = kb
	b.api.Send(msg)
}

func (b *Bot) editMessage(chatID int64, messageID int, r response) error {
	text := r.Text
	if len(text) > 4000 {
		text = text[:4000] + "\n..."
	}
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	if r.Inline != nil {
		edit.ReplyMarkup = r.Inline
	}
	_, err := b.api.Send(edit)
	return err
}
