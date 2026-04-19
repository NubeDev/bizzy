package flow

import (
	"testing"
	"time"
)

func TestFieldMatches(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value int
		min   int
		max   int
		want  bool
	}{
		// Wildcard
		{"star matches anything", "*", 5, 0, 59, true},

		// Bare integer
		{"exact match", "5", 5, 0, 59, true},
		{"exact no match", "5", 6, 0, 59, false},

		// Range
		{"range match low", "1-5", 1, 0, 59, true},
		{"range match mid", "1-5", 3, 0, 59, true},
		{"range match high", "1-5", 5, 0, 59, true},
		{"range no match below", "1-5", 0, 0, 59, false},
		{"range no match above", "1-5", 6, 0, 59, false},

		// List
		{"list match first", "1,3,5", 1, 0, 59, true},
		{"list match mid", "1,3,5", 3, 0, 59, true},
		{"list match last", "1,3,5", 5, 0, 59, true},
		{"list no match", "1,3,5", 4, 0, 59, false},

		// Step from star
		{"star step match", "*/5", 0, 0, 59, true},
		{"star step match 15", "*/5", 15, 0, 59, true},
		{"star step no match", "*/5", 3, 0, 59, false},
		{"star step every 2", "*/2", 4, 0, 59, true},
		{"star step every 2 odd", "*/2", 3, 0, 59, false},

		// Step from range
		{"range step match", "1-30/2", 1, 0, 59, true},
		{"range step match 3", "1-30/2", 3, 0, 59, true},
		{"range step no match even", "1-30/2", 2, 0, 59, false},
		{"range step outside range", "1-30/2", 31, 0, 59, false},
		{"range step match 29", "1-30/2", 29, 0, 59, true},

		// Combinations
		{"combo range and int", "1-5,10", 3, 0, 59, true},
		{"combo range and int exact", "1-5,10", 10, 0, 59, true},
		{"combo range and int no match", "1-5,10", 7, 0, 59, false},
		{"combo two ranges", "1-5,15-20", 17, 0, 59, true},
		{"combo two ranges no match", "1-5,15-20", 10, 0, 59, false},

		// DOW: Sunday=0
		{"dow weekdays", "1-5", 1, 0, 6, true},
		{"dow weekdays friday", "1-5", 5, 0, 6, true},
		{"dow weekdays no sunday", "1-5", 0, 0, 6, false},
		{"dow weekdays no saturday", "1-5", 6, 0, 6, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldMatches(tt.field, tt.value, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("fieldMatches(%q, %d, %d, %d) = %v, want %v",
					tt.field, tt.value, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestMatchesCron(t *testing.T) {
	tests := []struct {
		name string
		expr string
		time time.Time
		want bool
	}{
		{
			"every minute",
			"* * * * *",
			time.Date(2026, 4, 18, 14, 30, 0, 0, time.UTC),
			true,
		},
		{
			"exact match",
			"30 14 18 4 *",
			time.Date(2026, 4, 18, 14, 30, 0, 0, time.UTC),
			true,
		},
		{
			"exact no match minute",
			"0 14 18 4 *",
			time.Date(2026, 4, 18, 14, 30, 0, 0, time.UTC),
			false,
		},
		{
			"weekdays at 9am",
			"0 9 * * 1-5",
			// 2026-04-20 is a Monday
			time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC),
			true,
		},
		{
			"weekdays at 9am - sunday rejected",
			"0 9 * * 1-5",
			// 2026-04-19 is a Sunday
			time.Date(2026, 4, 19, 9, 0, 0, 0, time.UTC),
			false,
		},
		{
			"every 5 minutes",
			"*/5 * * * *",
			time.Date(2026, 4, 18, 10, 15, 0, 0, time.UTC),
			true,
		},
		{
			"every 5 minutes no match",
			"*/5 * * * *",
			time.Date(2026, 4, 18, 10, 13, 0, 0, time.UTC),
			false,
		},
		{
			"business hours weekdays",
			"0 9-17 * * 1-5",
			time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC), // Monday noon
			true,
		},
		{
			"business hours weekdays - too early",
			"0 9-17 * * 1-5",
			time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC), // Monday 8am
			false,
		},
		{
			"1st and 15th of month",
			"0 0 1,15 * *",
			time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			true,
		},
		{
			"1st and 15th of month - wrong day",
			"0 0 1,15 * *",
			time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesCron(tt.expr, tt.time)
			if got != tt.want {
				t.Errorf("matchesCron(%q, %v) = %v, want %v",
					tt.expr, tt.time, got, tt.want)
			}
		})
	}
}
