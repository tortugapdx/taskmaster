package bot

import "testing"

func TestCallbackRouting_ParsesActions(t *testing.T) {
	tests := []struct {
		data       string
		wantAction string
	}{
		{"ls", "ls"},
		{"t:ace-fox", "t"},
		{"r:ace-fox:v", "r"},
		{"e:ace-fox", "e"},
		{"a:ace-fox:c", "a"},
		{"x", "x"},
	}
	for _, tt := range tests {
		action, _ := parseCallback(tt.data)
		if action != tt.wantAction {
			t.Errorf("parseCallback(%q) action = %q, want %q", tt.data, action, tt.wantAction)
		}
	}
}
