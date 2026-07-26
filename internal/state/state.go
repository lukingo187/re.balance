// Package state loads and saves the app-owned state.json: the current market
// value of each position (keyed by ISIN, in cents). Unlike config.toml this is
// rewritten freely.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State is a parsed state.json. Values are the source of truth for current market
// values.
type State struct {
	Updated string           `json:"updated"`
	Values  map[string]int64 `json:"values"`
}

// DefaultPath returns $XDG_STATE_HOME/rebal/state.json, falling back to
// ~/.local/state/rebal/state.json when XDG_STATE_HOME is unset.
func DefaultPath() (string, error) {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "rebal", "state.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "rebal", "state.json"), nil
}

// Load reads state.json. A missing file is not an error — it yields empty state,
// so every value defaults to 0.
func Load(path string) (State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{Values: map[string]int64{}}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if s.Values == nil {
		s.Values = map[string]int64{}
	}
	return s, nil
}

// Save writes state.json, creating parent directories as needed.
func Save(path string, s State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// SetValue records a current value (cents) for an ISIN and stamps Updated.
func (s *State) SetValue(isin string, cents int64, now time.Time) {
	if s.Values == nil {
		s.Values = map[string]int64{}
	}
	s.Values[isin] = cents
	s.Updated = now.UTC().Format(time.RFC3339)
}
