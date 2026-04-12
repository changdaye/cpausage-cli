package main

import (
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "login page url",
			in:   "http://34.146.152.231:8317/management.html#/login",
			want: "http://34.146.152.231:8317",
		},
		{
			name: "management api path",
			in:   "http://127.0.0.1:8317/v0/management/auth-files",
			want: "http://127.0.0.1:8317",
		},
		{
			name: "bare host",
			in:   "34.146.152.231:8317",
			want: "http://34.146.152.231:8317",
		},
		{
			name: "empty",
			in:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeBaseURL(tt.in); got != tt.want {
				t.Fatalf("normalizeBaseURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestConfigFieldFallbacks(t *testing.T) {
	cfg := userConfig{
		LoginURL:           "https://example.com/management.html#/login",
		ManagementPassword: "secret",
	}

	if got := configBaseURL(cfg); got != cfg.LoginURL {
		t.Fatalf("configBaseURL() = %q, want %q", got, cfg.LoginURL)
	}
	if got := configManagementKey(cfg); got != cfg.ManagementPassword {
		t.Fatalf("configManagementKey() = %q, want %q", got, cfg.ManagementPassword)
	}
}

func TestResolveBaseURLPrefersEnvOverConfig(t *testing.T) {
	t.Setenv("CPA_BASE_URL", "http://env.example:8317/management.html#/login")
	t.Setenv("CPA_URL", "")

	got := resolveBaseURL("", userConfig{
		LoginURL: "http://config.example:8317/management.html#/login",
	})

	if want := "http://env.example:8317"; got != want {
		t.Fatalf("resolveBaseURL() = %q, want %q", got, want)
	}
}

func TestResolveManagementKeyPrefersEnvOverConfig(t *testing.T) {
	t.Setenv("CPA_MANAGEMENT_KEY", "env-secret")
	t.Setenv("CPA_MANAGEMENT_PASSWORD", "")
	t.Setenv("MANAGEMENT_PASSWORD", "")

	got := resolveManagementKey("", userConfig{
		ManagementPassword: "config-secret",
	})

	if want := "env-secret"; got != want {
		t.Fatalf("resolveManagementKey() = %q, want %q", got, want)
	}
}

func TestUniqueStrings(t *testing.T) {
	values := []string{"a", "b", "a", "", "b", "c"}
	got := uniqueStrings(values)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("uniqueStrings() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("uniqueStrings()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
