package config

import "testing"

func TestProjectMappings(t *testing.T) {
	m := ProjectMappings{Entries: make(map[string]MappingEntry)}

	// Set
	m.Set("proj-a", MappingEntry{TargetProject: "proj-a-target", TargetGroup: "grp", TargetBranch: "main"})

	// Lookup found
	entry, ok := m.Lookup("proj-a")
	if !ok || entry.TargetProject != "proj-a-target" {
		t.Errorf("expected proj-a-target, got %v", entry)
	}

	// Lookup not found
	_, ok = m.Lookup("nonexistent")
	if ok {
		t.Error("should not find nonexistent mapping")
	}

	// Delete
	m.Delete("proj-a")
	_, ok = m.Lookup("proj-a")
	if ok {
		t.Error("should not find deleted mapping")
	}
}
