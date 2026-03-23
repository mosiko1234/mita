package config

type MappingEntry struct {
	TargetProject string `json:"target_project"`
	TargetGroup   string `json:"target_group"`
	TargetBranch  string `json:"target_branch"`
}

type ProjectMappings struct {
	Entries map[string]MappingEntry `json:"mappings"`
}

// Lookup finds the mapping for a source project name.
// The bundlePath can be a full path or just the project name;
// we extract the project name from the bundle filename.
func (m *ProjectMappings) Lookup(sourceProject string) (MappingEntry, bool) {
	entry, ok := m.Entries[sourceProject]
	return entry, ok
}

func (m *ProjectMappings) Set(sourceProject string, entry MappingEntry) {
	if m.Entries == nil {
		m.Entries = make(map[string]MappingEntry)
	}
	m.Entries[sourceProject] = entry
}

func (m *ProjectMappings) Delete(sourceProject string) {
	delete(m.Entries, sourceProject)
}

func (m *ProjectMappings) List() map[string]MappingEntry {
	return m.Entries
}
