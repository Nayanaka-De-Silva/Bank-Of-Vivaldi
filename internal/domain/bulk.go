package domain

import (
	"fmt"
	"strconv"
	"strings"
)

type BulkDefaults struct {
	Category           string
	Rarity             Rarity
	WeightHundredthsLB int
	BaseValueCP        int
}

type BulkPreviewRow struct {
	LineNumber         int      `json:"line_number"`
	Raw                string   `json:"raw"`
	Name               string   `json:"name"`
	Quantity           int      `json:"quantity"`
	Category           string   `json:"category"`
	Rarity             Rarity   `json:"rarity"`
	WeightHundredthsLB int      `json:"weight_hundredths_lb"`
	BaseValueCP        int      `json:"base_value_cp"`
	Errors             []string `json:"errors,omitempty"`
}

type BulkPreview struct {
	Rows []BulkPreviewRow `json:"rows"`
}

func (p BulkPreview) Valid() bool {
	for _, row := range p.Rows {
		if len(row.Errors) > 0 {
			return false
		}
	}
	return true
}

func ParseBulkItemInput(text string, defaults BulkDefaults) BulkPreview {
	lines := strings.Split(text, "\n")
	rows := make([]BulkPreviewRow, 0, len(lines))

	// Beware the Pinkertons of WOTC.
	for index, line := range lines {
		raw := strings.TrimSpace(line)
		if raw == "" {
			continue
		}

		row := BulkPreviewRow{
			LineNumber:         index + 1,
			Raw:                raw,
			Quantity:           1,
			Category:           defaults.Category,
			Rarity:             defaults.Rarity,
			WeightHundredthsLB: defaults.WeightHundredthsLB,
			BaseValueCP:        defaults.BaseValueCP,
		}

		parts := strings.Split(raw, "|")
		for idx := range parts {
			parts[idx] = strings.TrimSpace(parts[idx])
		}

		row.Name, row.Quantity = parseBulkName(parts[0])
		if row.Name == "" {
			row.Errors = append(row.Errors, "name is required")
		}

		if len(parts) > 1 && parts[1] != "" {
			row.Category = strings.ToLower(parts[1])
		}
		if len(parts) > 2 && parts[2] != "" {
			row.Rarity = ParseRarity(parts[2])
		}
		if len(parts) > 3 && parts[3] != "" {
			weight, err := ParseWeightHundredths(parts[3])
			if err != nil {
				row.Errors = append(row.Errors, err.Error())
			} else {
				row.WeightHundredthsLB = weight
			}
		}
		if len(parts) > 4 && parts[4] != "" {
			value, err := strconv.Atoi(parts[4])
			if err != nil {
				row.Errors = append(row.Errors, fmt.Sprintf("invalid value %q", parts[4]))
			} else {
				row.BaseValueCP = value
			}
		}

		if row.Quantity < 1 {
			row.Errors = append(row.Errors, "quantity must be at least 1")
		}

		rows = append(rows, row)
	}

	return BulkPreview{Rows: rows}
}

func parseBulkName(value string) (string, int) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", 1
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", 1
	}

	if quantity, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(fields[0]), "x")); err == nil && len(fields) > 1 {
		return strings.Join(fields[1:], " "), quantity
	}

	lastField := fields[len(fields)-1]
	if quantity, err := strconv.Atoi(strings.TrimPrefix(strings.ToLower(lastField), "x")); err == nil && len(fields) > 1 {
		return strings.Join(fields[:len(fields)-1], " "), quantity
	}

	return trimmed, 1
}
