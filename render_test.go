package main

import "testing"

func TestQuotaBarFilledCells(t *testing.T) {
	cases := []struct {
		value float64
		want  int
	}{
		{value: 0, want: 0},
		{value: 5, want: 1},
		{value: 25, want: 5},
		{value: 50, want: 10},
		{value: 75, want: 15},
		{value: 100, want: 20},
	}

	for _, tc := range cases {
		if got := quotaBarFilledCells(tc.value); got != tc.want {
			t.Fatalf("quotaBarFilledCells(%v) = %d, want %d", tc.value, got, tc.want)
		}
	}
}

func TestQuotaBarDisplayWidth(t *testing.T) {
	if got := quotaBarDisplayWidth(); got != 27 {
		t.Fatalf("quotaBarDisplayWidth() = %d, want 27", got)
	}
}
