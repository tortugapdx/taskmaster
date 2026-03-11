//go:build integration

package agent

import (
	"testing"
)

func TestDiscoverAgents_Live(t *testing.T) {
	agents, err := DiscoverAgents()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Found %d agents:", len(agents))
	for _, a := range agents {
		t.Logf("  %-16s %-8s %s [%s]", a.Name, a.Type, a.CWD, a.Status())
		t.Logf("    Session: %s", a.SessionPath)
		t.Logf("    TTY: %s  PID: %d", a.TTY, a.PID)
	}
}
