package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Manifest contains metadata about a transfer bundle.
type Manifest struct {
	Version         string `json:"version"`
	Tool            string `json:"tool"`
	CreatedAt       string `json:"created_at"`
	SourceProject   string `json:"source_project"`
	SourceGitLab    string `json:"source_gitlab"`
	Branch          string `json:"branch"`
	CommitSHA       string `json:"commit_sha"`
	TransferType    string `json:"transfer_type"` // "shallow" or "full"
	HasLFS          bool   `json:"has_lfs"`
	LFSObjectsCount int    `json:"lfs_objects_count"`
	BundleSizeBytes int64  `json:"bundle_size_bytes"`
}

// NewManifest creates a new manifest with the current timestamp.
func NewManifest(sourceProject, sourceGitLab, branch, commitSHA, transferType string) *Manifest {
	return &Manifest{
		Version:       "1.0",
		Tool:          "mita",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		SourceProject: sourceProject,
		SourceGitLab:  sourceGitLab,
		Branch:        branch,
		CommitSHA:     commitSHA,
		TransferType:  transferType,
	}
}

// WriteToFile saves the manifest as JSON to a file.
func (m *Manifest) WriteToFile(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ReadManifest reads a manifest from a JSON file.
func ReadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	return &m, nil
}
