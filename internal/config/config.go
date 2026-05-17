package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type Config struct {
	ListenAddr     string                      `json:"listen_addr"`
	APIToken       string                      `json:"api_token"`
	Target         Target                      `json:"target"`
	BranchPatterns []BranchPattern             `json:"branch_patterns"`
	Installations  map[string]GitHubAppInstall `json:"installations"`
}

type Target struct {
	Owner            string `json:"owner"`
	Repo             string `json:"repo"`
	BaseRef          string `json:"base_ref"`
	Version          string `json:"version"`
	ReleaseNotesPath string `json:"release_notes_path"`
}

type BranchPattern struct {
	Pattern       string `json:"pattern"`
	CreateTag     bool   `json:"create_tag"`
	TagName       string `json:"tag_name"`
	CreateRelease bool   `json:"create_release"`
}

type GitHubAppInstall struct {
	AppID          int64  `json:"app_id"`
	InstallationID int64  `json:"installation_id"`
	PrivateKeyFile string `json:"private_key_file"`
	PrivateKeyEnv  string `json:"private_key_env"`
}

func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("PIPERY_RELEASE_CONFIG is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.APIToken == "" {
		cfg.APIToken = os.Getenv("PIPERY_RELEASE_API_TOKEN")
	}
	if len(cfg.BranchPatterns) == 0 {
		return Config{}, errors.New("at least one branch pattern is required")
	}
	if len(cfg.Installations) == 0 {
		return Config{}, errors.New("at least one GitHub App installation is required")
	}
	for key, install := range cfg.Installations {
		if install.AppID == 0 || install.InstallationID == 0 {
			return Config{}, fmt.Errorf("installation %q requires app_id and installation_id", key)
		}
		if install.PrivateKeyFile == "" && install.PrivateKeyEnv == "" {
			return Config{}, fmt.Errorf("installation %q requires private_key_file or private_key_env", key)
		}
	}

	return cfg, nil
}
