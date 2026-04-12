package main

import "time"

const (
	defaultCPABaseURL     = "http://127.0.0.1:8317"
	defaultTimeoutSeconds = 30
	defaultRetryAttempts  = 3
	window5HSeconds       = 5 * 60 * 60
	window7DSeconds       = 7 * 24 * 60 * 60
	whamUsageURL          = "https://chatgpt.com/backend-api/wham/usage"
)

var whamHeaders = map[string]string{
	"Authorization": "Bearer $TOKEN$",
	"Content-Type":  "application/json",
	"User-Agent":    "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal",
}

type config struct {
	BaseURL       string
	ManagementKey string
	ConfigPath    string
	ShowVersion   bool
	JSON          bool
	Plain         bool
	SummaryOnly   bool
	ASCIIBars     bool
	NoProgress    bool
	FilterPlan    string
	FilterStatus  string
	Concurrency   int
	Timeout       time.Duration
	RetryAttempts int
}

type quotaWindow struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	UsedPercent      *float64 `json:"used_percent"`
	RemainingPercent *float64 `json:"remaining_percent"`
	ResetLabel       string   `json:"reset_label"`
	Exhausted        bool     `json:"exhausted"`
}

type quotaReport struct {
	Name              string        `json:"name"`
	AuthIndex         string        `json:"auth_index,omitempty"`
	AccountID         string        `json:"account_id,omitempty"`
	PlanType          string        `json:"plan_type,omitempty"`
	Status            string        `json:"status"`
	Windows           []quotaWindow `json:"windows"`
	AdditionalWindows []quotaWindow `json:"additional_windows"`
	Error             string        `json:"error,omitempty"`
}

type summary struct {
	Accounts          int            `json:"accounts"`
	StatusCounts      map[string]int `json:"status_counts"`
	PlanCounts        map[string]int `json:"plan_counts"`
	ExhaustedAccounts int            `json:"exhausted_accounts"`
	LowAccounts       int            `json:"low_accounts"`
	ErrorAccounts     int            `json:"error_accounts"`
	AdditionalWindows int            `json:"additional_windows"`
	ExhaustedNames    []string       `json:"exhausted_names"`
	LowNames          []string       `json:"low_names"`
	ErrorNames        []string       `json:"error_names"`
	FreeEquivalent7D  float64        `json:"free_equivalent_7d"`
	PlusEquivalent7D  float64        `json:"plus_equivalent_7d"`
}

type authEntry struct {
	raw map[string]any
}
