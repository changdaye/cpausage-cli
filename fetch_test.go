package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoadCodexAuthsKeepsDisabledAccounts(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/management/auth-files" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"files": []any{
				map[string]any{"name": "enabled-codex", "provider": "codex"},
				map[string]any{"name": "disabled-codex", "provider": "codex", "disabled": true},
				map[string]any{"name": "status-disabled-codex", "provider": "codex", "status": "disabled"},
				map[string]any{"name": "other-provider", "provider": "claude"},
			},
		})
	}))
	defer server.Close()

	auths, err := loadCodexAuths(context.Background(), config{
		BaseURL: server.URL,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("loadCodexAuths() error = %v", err)
	}

	if len(auths) != 3 {
		t.Fatalf("len(auths) = %d, want 3", len(auths))
	}
	gotNames := []string{
		cleanString(auths[0].raw["name"]),
		cleanString(auths[1].raw["name"]),
		cleanString(auths[2].raw["name"]),
	}
	wantNames := []string{"enabled-codex", "disabled-codex", "status-disabled-codex"}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("auth[%d] = %q, want %q", i, gotNames[i], wantNames[i])
		}
	}
}

func TestQuerySingleQuotaReturnsDisabledWithoutQuotaRequest(t *testing.T) {
	t.Helper()

	var apiCallCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v0/management/api-call" {
			apiCallCount++
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	report, err := querySingleQuota(context.Background(), server.Client(), config{
		BaseURL:       server.URL,
		Timeout:       time.Second,
		RetryAttempts: 3,
	}, authEntry{
		raw: map[string]any{
			"name":       "disabled-auth",
			"auth_index": "auth-1",
			"provider":   "codex",
			"disabled":   true,
		},
	})
	if err != nil {
		t.Fatalf("querySingleQuota() error = %v", err)
	}

	if apiCallCount != 0 {
		t.Fatalf("apiCallCount = %d, want 0", apiCallCount)
	}
	if !report.Disabled {
		t.Fatalf("report.Disabled = false, want true")
	}
	if report.Status != "disabled" {
		t.Fatalf("report.Status = %q, want %q", report.Status, "disabled")
	}
	if report.Error != "" {
		t.Fatalf("report.Error = %q, want empty", report.Error)
	}
}

func TestQuerySingleQuotaDisablesExpiredTokenAccount(t *testing.T) {
	t.Helper()

	var disabledRequestBody string
	var disableCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v0/management/api-call":
			if r.Method != http.MethodPost {
				t.Fatalf("api-call method = %s, want POST", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_code": http.StatusUnauthorized,
				"body": map[string]any{
					"error": map[string]any{
						"message": "Provided authentication token is expired. Please try signing in again.",
						"code":    "token_expired",
					},
					"status": http.StatusUnauthorized,
				},
			})
		case "/v0/management/auth-files/status":
			if r.Method != http.MethodPatch {
				t.Fatalf("auth-files/status method = %s, want PATCH", r.Method)
			}
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read patch body: %v", err)
			}
			disabledRequestBody = string(raw)
			disableCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":   "ok",
				"disabled": true,
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	report, err := querySingleQuota(context.Background(), server.Client(), config{
		BaseURL:       server.URL,
		Timeout:       time.Second,
		RetryAttempts: 3,
	}, authEntry{
		raw: map[string]any{
			"name":       "expired-auth",
			"auth_index": "auth-1",
			"provider":   "codex",
			"id_token": map[string]any{
				"chatgpt_account_id": "acct-1",
			},
		},
	})
	if err != nil {
		t.Fatalf("querySingleQuota() error = %v", err)
	}

	if disableCalls != 1 {
		t.Fatalf("disableCalls = %d, want 1", disableCalls)
	}
	if !strings.Contains(disabledRequestBody, `"name":"expired-auth"`) {
		t.Fatalf("disable request body = %s", disabledRequestBody)
	}
	if !strings.Contains(disabledRequestBody, `"disabled":true`) {
		t.Fatalf("disable request body = %s", disabledRequestBody)
	}
	if !strings.Contains(report.Error, "Provided authentication token is expired") {
		t.Fatalf("report.Error = %q", report.Error)
	}
	if !strings.Contains(report.Error, "account disabled") {
		t.Fatalf("report.Error = %q", report.Error)
	}
	if !report.Disabled {
		t.Fatalf("report.Disabled = false, want true")
	}
	if report.Status != "disabled" {
		t.Fatalf("report.Status = %q, want %q", report.Status, "disabled")
	}
}
