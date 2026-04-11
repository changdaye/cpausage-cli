package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

func main() {
	cfg := parseFlags()
	if cfg.ShowVersion {
		fmt.Println(versionString())
		return
	}

	ctx := context.Background()
	reports, _, err := fetchAll(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	filtered := filterReports(reports, cfg.FilterPlan, cfg.FilterStatus)
	sortReportsDefault(filtered)
	sum := summarize(filtered)

	if cfg.JSON {
		payload := map[string]any{
			"base_url": cfg.BaseURL,
			"summary":  sum,
			"reports":  filtered,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(payload)
		return
	}

	if cfg.Plain || cfg.SummaryOnly {
		renderPlain(filtered, sum, cfg.SummaryOnly)
		return
	}

	renderPrettyReport(filtered, sum, cfg)
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.BaseURL, "cpa-base-url", defaultCPABaseURL, "CPA base URL")
	flag.StringVar(&cfg.ManagementKey, "management-key", "", "CPA management key")
	flag.StringVar(&cfg.ManagementKey, "k", "", "Alias of --management-key")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "Print version/build information")
	flag.BoolVar(&cfg.JSON, "json", false, "Print JSON output")
	flag.BoolVar(&cfg.Plain, "plain", false, "Print plain output")
	flag.BoolVar(&cfg.SummaryOnly, "summary-only", false, "Print summary only")
	flag.BoolVar(&cfg.ASCIIBars, "ascii-bars", false, "Use ASCII progress bars instead of Unicode bars")
	flag.BoolVar(&cfg.NoProgress, "no-progress", false, "Disable quota query progress output")
	flag.StringVar(&cfg.FilterPlan, "filter-plan", "", "Only show accounts with this plan_type")
	flag.StringVar(&cfg.FilterStatus, "filter-status", "", "Only show accounts with this derived status")
	flag.IntVar(&cfg.Concurrency, "concurrency", 8, "Concurrent quota refresh workers")
	timeoutSeconds := flag.Int("timeout", defaultTimeoutSeconds, "HTTP timeout in seconds")
	flag.IntVar(&cfg.RetryAttempts, "retry-attempts", defaultRetryAttempts, "Retry attempts for transient per-account quota queries")
	flag.Parse()

	cfg.BaseURL = normalizeBaseURL(cfg.BaseURL)
	cfg.ManagementKey = resolveManagementKey(cfg.ManagementKey)
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}
	if cfg.RetryAttempts < 1 {
		cfg.RetryAttempts = 1
	}
	if *timeoutSeconds < 1 {
		*timeoutSeconds = defaultTimeoutSeconds
	}
	cfg.Timeout = time.Duration(*timeoutSeconds) * time.Second
	return cfg
}

func normalizeBaseURL(v string) string {
	raw := strings.TrimSpace(v)
	if raw == "" {
		return defaultCPABaseURL
	}
	raw = strings.TrimSuffix(raw, "/")
	raw = strings.ReplaceAll(raw, "/v0/management", "")
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "http://" + raw
	}
	return raw
}

func resolveManagementKey(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	for _, key := range []string{"MANAGEMENT_PASSWORD", "CPA_MANAGEMENT_KEY"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

func fetchAll(ctx context.Context, cfg config) ([]quotaReport, summary, error) {
	auths, err := loadCodexAuths(ctx, cfg)
	if err != nil {
		return nil, summary{}, err
	}
	showProgress := !cfg.JSON && !cfg.NoProgress && isStdoutTerminal()
	reports, err := queryAllQuotas(ctx, cfg, auths, showProgress)
	if err != nil {
		return nil, summary{}, err
	}
	return reports, summarize(reports), nil
}

func sortReportsDefault(reports []quotaReport) {
	planRank := func(plan string) int {
		switch strings.ToLower(strings.TrimSpace(plan)) {
		case "free":
			return 0
		case "team":
			return 1
		case "plus":
			return 2
		default:
			return 3
		}
	}
	remaining7d := func(report quotaReport) float64 {
		window := findWindow(report.Windows, "code-7d")
		if window == nil || window.RemainingPercent == nil {
			return 101
		}
		return *window.RemainingPercent
	}
	sort.SliceStable(reports, func(i, j int) bool {
		li := planRank(reports[i].PlanType)
		lj := planRank(reports[j].PlanType)
		if li != lj {
			return li < lj
		}
		ri := remaining7d(reports[i])
		rj := remaining7d(reports[j])
		if ri != rj {
			return ri < rj
		}
		return strings.ToLower(reports[i].Name) < strings.ToLower(reports[j].Name)
	})
}
