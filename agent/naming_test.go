package agent

import "testing"

func TestGenerateName(t *testing.T) {
	name1 := GenerateName("ace", "abc-123-session", 1000)
	name2 := GenerateName("ace", "abc-123-session", 1000)
	if name1 != name2 {
		t.Errorf("names not stable: %q vs %q", name1, name2)
	}

	if len(name1) < 4 {
		t.Errorf("name too short: %q", name1)
	}

	name3 := GenerateName("ace", "def-456-session", 1000)
	if name1 == name3 {
		t.Errorf("different sessions got same name: %q", name1)
	}

	name4 := GenerateName("venture", "abc-123-session", 1000)
	if name1 == name4 {
		t.Errorf("different dirs got same name: %q vs %q", name1, name4)
	}

	// Same session, different PIDs = different names
	name5 := GenerateName("ace", "abc-123-session", 2000)
	if name1 == name5 {
		t.Errorf("different PIDs got same name: %q", name1)
	}
}
