// Package config loads and represents the user-owned config.toml: portfolio
// settings and the list of positions. It is hand-edited and may carry comments,
// so re.balance only ever reads it — current market values live in state.json.
package config

import (
	"fmt"
	"os"

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
	MinMonthly *int    `toml:"min_monthly"`
	WindDown   bool    `toml:"wind_down"`
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
