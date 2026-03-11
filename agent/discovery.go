package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type processInfo struct {
	PID       int
	TTY       string
	Args      string
	AgentType AgentType
}

func DiscoverAgents() ([]Agent, error) {
	procs, err := findAgentProcesses()
	if err != nil {
		return nil, err
	}
	if len(procs) == 0 {
		return nil, nil
	}

	cwds, err := resolveCWDs(procs)
	if err != nil {
		return nil, err
	}

	home, _ := os.UserHomeDir()
	claudeProjectsDir := filepath.Join(home, ".claude", "projects")
	codexDir := filepath.Join(home, ".codex")

	var agents []Agent
	for _, p := range procs {
		cwd, ok := cwds[p.PID]
		if !ok {
			continue
		}

		a := Agent{
			Type: p.AgentType,
			CWD:  cwd,
			PID:  p.PID,
			TTY:  p.TTY,
		}

		dirBase := filepath.Base(cwd)

		switch a.Type {
		case TypeClaude:
			sessionID, sessionPath, err := FindClaudeSession(claudeProjectsDir, cwd)
			if err == nil {
				a.SessionID = sessionID
				a.SessionPath = sessionPath
				info, _ := os.Stat(sessionPath)
				if info != nil {
					a.SessionModTime = info.ModTime()
				}
				a.LastEntryType, _ = ClaudeLastEntryType(sessionPath)
			}
			if a.SessionID == "" {
				a.SessionID = fmt.Sprintf("pid-%d", p.PID)
			}
			a.Name = GenerateName(dirBase, a.SessionID)

		case TypeCodex:
			thread, err := FindCodexSession(codexDir, cwd)
			if err == nil {
				a.SessionID = thread.ID
				a.SessionPath = thread.RolloutPath
				info, _ := os.Stat(thread.RolloutPath)
				if info != nil {
					a.SessionModTime = info.ModTime()
				}
				a.LastEntryType, _ = CodexLastEntryType(thread.RolloutPath)
			}
			if a.SessionID == "" {
				a.SessionID = fmt.Sprintf("pid-%d", p.PID)
			}
			a.Name = GenerateName(dirBase, a.SessionID)
		}

		agents = append(agents, a)
	}

	return agents, nil
}

func findAgentProcesses() ([]processInfo, error) {
	out, err := exec.Command("ps", "-eo", "pid,tty,args").Output()
	if err != nil {
		return nil, fmt.Errorf("running ps: %w", err)
	}

	var procs []processInfo
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "PID") {
			continue
		}
		if !strings.Contains(line, "claude") && !strings.Contains(line, "codex") {
			continue
		}

		pid, tty, args, err := parsePsLine(line)
		if err != nil {
			continue
		}

		agentType, skip := classifyArgs(args)
		if skip {
			continue
		}

		procs = append(procs, processInfo{PID: pid, TTY: tty, Args: args, AgentType: agentType})
	}
	return procs, nil
}

func parsePsLine(line string) (pid int, tty string, args string, err error) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return 0, "", "", fmt.Errorf("too few fields: %q", line)
	}
	pid, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, "", "", fmt.Errorf("parsing pid: %w", err)
	}
	tty = fields[1]
	args = strings.Join(fields[2:], " ")
	return pid, tty, args, nil
}

func classifyArgs(args string) (agentType AgentType, skip bool) {
	skipPatterns := []string{"ShipIt", "app-server", "Codex.app", "Claude.app"}
	for _, pat := range skipPatterns {
		if strings.Contains(args, pat) {
			return "", true
		}
	}

	firstToken := strings.Fields(args)[0]
	base := filepath.Base(firstToken)

	if base == "claude" || strings.Contains(firstToken, ".claude/local/") {
		return TypeClaude, false
	}
	if base == "codex" || strings.Contains(firstToken, "codex") {
		return TypeCodex, false
	}

	return "", true
}

func resolveCWDs(procs []processInfo) (map[int]string, error) {
	pids := make([]string, len(procs))
	for i, p := range procs {
		pids[i] = strconv.Itoa(p.PID)
	}

	out, err := exec.Command("lsof", "-a", "-d", "cwd", "-p", strings.Join(pids, ",")).Output()
	if err != nil {
		if len(out) == 0 {
			return nil, fmt.Errorf("lsof failed: %w", err)
		}
	}

	cwds := make(map[int]string)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "COMMAND") {
			continue
		}
		pid, cwd, err := parseLsofLine(line)
		if err != nil {
			continue
		}
		cwds[pid] = cwd
	}
	return cwds, nil
}

func parseLsofLine(line string) (pid int, cwd string, err error) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return 0, "", fmt.Errorf("too few fields: %q", line)
	}
	pid, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, "", fmt.Errorf("parsing pid: %w", err)
	}
	cwd = strings.Join(fields[8:], " ")
	return pid, cwd, nil
}
