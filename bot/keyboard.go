package bot

import "strings"

const maxCallbackData = 64
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
