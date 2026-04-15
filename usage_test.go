package main

import (
	"testing"
	"time"
)

func TestParseTokenUsageByAuth(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 4, 15, 10, 0, 0, 0, loc)

	payload := map[string]any{
		"usage": map[string]any{
			"apis": map[string]any{
				"api-1": map[string]any{
					"models": map[string]any{
						"gpt-5.4": map[string]any{
							"details": []any{
								map[string]any{
									"timestamp":  "2026-04-14T17:00:00Z",
									"auth_index": "auth-a",
									"tokens": map[string]any{
										"total_tokens": 100,
									},
								},
								map[string]any{
									"timestamp":  "2026-04-14T01:00:00Z",
									"auth_index": "auth-a",
									"tokens": map[string]any{
										"total_tokens": 200,
									},
								},
								map[string]any{
									"timestamp":  "2026-04-08T03:00:00Z",
									"auth_index": "auth-a",
									"tokens": map[string]any{
										"total_tokens": 300,
									},
								},
								map[string]any{
									"timestamp":  "2026-03-20T03:00:00Z",
									"auth_index": "auth-a",
									"tokens": map[string]any{
										"total_tokens": 400,
									},
								},
								map[string]any{
									"timestamp":  "2026-03-10T03:00:00Z",
									"auth_index": "auth-a",
									"tokens": map[string]any{
										"total_tokens": 500,
									},
								},
								map[string]any{
									"timestamp":  "2026-04-15T01:30:00Z",
									"auth_index": "auth-b",
									"tokens": map[string]any{
										"input_tokens":     40,
										"output_tokens":    10,
										"reasoning_tokens": 5,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	got := parseTokenUsageByAuth(payload, now)

	if got["auth-a"].Today != 100 {
		t.Fatalf("auth-a today = %d, want 100", got["auth-a"].Today)
	}
	if got["auth-a"].Last24Hours != 100 {
		t.Fatalf("auth-a last24h = %d, want 100", got["auth-a"].Last24Hours)
	}
	if got["auth-a"].Last7Days != 600 {
		t.Fatalf("auth-a last7d = %d, want 600", got["auth-a"].Last7Days)
	}
	if got["auth-a"].Last30Days != 1000 {
		t.Fatalf("auth-a last30d = %d, want 1000", got["auth-a"].Last30Days)
	}

	if got["auth-b"].Today != 55 {
		t.Fatalf("auth-b today = %d, want 55", got["auth-b"].Today)
	}
	if got["auth-b"].Last24Hours != 55 {
		t.Fatalf("auth-b last24h = %d, want 55", got["auth-b"].Last24Hours)
	}
	if got["auth-b"].Last7Days != 55 {
		t.Fatalf("auth-b last7d = %d, want 55", got["auth-b"].Last7Days)
	}
	if got["auth-b"].Last30Days != 55 {
		t.Fatalf("auth-b last30d = %d, want 55", got["auth-b"].Last30Days)
	}
}

func TestSummarizeAggregatesTokenUsage(t *testing.T) {
	sum := summarize([]quotaReport{
		{
			Name:     "a",
			PlanType: "free",
			Status:   "high",
			tokenUsage: tokenUsageSummary{
				Available:   true,
				Today:       10,
				Last24Hours: 20,
				Last7Days:   30,
				Last30Days:  40,
			},
		},
		{
			Name:     "b",
			PlanType: "plus",
			Status:   "low",
			tokenUsage: tokenUsageSummary{
				Available:   true,
				Today:       1,
				Last24Hours: 2,
				Last7Days:   3,
				Last30Days:  4,
			},
		},
	})

	if !sum.TokenUsage.Available {
		t.Fatalf("token usage should be available")
	}
	if sum.TokenUsage.Today != 11 {
		t.Fatalf("today = %d, want 11", sum.TokenUsage.Today)
	}
	if sum.TokenUsage.Last24Hours != 22 {
		t.Fatalf("last24h = %d, want 22", sum.TokenUsage.Last24Hours)
	}
	if sum.TokenUsage.Last7Days != 33 {
		t.Fatalf("last7d = %d, want 33", sum.TokenUsage.Last7Days)
	}
	if sum.TokenUsage.Last30Days != 44 {
		t.Fatalf("last30d = %d, want 44", sum.TokenUsage.Last30Days)
	}
}

func TestFormatInt64WithCommas(t *testing.T) {
	if got := formatInt64WithCommas(71323195); got != "71,323,195" {
		t.Fatalf("formatInt64WithCommas() = %q, want %q", got, "71,323,195")
	}
}
