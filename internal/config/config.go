package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultConfigDir  = ".mita"
	DefaultConfigFile = "config.json"
)

type Config struct {
	SourceGitLabURL string          `json:"source_gitlab_url"`
	SourceToken     string          `json:"source_token,omitempty"`
	TargetGitLabURL string          `json:"target_gitlab_url"`
	TargetUsername  string          `json:"target_username,omitempty"`
	TargetPassword  string          `json:"target_password,omitempty"`
	InsecureTLS     bool            `json:"insecure_tls"`
	Mappings        ProjectMappings `json:"mappings"`
}

func DefaultConfig() *Config {
	return &Config{
		SourceGitLabURL: "https://192.168.102.104/",
		InsecureTLS:     true,
		Mappings: ProjectMappings{
			Entries: make(map[string]MappingEntry),
		},
	}
}

func configPath(customPath string) (string, error) {
	if customPath != "" {
		return customPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, DefaultConfigDir, DefaultConfigFile), nil
}

func Load(customPath string) (*Config, error) {
	p, err := configPath(customPath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found at %s: run 'airlift' and configure first", p)
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Mappings.Entries == nil {
		cfg.Mappings.Entries = make(map[string]MappingEntry)
	}

	return &cfg, nil
}

func (c *Config) Save(customPath string) error {
	p, err := configPath(customPath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(p, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
