package agent

import "testing"

func TestGenerateName(t *testing.T) {
	name1 := GenerateName("ace", "abc-123-session")
	name2 := GenerateName("ace", "abc-123-session")
	if name1 != name2 {
		t.Errorf("names not stable: %q vs %q", name1, name2)
	}

	if len(name1) < 4 {
		t.Errorf("name too short: %q", name1)
	}

	name3 := GenerateName("ace", "def-456-session")
	if name1 == name3 {
		t.Errorf("different sessions got same name: %q", name1)
	}

	name4 := GenerateName("venture", "abc-123-session")
	if name1 == name4 {
		t.Errorf("different dirs got same name: %q vs %q", name1, name4)
	}
}
