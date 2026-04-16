package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type userConfig struct {
	BaseURL            string `json:"cpa_base_url"`
	URL                string `json:"cpa_url"`
	LoginURL           string `json:"login_url"`
	ManagementKey      string `json:"management_key"`
	ManagementPassword string `json:"management_password"`
	Password           string `json:"password"`
}

func loadUserConfig(explicitPath string) (userConfig, error) {
	path, err := resolveConfigPath(explicitPath)
	if err != nil {
		return userConfig{}, err
	}
	if path == "" {
		return userConfig{}, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return userConfig{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg userConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return userConfig{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

func resolveConfigPath(explicitPath string) (string, error) {
	if strings.TrimSpace(explicitPath) != "" {
		return expandHomePath(strings.TrimSpace(explicitPath))
	}

	for _, candidate := range candidateConfigPaths() {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat config %s: %w", candidate, err)
		}
	}
	return "", nil
}

func resolveStatePath(explicitConfigPath string) (string, error) {
	if strings.TrimSpace(explicitConfigPath) != "" {
		path, err := expandHomePath(strings.TrimSpace(explicitConfigPath))
		if err != nil {
			return "", err
		}
		return siblingStatePath(path), nil
	}

	configPath, err := resolveConfigPath("")
	if err != nil {
		return "", err
	}
	if configPath != "" {
		return siblingStatePath(configPath), nil
	}

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		base := strings.TrimSuffix(filepath.Base(exe), filepath.Ext(filepath.Base(exe)))
		return filepath.Join(dir, base+".state.json"), nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "cpa-usage", "state.json"), nil
	}
	return "", nil
}

func candidateConfigPaths() []string {
	paths := []string{}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		base := strings.TrimSuffix(filepath.Base(exe), filepath.Ext(filepath.Base(exe)))
		for _, name := range []string{
			base + ".json",
			"cpausage.json",
			"cpa-quota-inspector.json",
		} {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "cpa-usage", "config.json"))
	}
	return uniqueStrings(paths)
}

func expandHomePath(path string) (string, error) {
	if path == "~" {
		return os.UserHomeDir()
	}
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}

func configBaseURL(cfg userConfig) string {
	return firstNonEmpty(cfg.BaseURL, cfg.URL, cfg.LoginURL)
}

func configManagementKey(cfg userConfig) string {
	return firstNonEmpty(cfg.ManagementKey, cfg.ManagementPassword, cfg.Password)
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func siblingStatePath(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		name = "state"
	}
	return filepath.Join(dir, name+".state.json")
}
