package money

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want int64 // cents
	}{
		{"12450,00", 1245000},
		{"12450.00", 1245000},
		{"12 450 €", 1245000},
		{"12.450,00", 1245000}, // EU grouped
		{"12,450.00", 1245000}, // US grouped
		{"1.234.567,89", 123456789},
		{"1,50", 150},
		{"1.5", 150},
		{"1.234", 123400}, // 3 digits after dot -> grouping, 1234 euros
		{"1000", 100000},
		{"0,00", 0},
		{"7310,50", 731050},
		{"  4 370,00 €  ", 437000},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"", "€", "abc", "1.2345", "12,999.999"} {
		if got, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = %d, want error", in, got)
		}
	}
}

func TestEuro(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{1245000, "12 450,00 €"},
		{0, "0,00 €"},
		{731050, "7 310,50 €"},
		{-50000, "-500,00 €"},
	}
	for _, tt := range tests {
		if got := Euro(tt.in); got != tt.want {
			t.Errorf("Euro(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
