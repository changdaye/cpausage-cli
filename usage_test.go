package main

import (
	"bytes"
	"io"
	"os"
	"strings"
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

	result := parseTokenUsageByAuth(payload, now)
	got := result.ByAuth

	if got["auth-a"].Last7Hours != 0 {
		t.Fatalf("auth-a last7h = %d, want 0", got["auth-a"].Last7Hours)
	}
	if got["auth-a"].Last24Hours != 100 {
		t.Fatalf("auth-a last24h = %d, want 100", got["auth-a"].Last24Hours)
	}
	if got["auth-a"].Last7Days != 600 {
		t.Fatalf("auth-a last7d = %d, want 600", got["auth-a"].Last7Days)
	}
	if got["auth-a"].AllTime != 1500 {
		t.Fatalf("auth-a all = %d, want 1500", got["auth-a"].AllTime)
	}

	if got["auth-b"].Last7Hours != 55 {
		t.Fatalf("auth-b last7h = %d, want 55", got["auth-b"].Last7Hours)
	}
	if got["auth-b"].Last24Hours != 55 {
		t.Fatalf("auth-b last24h = %d, want 55", got["auth-b"].Last24Hours)
	}
	if got["auth-b"].Last7Days != 55 {
		t.Fatalf("auth-b last7d = %d, want 55", got["auth-b"].Last7Days)
	}
	if got["auth-b"].AllTime != 55 {
		t.Fatalf("auth-b all = %d, want 55", got["auth-b"].AllTime)
	}

	if result.HistoryStart.IsZero() || result.HistoryStart.Format(time.RFC3339Nano) != "2026-03-10T03:00:00Z" {
		t.Fatalf("history_start = %s, want %s", result.HistoryStart.Format(time.RFC3339Nano), "2026-03-10T03:00:00Z")
	}
	if result.HistoryEnd.IsZero() || result.HistoryEnd.Format(time.RFC3339Nano) != "2026-04-15T01:30:00Z" {
		t.Fatalf("history_end = %s, want %s", result.HistoryEnd.Format(time.RFC3339Nano), "2026-04-15T01:30:00Z")
	}
	if !result.Complete7Hours || !result.Complete24Hours || !result.Complete7Days {
		t.Fatalf("expected all token usage windows to be complete, got %+v", result)
	}
}

func TestSummarizeAggregatesTokenUsage(t *testing.T) {
	sum := summarize([]quotaReport{
		{
			Name:     "a",
			PlanType: "free",
			Status:   "high",
			tokenUsage: tokenUsageSummary{
				Available:       true,
				AllTime:         100,
				Last7Hours:      10,
				Last24Hours:     20,
				Last7Days:       30,
				HistoryStart:    "2026-04-10T12:58:01+08:00",
				HistoryEnd:      "2026-04-15T18:08:08+08:00",
				Complete7Hours:  true,
				Complete24Hours: true,
				Complete7Days:   false,
			},
		},
		{
			Name:     "b",
			PlanType: "plus",
			Status:   "low",
			tokenUsage: tokenUsageSummary{
				Available:       true,
				AllTime:         10,
				Last7Hours:      1,
				Last24Hours:     2,
				Last7Days:       3,
				HistoryStart:    "2026-04-10T12:58:01+08:00",
				HistoryEnd:      "2026-04-15T18:08:08+08:00",
				Complete7Hours:  true,
				Complete24Hours: true,
				Complete7Days:   false,
			},
		},
	})

	if !sum.TokenUsage.Available {
		t.Fatalf("token usage should be available")
	}
	if sum.TokenUsage.Last7Hours != 11 {
		t.Fatalf("last7h = %d, want 11", sum.TokenUsage.Last7Hours)
	}
	if sum.TokenUsage.Last24Hours != 22 {
		t.Fatalf("last24h = %d, want 22", sum.TokenUsage.Last24Hours)
	}
	if sum.TokenUsage.Last7Days != 33 {
		t.Fatalf("last7d = %d, want 33", sum.TokenUsage.Last7Days)
	}
	if sum.TokenUsage.AllTime != 110 {
		t.Fatalf("all = %d, want 110", sum.TokenUsage.AllTime)
	}
	if sum.TokenUsage.HistoryStart != "2026-04-10T12:58:01+08:00" {
		t.Fatalf("history_start = %q, want %q", sum.TokenUsage.HistoryStart, "2026-04-10T12:58:01+08:00")
	}
	if sum.TokenUsage.HistoryEnd != "2026-04-15T18:08:08+08:00" {
		t.Fatalf("history_end = %q, want %q", sum.TokenUsage.HistoryEnd, "2026-04-15T18:08:08+08:00")
	}
	if !sum.TokenUsage.Complete7Hours || !sum.TokenUsage.Complete24Hours {
		t.Fatalf("7h/24h completeness should be preserved")
	}
	if sum.TokenUsage.Complete7Days {
		t.Fatalf("7d completeness should remain false")
	}
}

func TestFormatInt64WithCommas(t *testing.T) {
	if got := formatInt64WithCommas(71323195); got != "71,323,195" {
		t.Fatalf("formatInt64WithCommas() = %q, want %q", got, "71,323,195")
	}
}

func TestFormatTokenUsageValueReturnsRawCounts(t *testing.T) {
	usage := tokenUsageSummary{
		Available:       true,
		Last7Days:       123,
		AllTime:         456,
		HistoryStart:    "2026-04-10T12:58:01+08:00",
		Complete7Days:   false,
		Complete7Hours:  true,
		Complete24Hours: true,
	}

	if got := formatTokenUsageValue(usage, tokenUsageWindow7Days); got != "123" {
		t.Fatalf("formatTokenUsageValue(7d) = %q", got)
	}
	if got := formatTokenUsageValue(usage, tokenUsageWindow7Hours); got != "0" {
		t.Fatalf("formatTokenUsageValue(7h) = %q", got)
	}
	if got := formatTokenUsageValue(usage, tokenUsageWindowAll); got != "456" {
		t.Fatalf("formatTokenUsageValue(all) = %q", got)
	}
}

func TestRenderPlainIncludesTokenUsageAll(t *testing.T) {
	sum := summary{
		Accounts: 1,
		TokenUsage: tokenUsageSummary{
			Available: true,
			AllTime:   456,
		},
	}

	output := captureStdout(t, func() {
		renderPlain(nil, sum, true)
	})

	if !strings.Contains(output, "Token Usage All: 456") {
		t.Fatalf("renderPlain output missing token usage all line:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}

	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		t.Fatalf("io.Copy() error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("reader.Close() error = %v", err)
	}

	return buf.String()
}
