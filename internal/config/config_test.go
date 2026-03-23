package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.SourceGitLabURL == "" {
		t.Error("SourceGitLabURL should have a default")
	}
	if cfg.Mappings.Entries == nil {
		t.Error("Mappings.Entries should be initialized")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := DefaultConfig()
	cfg.SourceGitLabURL = "https://test.example.com"
	cfg.TargetGitLabURL = "https://target.example.com"
	cfg.Mappings.Set("myproject", MappingEntry{
		TargetProject: "myproject-copy",
		TargetGroup:   "mygroup",
		TargetBranch:  "main",
	})

	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.SourceGitLabURL != cfg.SourceGitLabURL {
		t.Errorf("got %s, want %s", loaded.SourceGitLabURL, cfg.SourceGitLabURL)
	}

	entry, ok := loaded.Mappings.Lookup("myproject")
	if !ok {
		t.Fatal("mapping for myproject should exist")
	}
	if entry.TargetProject != "myproject-copy" {
		t.Errorf("got %s, want myproject-copy", entry.TargetProject)
	}
}

func TestLoadNotFound(t *testing.T) {
	_, err := Load(filepath.Join(os.TempDir(), "nonexistent-mita-config.json"))
	if err == nil {
		t.Error("should return error for non-existent config")
	}
}
