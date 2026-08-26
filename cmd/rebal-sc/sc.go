package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// envelope is the outer shape of every `sc ... --json` response.
type envelope struct {
	OK   bool            `json:"ok"`
	Data json.RawMessage `json:"data"`
}

// scRun invokes `sc <args...> --json` and returns the "data" payload. It
// reports a clear error if `sc` is not installed, not logged in, or the
// command itself fails.
func scRun(args ...string) (json.RawMessage, error) {
	full := append(append([]string{}, args...), "--json")
	cmd := exec.Command("sc", full...)
	out, err := cmd.Output()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return nil, fmt.Errorf("sc not found in PATH — install it from https://github.com/ScalableCapital/scalable-cli")
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if msg := extractErrorMessage(out); msg != "" {
				return nil, fmt.Errorf("sc %s: %s", strings.Join(args, " "), msg)
			}
			if len(exitErr.Stderr) > 0 {
				return nil, fmt.Errorf("sc %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
			}
		}
		return nil, fmt.Errorf("running sc %s: %w", strings.Join(args, " "), err)
	}

	var env envelope
	if err := json.Unmarshal(out, &env); err != nil {
		return nil, fmt.Errorf("parsing sc %s output: %w", strings.Join(args, " "), err)
	}
	if !env.OK {
		if msg := extractErrorMessage(out); msg != "" {
			return nil, fmt.Errorf("sc %s reported an error: %s", strings.Join(args, " "), msg)
		}
		return nil, fmt.Errorf("sc %s reported an error", strings.Join(args, " "))
	}
	return env.Data, nil
}

// extractErrorMessage best-effort-pulls a human message out of an sc error
// envelope. The exact error shape isn't documented, so this degrades
// gracefully instead of assuming a field name.
func extractErrorMessage(raw []byte) string {
	var probe struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	if probe.Message != "" {
		return probe.Message
	}
	if len(probe.Error) > 0 {
		var s string
		if json.Unmarshal(probe.Error, &s) == nil && s != "" {
			return s
		}
		var m map[string]any
		if json.Unmarshal(probe.Error, &m) == nil {
			if v, ok := m["message"].(string); ok && v != "" {
				return v
			}
		}
		return string(probe.Error)
	}
	return ""
}

// holding is one `sc broker holdings` item.
type holding struct {
	ISIN              string  `json:"isin"`
	Name              string  `json:"name"`
	Valuation         float64 `json:"valuation"`
	ValuationCurrency string  `json:"valuation_currency"`
}

func fetchHoldings() ([]holding, error) {
	data, err := scRun("broker", "holdings")
	if err != nil {
		return nil, err
	}
	var r struct {
		Result struct {
			Items []holding `json:"items"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parsing holdings: %w", err)
	}
	return r.Result.Items, nil
}

// savingsPlan is one `sc broker savings-plans` item.
type savingsPlan struct {
	ISIN              string  `json:"isin"`
	Name              string  `json:"name"`
	Amount            float64 `json:"amount"`
	Frequency         string  `json:"frequency"`
	DayOfMonth        int     `json:"day_of_month"`
	NextExecutionDate string  `json:"next_execution_date"`
}

func fetchSavingsPlans() ([]savingsPlan, float64, error) {
	data, err := scRun("broker", "savings-plans")
	if err != nil {
		return nil, 0, err
	}
	var r struct {
		Result struct {
			Items                  []savingsPlan `json:"items"`
			TotalSavingsPlanAmount float64       `json:"total_savings_plan_amount"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, 0, fmt.Errorf("parsing savings-plans: %w", err)
	}
	return r.Result.Items, r.Result.TotalSavingsPlanAmount, nil
}

// watchlistItem is one `sc broker watchlist` item.
type watchlistItem struct {
	ISIN          string  `json:"isin"`
	Name          string  `json:"name"`
	QuoteMidPrice float64 `json:"quote_mid_price"`
	QuoteCurrency string  `json:"quote_currency"`
}

func fetchWatchlist() ([]watchlistItem, error) {
	data, err := scRun("broker", "watchlist")
	if err != nil {
		return nil, err
	}
	var r struct {
		Result struct {
			Items []watchlistItem `json:"items"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parsing watchlist: %w", err)
	}
	return r.Result.Items, nil
}
