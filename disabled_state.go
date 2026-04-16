package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type disabledAccountStateFile struct {
	Accounts map[string]disabledAccountState `json:"accounts,omitempty"`
}

type disabledAccountState struct {
	LastProbeDay string `json:"last_probe_day,omitempty"`
}

type disabledAccountTracker struct {
	mu       sync.Mutex
	path     string
	today    string
	accounts map[string]disabledAccountState
}

func loadDisabledAccountTracker(path string, now time.Time) (*disabledAccountTracker, error) {
	tracker := &disabledAccountTracker{
		path:     path,
		today:    probeDay(now),
		accounts: map[string]disabledAccountState{},
	}
	if path == "" {
		return tracker, nil
	}

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return tracker, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read disabled account state %s: %w", path, err)
	}
	if len(raw) == 0 {
		return tracker, nil
	}

	var payload disabledAccountStateFile
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse disabled account state %s: %w", path, err)
	}
	if payload.Accounts != nil {
		tracker.accounts = payload.Accounts
	}
	return tracker, nil
}

func (t *disabledAccountTracker) shouldProbe(name string) bool {
	if t == nil || name == "" {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state, ok := t.accounts[name]
	if !ok {
		return false
	}
	return state.LastProbeDay != t.today
}

func (t *disabledAccountTracker) markAutoDisabled(name string) {
	if t == nil || name == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.accounts[name] = disabledAccountState{LastProbeDay: t.today}
}

func (t *disabledAccountTracker) markProbeAttempt(name string) {
	if t == nil || name == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.accounts[name]
	state.LastProbeDay = t.today
	t.accounts[name] = state
}

func (t *disabledAccountTracker) clear(name string) {
	if t == nil || name == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.accounts, name)
}

func (t *disabledAccountTracker) save() error {
	if t == nil || t.path == "" {
		return nil
	}

	t.mu.Lock()
	payload := disabledAccountStateFile{
		Accounts: make(map[string]disabledAccountState, len(t.accounts)),
	}
	for name, state := range t.accounts {
		payload.Accounts[name] = state
	}
	t.mu.Unlock()

	if len(payload.Accounts) == 0 {
		if err := os.Remove(t.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove disabled account state %s: %w", t.path, err)
		}
		return nil
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal disabled account state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
		return fmt.Errorf("create state dir for %s: %w", t.path, err)
	}
	tmp := t.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write disabled account state %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, t.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace disabled account state %s: %w", t.path, err)
	}
	return nil
}

func probeDay(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	return now.Local().Format("2006-01-02")
}
