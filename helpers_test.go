package main

import "testing"

func TestFormatStatusCounts(t *testing.T) {
	got := formatStatusCounts(map[string]int{
		"low":       6,
		"high":      17,
		"full":      48,
		"medium":    20,
		"exhausted": 31,
	})

	want := "full:48, high:17, medium:20, low:6, exhausted:31"
	if got != want {
		t.Fatalf("formatStatusCounts() = %q, want %q", got, want)
	}
}

func TestFormatStatusCountsKeepsUnknownStatusesAfterKnownOnes(t *testing.T) {
	got := formatStatusCounts(map[string]int{
		"missing":   2,
		"warning":   1,
		"full":      5,
		"exhausted": 3,
	})

	want := "full:5, exhausted:3, missing:2, warning:1"
	if got != want {
		t.Fatalf("formatStatusCounts() = %q, want %q", got, want)
	}
}
