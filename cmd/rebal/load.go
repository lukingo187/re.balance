package main

import (
	"fmt"
	"os"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/state"
	"github.com/spf13/cobra"
)

// errStarterWritten is returned when no config existed and a starter was
// written. It carries a friendly pointer rather than reading like a crash.
type errStarterWritten struct{ path string }

func (e errStarterWritten) Error() string {
	return fmt.Sprintf("no config found — wrote a starter to:\n  %s\nedit it, then run this command again", e.path)
}

// configPath resolves the config location: an explicit --config wins, then a
// config.toml in the current directory (handy when kept beside the code), then
// the XDG default.
func configPath(cmd *cobra.Command) (string, error) {
	if p, _ := cmd.Flags().GetString("config"); p != "" {
		return p, nil
	}
	if _, err := os.Stat("config.toml"); err == nil {
		return "config.toml", nil
	}
	return config.DefaultPath()
}

// statePath resolves the state location with the same precedence as configPath.
func statePath(cmd *cobra.Command) (string, error) {
	if p, _ := cmd.Flags().GetString("state"); p != "" {
		return p, nil
	}
	if _, err := os.Stat("state.json"); err == nil {
		return "state.json", nil
	}
	return state.DefaultPath()
}

// loadConfig resolves and loads the config. If the file does not exist it writes
// a starter and returns errStarterWritten.
func loadConfig(cmd *cobra.Command) (config.Config, error) {
	path, err := configPath(cmd)
	if err != nil {
		return config.Config{}, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := config.WriteStarter(path); err != nil {
			return config.Config{}, fmt.Errorf("writing starter config: %w", err)
		}
		return config.Config{}, errStarterWritten{path}
	}
	return config.Load(path)
}

// loadState resolves and loads state; a missing file yields empty state.
func loadState(cmd *cobra.Command) (state.State, string, error) {
	path, err := statePath(cmd)
	if err != nil {
		return state.State{}, "", err
	}
	s, err := state.Load(path)
	return s, path, err
}
