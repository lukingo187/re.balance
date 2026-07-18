package config

import (
	"os"
	"path/filepath"
)

// DefaultPath returns $XDG_CONFIG_HOME/rebal/config.toml, falling back to
// os.UserConfigDir()/rebal/config.toml when XDG_CONFIG_HOME is unset.
func DefaultPath() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "rebal", "config.toml"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rebal", "config.toml"), nil
}

// starter is the commented config written on first run. It is the spec's
// 50/30/20 example, ready to edit.
const starter = `# re.balance configuration — this file is yours to edit; re.balance only reads it.
# Current market values are NOT here; set them with 'rebal set <ISIN> <value>'.

[settings]
monthly_total       = 400   # euros/month, total across all savings plans
default_min_monthly = 25    # broker minimum per plan; per-position override below
horizon_months      = 12    # default horizon
tolerance_pp        = 0.5   # convergence = max drift within this many percentage points
max_horizon         = 36    # sweep upper bound

[[positions]]
name   = "MSCI World"
isin   = "IE00B4L5Y983"
target = 50

[[positions]]
name        = "MSCI EM IMI"
isin        = "IE00BKM4GZ66"
target      = 30
min_monthly = 50   # optional override of default_min_monthly

[[positions]]
name   = "Europe Small Cap"
isin   = "IE00B3VWMM18"
target = 20

# A position you want to hold but stop buying: set wind_down = true and drop its
# target. The remaining targets must then sum to 100 among themselves.
# [[positions]]
# name      = "Old Tech Fund"
# isin      = "IE00EXAMPLE00"
# wind_down = true
`

// WriteStarter creates parent directories and writes the starter config.
func WriteStarter(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(starter), 0o644)
}
