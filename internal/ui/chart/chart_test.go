package chart

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

// decaySeries is a monotone decay that crosses tolerance mid-range.
func decaySeries() Series {
	return Series{
		X: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		Y: []float64{8, 6, 4, 3, 2, 1.5, 1, 0.8, 0.5, 0.4, 0.3, 0.2},
	}
}

func TestRenderGolden(t *testing.T) {
	cases := []struct {
		name string
		s    Series
		o    Opts
	}{
		{"decay", decaySeries(), Opts{Tolerance: 0.5}},
		{"cursor", decaySeries(), Opts{Tolerance: 0.5, Cursor: 9}},
		{
			"no-converge",
			Series{X: []int{1, 2, 3, 4, 5, 6}, Y: []float64{5, 4.5, 4, 3.8, 3.6, 3.5}},
			Opts{Tolerance: 0.5},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Render(tc.s, tc.o)
			golden := filepath.Join("testdata", tc.name+".golden")
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if got != string(want) {
				t.Errorf("output differs from %s:\n--- got ---\n%s", golden, got)
			}
		})
	}
}

func TestRenderEmpty(t *testing.T) {
	if got := Render(Series{}, Opts{}); got != "" {
		t.Errorf("empty series = %q, want \"\"", got)
	}
}

// TestMarkCalledPerColumn checks the Mark hook wraps every column so the TUI can
// hit-test it. The wrapper here tags each month; we count distinct tags.
func TestMarkCalledPerColumn(t *testing.T) {
	seen := map[int]bool{}
	Render(decaySeries(), Opts{
		Tolerance: 0.5,
		Mark: func(month int, s string) string {
			seen[month] = true
			return s
		},
	})
	if len(seen) != 12 {
		t.Errorf("Mark saw %d distinct months, want 12", len(seen))
	}
}

func TestSummaryConverges(t *testing.T) {
	out := Render(decaySeries(), Opts{Tolerance: 0.5})
	if !strings.Contains(out, "earliest on target: month 9 (±0.5pp) ✓") {
		t.Errorf("missing convergence summary; got:\n%s", out)
	}
}
