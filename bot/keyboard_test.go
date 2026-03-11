package bot

import (
	"fmt"
	"testing"
)

func TestFormatCallback(t *testing.T) {
	tests := []struct {
		action string
		args   []string
		want   string
	}{
		{"ls", nil, "ls"},
		{"t", []string{"ace-fox"}, "t:ace-fox"},
		{"r", []string{"ace-fox", "v"}, "r:ace-fox:v"},
		{"x", nil, "x"},
	}
	for _, tt := range tests {
		got := formatCallback(tt.action, tt.args...)
		if got != tt.want {
			t.Errorf("formatCallback(%q, %v) = %q, want %q", tt.action, tt.args, got, tt.want)
		}
	}
}

func TestFormatCallback_TruncatesLongNames(t *testing.T) {
	longName := "my-very-long-project-directory-name-that-exceeds-fifty-chars-fox"
	cb := formatCallback("t", longName)
	if len(cb) > 64 {
		t.Errorf("callback data exceeds 64 bytes: %d bytes: %q", len(cb), cb)
	}
}

func TestParseCallback(t *testing.T) {
	tests := []struct {
		data       string
		wantAction string
		wantArgs   []string
	}{
		{"ls", "ls", nil},
		{"t:ace-fox", "t", []string{"ace-fox"}},
		{"r:ace-fox:v", "r", []string{"ace-fox", "v"}},
		{"x", "x", nil},
	}
	for _, tt := range tests {
		action, args := parseCallback(tt.data)
		if action != tt.wantAction {
			t.Errorf("parseCallback(%q) action = %q, want %q", tt.data, action, tt.wantAction)
		}
		if len(args) != len(tt.wantArgs) {
			t.Errorf("parseCallback(%q) args len = %d, want %d", tt.data, len(args), len(tt.wantArgs))
			continue
		}
		for i, a := range args {
			if a != tt.wantArgs[i] {
				t.Errorf("parseCallback(%q) args[%d] = %q, want %q", tt.data, i, a, tt.wantArgs[i])
			}
		}
	}
}

func TestReplyKeyboard(t *testing.T) {
	kb := replyKeyboard()
	if len(kb.Keyboard) != 1 {
		t.Fatalf("expected 1 row, got %d", len(kb.Keyboard))
	}
	if len(kb.Keyboard[0]) != 3 {
		t.Fatalf("expected 3 buttons, got %d", len(kb.Keyboard[0]))
	}
	if kb.Keyboard[0][0].Text != "ls" {
		t.Errorf("first button = %q, want %q", kb.Keyboard[0][0].Text, "ls")
	}
	if !kb.ResizeKeyboard {
		t.Error("ResizeKeyboard should be true")
	}
}

func TestLsKeyboard(t *testing.T) {
	names := []string{"ace-fox", "web-owl"}
	kb := lsInlineKeyboard(names)
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(kb.InlineKeyboard))
	}
	if len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 2 buttons in first row, got %d", len(kb.InlineKeyboard[0]))
	}
	if *kb.InlineKeyboard[0][0].CallbackData != "t:ace-fox" {
		t.Errorf("first button callback = %q, want %q", *kb.InlineKeyboard[0][0].CallbackData, "t:ace-fox")
	}
	lastRow := kb.InlineKeyboard[len(kb.InlineKeyboard)-1]
	if *lastRow[0].CallbackData != "ls" {
		t.Errorf("refresh button callback = %q, want %q", *lastRow[0].CallbackData, "ls")
	}
}

func TestLsKeyboard_CapsAt10(t *testing.T) {
	names := make([]string, 15)
	for i := range names {
		names[i] = fmt.Sprintf("agent-%d", i)
	}
	kb := lsInlineKeyboard(names)
	agentButtons := 0
	for _, row := range kb.InlineKeyboard[:len(kb.InlineKeyboard)-1] {
		agentButtons += len(row)
	}
	if agentButtons > 10 {
		t.Errorf("expected at most 10 agent buttons, got %d", agentButtons)
	}
}

func TestAgentPickerKeyboard(t *testing.T) {
	agents := []agentInfo{
		{name: "ace-fox", icon: "🟢", project: "taskmaster"},
		{name: "web-owl", icon: "🟡", project: "webapp"},
	}
	kb := agentPickerKeyboard(agents, "t")
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(kb.InlineKeyboard))
	}
	if *kb.InlineKeyboard[0][0].CallbackData != "t:ace-fox" {
		t.Errorf("callback = %q, want %q", *kb.InlineKeyboard[0][0].CallbackData, "t:ace-fox")
	}
}

func TestTailKeyboard(t *testing.T) {
	kb := tailInlineKeyboard("ace-fox", false, false)
	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(kb.InlineKeyboard))
	}
	if len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("row 1: expected 2 buttons, got %d", len(kb.InlineKeyboard[0]))
	}
	if len(kb.InlineKeyboard[1]) != 2 {
		t.Fatalf("row 2: expected 2 buttons, got %d", len(kb.InlineKeyboard[1]))
	}
	if len(kb.InlineKeyboard[2]) != 1 {
		t.Fatalf("row 3: expected 1 button, got %d", len(kb.InlineKeyboard[2]))
	}
}

func TestTailKeyboard_VerboseToggle(t *testing.T) {
	compact := tailInlineKeyboard("ace-fox", false, false)
	verbose := tailInlineKeyboard("ace-fox", true, false)
	compactBtn := compact.InlineKeyboard[0][1].Text
	verboseBtn := verbose.InlineKeyboard[0][1].Text
	if compactBtn == verboseBtn {
		t.Error("verbose toggle button text should differ between modes")
	}
}

func TestTailKeyboard_AutoRefreshToggle(t *testing.T) {
	normal := tailInlineKeyboard("ace-fox", false, false)
	refreshing := tailInlineKeyboard("ace-fox", false, true)
	normalBtn := normal.InlineKeyboard[2][0].Text
	refreshBtn := refreshing.InlineKeyboard[2][0].Text
	if normalBtn == refreshBtn {
		t.Error("auto-refresh button text should differ when active")
	}
}

func TestEchoPickerKeyboard(t *testing.T) {
	agents := []agentInfo{
		{name: "ace-fox", icon: "🟢", project: "taskmaster"},
	}
	kb := echoPickerKeyboard(agents)
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(kb.InlineKeyboard))
	}
	lastRow := kb.InlineKeyboard[len(kb.InlineKeyboard)-1]
	if *lastRow[0].CallbackData != "x" {
		t.Errorf("cancel button callback = %q, want %q", *lastRow[0].CallbackData, "x")
	}
}

func TestEchoPromptKeyboard(t *testing.T) {
	kb := echoPromptKeyboard()
	if len(kb.InlineKeyboard) != 1 {
		t.Fatalf("expected 1 row, got %d", len(kb.InlineKeyboard))
	}
	if *kb.InlineKeyboard[0][0].CallbackData != "x" {
		t.Errorf("cancel button callback = %q, want %q", *kb.InlineKeyboard[0][0].CallbackData, "x")
	}
}

