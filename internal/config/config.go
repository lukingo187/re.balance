// Package config loads, mutates, and writes config.toml: portfolio settings and
// the list of positions. The file is owned by re.balance and managed through the
// CLI (add, rm, retarget, edit, settings) rather than by hand — current market
// values live separately in state.json.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/pelletier/go-toml/v2"
)

// Settings is the [settings] table.
type Settings struct {
	MonthlyTotal      int     `toml:"monthly_total"`
	DefaultMinMonthly int     `toml:"default_min_monthly"`
	HorizonMonths     int     `toml:"horizon_months"`
	TolerancePP       float64 `toml:"tolerance_pp"`
	MaxHorizon        int     `toml:"max_horizon"`
}

// Position is one [[positions]] entry. MinMonthly is a pointer so an omitted
// value ("use the default") is distinguishable from an explicit 0.
type Position struct {
	Name       string  `toml:"name"`
	ISIN       string  `toml:"isin"`
	Target     float64 `toml:"target"`
	MinMonthly *int    `toml:"min_monthly,omitempty"`
	WindDown   bool    `toml:"wind_down,omitempty"`
}

// Config is a parsed config.toml.
type Config struct {
	Settings  Settings   `toml:"settings"`
	Positions []Position `toml:"positions"`
}

// Load reads and parses a config.toml.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := toml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return c, nil
}

// managedHeader is prepended to every written config so a human who opens the
// file knows it is CLI-managed. go-toml does not round-trip comments, so this is
// the one comment that survives — the tool rewrites it on every save.
const managedHeader = "# Managed by re.balance — edit through the CLI (add, rm, retarget, edit,\n" +
	"# settings), not by hand. Current market values live in state.json.\n\n"

// Write serialises a config to path atomically: it marshals to a temp file in the
// same directory and renames it into place, so an interrupted write can never
// leave a half-written config. Parent directories are created as needed.
func Write(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	data := append([]byte(managedHeader), body...)

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// ToInput builds a solver Input by merging the config with current values from
// state (keyed by ISIN, in cents). Missing ISINs default to 0. Per-position
// MinMonthly overrides the default; wind-down positions are forced to 0.
func (c Config) ToInput(values map[string]int64) rebal.Input {
	positions := make([]rebal.Position, len(c.Positions))
	vals := make([]int64, len(c.Positions))
	for i, p := range c.Positions {
		min := c.Settings.DefaultMinMonthly
		if p.MinMonthly != nil {
			min = *p.MinMonthly
		}
		if p.WindDown {
			min = 0
		}
		positions[i] = rebal.Position{
			Name:       p.Name,
			ISIN:       p.ISIN,
			TargetPct:  p.Target,
			MinMonthly: min,
			WindDown:   p.WindDown,
		}
		vals[i] = values[p.ISIN] // 0 if absent — makes add/delete free
	}
	return rebal.Input{
		Positions: positions,
		Values:    vals,
		Monthly:   c.Settings.MonthlyTotal,
		Months:    c.Settings.HorizonMonths,
	}
}
