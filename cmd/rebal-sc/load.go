package main

import (
	"os"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/state"
	"github.com/spf13/cobra"
)

// configPath resolves rebal's config location with the same precedence rebal
// itself uses: an explicit --config wins, then config.toml in the current
// directory, then the XDG default.
func configPath(cmd *cobra.Command) (string, error) {
	if p, _ := cmd.Flags().GetString("config"); p != "" {
		return p, nil
	}
	if _, err := os.Stat("config.toml"); err == nil {
		return "config.toml", nil
	}
	return config.DefaultPath()
}

func statePath(cmd *cobra.Command) (string, error) {
	if p, _ := cmd.Flags().GetString("state"); p != "" {
		return p, nil
	}
	if _, err := os.Stat("state.json"); err == nil {
		return "state.json", nil
	}
	return state.DefaultPath()
}

// loadConfigIfExists resolves and loads rebal's config. Unlike rebal itself,
// this addon never writes a starter config — a missing file is reported back
// via existed=false so callers can decide what to do (sync requires an
// existing config unless --create-missing is set).
func loadConfigIfExists(cmd *cobra.Command) (cfg config.Config, path string, existed bool, err error) {
	path, err = configPath(cmd)
	if err != nil {
		return config.Config{}, "", false, err
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return config.Config{}, path, false, nil
	}
	cfg, err = config.Load(path)
	return cfg, path, true, err
}

func loadState(cmd *cobra.Command) (state.State, string, error) {
	path, err := statePath(cmd)
	if err != nil {
		return state.State{}, "", err
	}
	s, err := state.Load(path)
	return s, path, err
}
