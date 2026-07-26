package config

import (
	"fmt"
	"math"
	"strings"
)

// Source is an explicit percentage-point contribution for one position,
// identified by ISIN. It is how a caller says "take 5pp from A and 5pp from B".
type Source struct {
	ISIN string
	PP   float64
}

// Redistribution says where target room comes from when a position is added or
// raised, or where it goes when one is removed or lowered. Exactly one field is
// meaningful: Scale spreads the change across every other active position in
// proportion to its current weight; Sources names explicit per-position amounts.
type Redistribution struct {
	Scale   bool
	Sources []Source
}

// The mutation functions below are pure: each takes a Config and returns a new
// one, touching no I/O. They enforce the arithmetic invariants that hold
// regardless of market values (targets sum to 100, sources sum correctly, no
// target driven to zero). Rules that depend on state — chiefly the
// never-converges check — are left to rebal.Validate at the write boundary.

// AddPosition returns cfg with a new position appended, making room for its
// target via r. Existing active targets are adjusted and re-rounded so the active
// set still sums to exactly 100.
func AddPosition(cfg Config, name, isin string, target float64, min *int, r Redistribution) (Config, error) {
	name = strings.TrimSpace(name)
	isin = normISIN(isin)
	if name == "" || isin == "" {
		return Config{}, fmt.Errorf("name and ISIN are required")
	}
	if target <= 0 {
		return Config{}, fmt.Errorf("target must be > 0")
	}
	if cfg.indexByISIN(isin) >= 0 {
		return Config{}, fmt.Errorf("a position with ISIN %s already exists", isin)
	}

	out := cfg
	out.Positions = clonePositions(cfg.Positions)
	active := out.activeIndexes()

	switch {
	case len(active) == 0:
		// First active position: nothing to take room from. It stands at its
		// target; rebal.Validate flags it later if that is not 100.
	case r.Scale:
		out.scaleActive(active, 100-target)
	case len(r.Sources) > 0:
		if err := out.applySources(r.Sources, -1, target, ""); err != nil {
			return Config{}, err
		}
	default:
		return Config{}, fmt.Errorf("adding to an existing portfolio needs --scale or --from")
	}

	newPos := Position{Name: name, ISIN: isin, Target: target}
	if min != nil {
		m := *min
		newPos.MinMonthly = &m
	}
	out.Positions = append(out.Positions, newPos)
	out.roundActive()
	return out, nil
}

// RemovePosition returns cfg with the ISIN dropped, giving its target back to the
// remaining active positions via r. Removing a wind-down position changes no
// targets — it has none.
func RemovePosition(cfg Config, isin string, r Redistribution) (Config, error) {
	isin = normISIN(isin)
	idx := cfg.indexByISIN(isin)
	if idx < 0 {
		return Config{}, fmt.Errorf("no position matches ISIN %s", isin)
	}
	removed := cfg.Positions[idx]

	out := cfg
	out.Positions = clonePositions(cfg.Positions)

	if removed.WindDown {
		out.Positions = removeAt(out.Positions, idx)
		return out, nil
	}

	// Active positions other than the one being removed.
	others := out.activeIndexesExcept(idx)
	if len(others) == 0 {
		return Config{}, fmt.Errorf("cannot remove the last active position")
	}

	switch {
	case r.Scale:
		out.scaleActive(others, 100)
	case len(r.Sources) > 0:
		if err := out.applySources(r.Sources, +1, removed.Target, isin); err != nil {
			return Config{}, err
		}
	default:
		return Config{}, fmt.Errorf("removing a position needs --scale or --to")
	}

	out.Positions = removeAt(out.Positions, idx)
	out.roundActive()
	return out, nil
}

// Retarget returns cfg with one position's target set to newTarget, absorbing the
// delta into the other active positions via r.
func Retarget(cfg Config, isin string, newTarget float64, r Redistribution) (Config, error) {
	isin = normISIN(isin)
	idx := cfg.indexByISIN(isin)
	if idx < 0 {
		return Config{}, fmt.Errorf("no position matches ISIN %s", isin)
	}
	if cfg.Positions[idx].WindDown {
		return Config{}, fmt.Errorf("%s is wind-down and has no target", isin)
	}
	if newTarget <= 0 {
		return Config{}, fmt.Errorf("target must be > 0")
	}

	out := cfg
	out.Positions = clonePositions(cfg.Positions)
	delta := newTarget - out.Positions[idx].Target
	if math.Abs(delta) < 1e-9 {
		return out, nil // no change
	}

	others := out.activeIndexesExcept(idx)
	if len(others) == 0 {
		return Config{}, fmt.Errorf("no other active position to absorb the change")
	}

	switch {
	case r.Scale:
		// The others must now fill whatever the retargeted position leaves.
		out.scaleActive(others, 100-newTarget)
	case len(r.Sources) > 0:
		// delta > 0: the position grows, so sources shrink; delta < 0: the mirror.
		sign := -1.0
		if delta < 0 {
			sign = +1.0
		}
		if err := out.applySources(r.Sources, sign, math.Abs(delta), isin); err != nil {
			return Config{}, err
		}
	default:
		return Config{}, fmt.Errorf("retarget needs --scale or --from")
	}

	out.Positions[idx].Target = newTarget
	out.roundActive()
	return out, nil
}

// Freeze marks a position wind-down: the holding is kept but never bought again.
// Its target is redistributed to the remaining active positions via r, and its
// own target and floor are zeroed.
func Freeze(cfg Config, isin string, r Redistribution) (Config, error) {
	isin = normISIN(isin)
	idx := cfg.indexByISIN(isin)
	if idx < 0 {
		return Config{}, fmt.Errorf("no position matches ISIN %s", isin)
	}
	if cfg.Positions[idx].WindDown {
		return Config{}, fmt.Errorf("%s is already wind-down", isin)
	}

	out := cfg
	out.Positions = clonePositions(cfg.Positions)
	freed := out.Positions[idx].Target

	others := out.activeIndexesExcept(idx)
	if len(others) == 0 {
		return Config{}, fmt.Errorf("cannot freeze the only active position")
	}
	switch {
	case r.Scale:
		out.scaleActive(others, 100)
	case len(r.Sources) > 0:
		if err := out.applySources(r.Sources, +1, freed, isin); err != nil {
			return Config{}, err
		}
	default:
		return Config{}, fmt.Errorf("freezing a position needs --scale or --to")
	}

	out.Positions[idx].WindDown = true
	out.Positions[idx].Target = 0
	out.Positions[idx].MinMonthly = nil
	out.roundActive()
	return out, nil
}

// Unfreeze reactivates a wind-down position with the given target, taking room
// from the other active positions via r.
func Unfreeze(cfg Config, isin string, target float64, r Redistribution) (Config, error) {
	isin = normISIN(isin)
	idx := cfg.indexByISIN(isin)
	if idx < 0 {
		return Config{}, fmt.Errorf("no position matches ISIN %s", isin)
	}
	if !cfg.Positions[idx].WindDown {
		return Config{}, fmt.Errorf("%s is not wind-down", isin)
	}
	if target <= 0 {
		return Config{}, fmt.Errorf("target must be > 0")
	}

	out := cfg
	out.Positions = clonePositions(cfg.Positions)
	active := out.activeIndexes() // excludes this position, which is still frozen

	switch {
	case len(active) == 0:
		// No other active positions: the reactivated one must be the whole 100;
		// rebal.Validate flags it later if target != 100.
	case r.Scale:
		out.scaleActive(active, 100-target)
	case len(r.Sources) > 0:
		if err := out.applySources(r.Sources, -1, target, isin); err != nil {
			return Config{}, err
		}
	default:
		return Config{}, fmt.Errorf("unfreezing a position needs --scale or --from")
	}

	out.Positions[idx].WindDown = false
	out.Positions[idx].Target = target
	out.roundActive()
	return out, nil
}

// EditPosition changes a position's display name and/or min_monthly floor. A nil
// argument leaves that field alone; clearMin removes an override so the position
// falls back to default_min_monthly.
func EditPosition(cfg Config, isin string, name *string, min *int, clearMin bool) (Config, error) {
	isin = normISIN(isin)
	idx := cfg.indexByISIN(isin)
	if idx < 0 {
		return Config{}, fmt.Errorf("no position matches ISIN %s", isin)
	}
	out := cfg
	out.Positions = clonePositions(cfg.Positions)

	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return Config{}, fmt.Errorf("name cannot be empty")
		}
		out.Positions[idx].Name = n
	}
	switch {
	case clearMin:
		out.Positions[idx].MinMonthly = nil
	case min != nil:
		m := *min
		out.Positions[idx].MinMonthly = &m
	}
	return out, nil
}

// SettingsPatch carries optional overrides for [settings]; a nil field is left
// unchanged.
type SettingsPatch struct {
	Monthly    *int
	Tolerance  *float64
	Horizon    *int
	MaxHorizon *int
	DefaultMin *int
}

// SetSettings returns cfg with the non-nil patch fields applied.
func SetSettings(cfg Config, p SettingsPatch) Config {
	out := cfg
	if p.Monthly != nil {
		out.Settings.MonthlyTotal = *p.Monthly
	}
	if p.Tolerance != nil {
		out.Settings.TolerancePP = *p.Tolerance
	}
	if p.Horizon != nil {
		out.Settings.HorizonMonths = *p.Horizon
	}
	if p.MaxHorizon != nil {
		out.Settings.MaxHorizon = *p.MaxHorizon
	}
	if p.DefaultMin != nil {
		out.Settings.DefaultMinMonthly = *p.DefaultMin
	}
	return out
}

// --- internal helpers -------------------------------------------------------

func normISIN(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

func clonePositions(ps []Position) []Position {
	out := make([]Position, len(ps))
	copy(out, ps)
	return out
}

func removeAt(ps []Position, i int) []Position {
	out := make([]Position, 0, len(ps)-1)
	out = append(out, ps[:i]...)
	return append(out, ps[i+1:]...)
}

func (c Config) indexByISIN(isin string) int {
	for i, p := range c.Positions {
		if strings.EqualFold(p.ISIN, isin) {
			return i
		}
	}
	return -1
}

func (c Config) activeIndexes() []int {
	var idx []int
	for i, p := range c.Positions {
		if !p.WindDown {
			idx = append(idx, i)
		}
	}
	return idx
}

func (c Config) activeIndexesExcept(skip int) []int {
	var idx []int
	for i, p := range c.Positions {
		if i != skip && !p.WindDown {
			idx = append(idx, i)
		}
	}
	return idx
}

// scaleActive rescales the targets of the given position indexes so they sum to
// `to`, in proportion to their current values.
func (c *Config) scaleActive(idx []int, to float64) {
	cur := make([]float64, len(idx))
	for i, pi := range idx {
		cur[i] = c.Positions[pi].Target
	}
	scaled := Scale(cur, to)
	for i, pi := range idx {
		c.Positions[pi].Target = scaled[i]
	}
}

// roundActive largest-remainder rounds every active target to one decimal so the
// active set sums to exactly 100.0.
func (c *Config) roundActive() {
	idx := c.activeIndexes()
	cur := make([]float64, len(idx))
	for i, pi := range idx {
		cur[i] = c.Positions[pi].Target
	}
	rounded := RoundTargets(cur)
	for i, pi := range idx {
		c.Positions[pi].Target = rounded[i]
	}
}

// applySources adjusts each named source's target by sign*PP. The amounts must
// sum to want; a source may not be wind-down, must exist, may not be the excluded
// ISIN (the position being removed or retargeted), and may not be driven to <= 0.
func (c *Config) applySources(sources []Source, sign, want float64, exclude string) error {
	var sum float64
	seen := map[string]bool{}
	for _, s := range sources {
		key := normISIN(s.ISIN)
		if seen[key] {
			return fmt.Errorf("ISIN %s named twice", key)
		}
		seen[key] = true
		sum += s.PP
	}
	if math.Abs(sum-want) > 1e-6 {
		return fmt.Errorf("amounts sum to %s, but must sum to %s", pct(sum), pct(want))
	}
	for _, s := range sources {
		key := normISIN(s.ISIN)
		if exclude != "" && strings.EqualFold(key, exclude) {
			return fmt.Errorf("%s cannot give or take from itself", key)
		}
		idx := c.indexByISIN(key)
		if idx < 0 {
			return fmt.Errorf("no position matches ISIN %s", key)
		}
		if c.Positions[idx].WindDown {
			return fmt.Errorf("%s is wind-down and has no target", key)
		}
		c.Positions[idx].Target += sign * s.PP
		if c.Positions[idx].Target <= 0 {
			return fmt.Errorf("this would drive %s to %s (must stay > 0)", key, pct(c.Positions[idx].Target))
		}
	}
	return nil
}

// pct renders a percentage to one decimal without a trailing ".0".
func pct(f float64) string {
	return strings.TrimSuffix(fmt.Sprintf("%.1f", f), ".0")
}
