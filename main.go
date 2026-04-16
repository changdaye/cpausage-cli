package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
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

func parseFlags() (config, error) {
	cfg := config{}
	var baseURLFlag string
	var managementSecret string
	flag.StringVar(&baseURLFlag, "cpa-base-url", "", "CPA base URL or management page URL")
	flag.StringVar(&baseURLFlag, "cpa-url", "", "Alias of --cpa-base-url")
	flag.StringVar(&baseURLFlag, "url", "", "Short alias of --cpa-base-url")
	flag.StringVar(&managementSecret, "management-key", "", "CPA management bearer key")
	flag.StringVar(&managementSecret, "management-password", "", "Alias of --management-key")
	flag.StringVar(&managementSecret, "k", "", "Alias of --management-key")
	flag.StringVar(&managementSecret, "p", "", "Alias of --management-password")
	flag.StringVar(&cfg.ConfigPath, "config", "", "Path to JSON config file")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "Print version/build information")
	flag.BoolVar(&cfg.JSON, "json", false, "Print JSON output")
	flag.BoolVar(&cfg.Plain, "plain", false, "Print plain output")
	flag.BoolVar(&cfg.SummaryOnly, "summary-only", false, "Print summary only")
	flag.StringVar(&cfg.Style, "style", "1", "Pretty output style: 1 (classic) or 2 (cards)")
	flag.BoolVar(&cfg.ASCIIBars, "ascii-bars", false, "Use ASCII progress bars instead of Unicode bars")
	flag.BoolVar(&cfg.NoProgress, "no-progress", false, "Disable quota query progress output")
	flag.StringVar(&cfg.FilterPlan, "filter-plan", "", "Only show accounts with this plan_type")
	flag.StringVar(&cfg.FilterStatus, "filter-status", "", "Only show accounts with this derived status")
	flag.IntVar(&cfg.Concurrency, "concurrency", 8, "Concurrent quota refresh workers")
	timeoutSeconds := flag.Int("timeout", defaultTimeoutSeconds, "HTTP timeout in seconds")
	flag.IntVar(&cfg.RetryAttempts, "retry-attempts", defaultRetryAttempts, "Retry attempts for transient per-account quota queries")
	flag.Parse()

	if cfg.ShowVersion {
		return cfg, nil
	}
	normalizedStyle, err := normalizePrettyStyle(cfg.Style)
	if err != nil {
		return config{}, err
	}
	cfg.Style = normalizedStyle

	fileCfg, err := loadUserConfig(cfg.ConfigPath)
	if err != nil {
		return config{}, err
	}
	cfg.StatePath, err = resolveStatePath(cfg.ConfigPath)
	if err != nil {
		return config{}, err
	}

	cfg.BaseURL = resolveBaseURL(baseURLFlag, fileCfg)
	cfg.ManagementKey = resolveManagementKey(managementSecret, fileCfg)
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
	cfg.Now = time.Now
	return cfg, nil
}

func normalizePrettyStyle(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "1", "classic", "default":
		return "1", nil
	case "2", "cards", "card":
		return "2", nil
	default:
		return "", fmt.Errorf("invalid --style %q (supported: 1, 2)", v)
	}
}

func resolveBaseURL(explicit string, fileCfg userConfig) string {
	for _, candidate := range []string{
		explicit,
		strings.TrimSpace(os.Getenv("CPA_BASE_URL")),
		strings.TrimSpace(os.Getenv("CPA_URL")),
		configBaseURL(fileCfg),
		defaultCPABaseURL,
	} {
		if normalized := normalizeBaseURL(candidate); normalized != "" {
			return normalized
		}
	}
	return defaultCPABaseURL
}

func normalizeBaseURL(v string) string {
	raw := strings.TrimSpace(v)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "http://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		raw = strings.TrimSuffix(raw, "/")
		raw = strings.ReplaceAll(raw, "/v0/management", "")
		raw = strings.TrimSuffix(raw, "/management.html")
		raw = strings.TrimSuffix(raw, "/login")
		return strings.TrimSuffix(raw, "/")
	}

	path := strings.TrimRight(parsed.EscapedPath(), "/")
	for _, suffix := range []string{
		"/v0/management/auth-files",
		"/v0/management/api-call",
		"/v0/management",
		"/management.html",
		"/login",
	} {
		path = strings.TrimSuffix(path, suffix)
	}
	if path == "/" {
		path = ""
	}
	parsed.Path = path
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimSuffix(parsed.String(), "/")
}

func resolveManagementKey(explicit string, fileCfg userConfig) string {
	for _, candidate := range []string{
		explicit,
		strings.TrimSpace(os.Getenv("CPA_MANAGEMENT_KEY")),
		strings.TrimSpace(os.Getenv("CPA_MANAGEMENT_PASSWORD")),
		strings.TrimSpace(os.Getenv("MANAGEMENT_PASSWORD")),
		configManagementKey(fileCfg),
	} {
		if v := strings.TrimSpace(candidate); v != "" {
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
	if usageResult, err := fetchTokenUsageByAuth(ctx, cfg, time.Now()); err == nil {
		for i := range reports {
			reports[i].tokenUsage = tokenUsageSummary{
				Available:       true,
				HistoryStart:    formatTokenUsageHistoryTimestamp(usageResult.HistoryStart, time.Local),
				HistoryEnd:      formatTokenUsageHistoryTimestamp(usageResult.HistoryEnd, time.Local),
				Complete7Hours:  usageResult.Complete7Hours,
				Complete24Hours: usageResult.Complete24Hours,
				Complete7Days:   usageResult.Complete7Days,
			}
			if usage, ok := usageResult.ByAuth[reports[i].AuthIndex]; ok {
				usage.HistoryStart = reports[i].tokenUsage.HistoryStart
				usage.HistoryEnd = reports[i].tokenUsage.HistoryEnd
				usage.Complete7Hours = reports[i].tokenUsage.Complete7Hours
				usage.Complete24Hours = reports[i].tokenUsage.Complete24Hours
				usage.Complete7Days = reports[i].tokenUsage.Complete7Days
				usage.Available = true
				reports[i].tokenUsage = usage
			}
		}
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
