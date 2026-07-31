package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseDenominatedCopper converts a denominated coin string to copper pieces.
// The value may carry a pp/gp/ep/sp/cp suffix (case-insensitive) and may
// include comma separators (e.g. "1,000gp"). A bare integer is treated as
// copper. Returns an error for unknown suffixes and non-integer amounts.
func ParseDenominatedCopper(value string) (int, error) {
	s := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, ",", "")))
	if s == "" {
		return 0, fmt.Errorf("empty coin value")
	}

	type denom struct {
		suffix string
		mult   int
	}
	denoms := []denom{
		{"pp", 1000},
		{"gp", 100},
		{"ep", 50},
		{"sp", 10},
		{"cp", 1},
	}

	for _, d := range denoms {
		if strings.HasSuffix(s, d.suffix) {
			numStr := strings.TrimSpace(s[:len(s)-len(d.suffix)])
			n, err := strconv.Atoi(numStr)
			if err != nil {
				return 0, fmt.Errorf("invalid coin value %q", value)
			}
			return n * d.mult, nil
		}
	}

	// No known denomination suffix: treat the value as a bare copper integer.
	// If the string ends with any other alphabetic characters, that is an unknown
	// suffix and should be rejected rather than silently misparsed.
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return 0, fmt.Errorf("unknown coin denomination in %q", value)
		}
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid coin value %q", value)
	}
	return n, nil
}
