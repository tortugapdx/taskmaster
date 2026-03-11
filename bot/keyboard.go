package bot

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jpoz/taskmaster/agent"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const maxNameLen = 50

// formatCallback builds a callback data string like "action:arg1:arg2".
// Agent names are truncated to maxNameLen to stay within Telegram's 64-byte limit.
func formatCallback(action string, args ...string) string {
	if len(args) == 0 {
		return action
	}
	for i, a := range args {
		if len(a) > maxNameLen {
			args[i] = a[:maxNameLen]
		}
	}
	return action + ":" + strings.Join(args, ":")
}

// parseCallback splits callback data into action and arguments.
func parseCallback(data string) (string, []string) {
	parts := strings.SplitN(data, ":", 3)
	if len(parts) == 1 {
		return parts[0], nil
	}
	return parts[0], parts[1:]
}

type agentInfo struct {
	name    string
	icon    string
	project string
}

func agentInfoList(agents []agent.Agent) []agentInfo {
	var list []agentInfo
	for _, a := range agents {
		list = append(list, agentInfo{
			name:    a.Name,
			icon:    statusToIcon(a.Status()),
			project: filepath.Base(a.CWD),
		})
	}
	return list
}

func replyKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("ls"),
			tgbotapi.NewKeyboardButton("tail"),
			tgbotapi.NewKeyboardButton("echo"),
		),
	)
}

func lsInlineKeyboard(names []string) tgbotapi.InlineKeyboardMarkup {
	if len(names) > 10 {
		names = names[:10]
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton
	for i, name := range names {
		cb := formatCallback("t", name)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("📜 %s", name), cb))
		if (i+1)%3 == 0 || i == len(names)-1 {
			rows = append(rows, row)
			row = nil
		}
	}
	cb := formatCallback("ls")
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", cb),
	))
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func agentPickerKeyboard(agents []agentInfo, action string) tgbotapi.InlineKeyboardMarkup {
	if len(agents) > 10 {
		agents = agents[:10]
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, a := range agents {
		cb := formatCallback(action, a.name)
		label := fmt.Sprintf("%s %s — %s", a.icon, a.name, a.project)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, cb),
		))
	}
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func tailInlineKeyboard(name string, verbose bool, autoRefreshing bool) tgbotapi.InlineKeyboardMarkup {
	mode := "c"
	if verbose {
		mode = "v"
	}
	toggleMode := "v"
	toggleLabel := "🔍 Verbose"
	if verbose {
		toggleMode = "c"
		toggleLabel = "📄 Compact"
	}

	row1 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", formatCallback("r", name, mode)),
		tgbotapi.NewInlineKeyboardButtonData(toggleLabel, formatCallback("r", name, toggleMode)),
	)
	row2 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("💬 Echo", formatCallback("e", name)),
		tgbotapi.NewInlineKeyboardButtonData("📋 Back to ls", formatCallback("ls")),
	)

	autoLabel := "⏱️ Auto-refresh (5s)"
	autoCB := formatCallback("a", name, mode)
	if autoRefreshing {
		autoLabel = "⏹️ Stop refresh"
		autoCB = formatCallback("x")
	}
	row3 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(autoLabel, autoCB),
	)

	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{row1, row2, row3},
	}
}

func echoPickerKeyboard(agents []agentInfo) tgbotapi.InlineKeyboardMarkup {
	kb := agentPickerKeyboard(agents, "e")
	kb.InlineKeyboard = append(kb.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", formatCallback("x")),
	))
	return kb
}

func echoPromptKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", formatCallback("x")),
			),
		},
	}
}
