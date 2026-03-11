package agent

import "testing"

func TestGenerateName(t *testing.T) {
	name1 := GenerateName("ace", 1000)
	name2 := GenerateName("ace", 1000)
	if name1 != name2 {
		t.Errorf("names not stable: %q vs %q", name1, name2)
	}

	if len(name1) < 4 {
		t.Errorf("name too short: %q", name1)
	}

	name3 := GenerateName("venture", 1000)
	if name1 == name3 {
		t.Errorf("different dirs got same name: %q vs %q", name1, name3)
	}

	// Same dir, different PIDs = different names
	name4 := GenerateName("ace", 2000)
	if name1 == name4 {
		t.Errorf("different PIDs got same name: %q", name1)
	}
}
