package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

func loadCodexAuths(ctx context.Context, cfg config) ([]authEntry, error) {
	client := &http.Client{Timeout: cfg.Timeout}
	payload, err := fetchJSON(ctx, client, cfg, cfg.BaseURL+"/v0/management/auth-files")
	if err != nil {
		return nil, err
	}
	files, ok := payload["files"].([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected auth-files payload from CPA management API")
	}
	out := make([]authEntry, 0, len(files))
	for _, item := range files {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		provider := normalizePlan(firstValue(entry["provider"], entry["type"]))
		if provider != "codex" {
			continue
		}
		out = append(out, authEntry{raw: entry})
	}
	return out, nil
}

func queryAllQuotas(ctx context.Context, cfg config, auths []authEntry, showProgress bool) ([]quotaReport, error) {
	if len(auths) == 0 {
		return []quotaReport{}, nil
	}
	client := &http.Client{Timeout: cfg.Timeout}
	reports := make([]quotaReport, len(auths))
	errCh := make(chan error, len(auths))
	progressCh := make(chan string, len(auths))
	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	for i := range auths {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			report, err := querySingleQuota(ctx, client, cfg, auths[i])
			if err != nil {
				errCh <- err
				return
			}
			reports[i] = report
			progressCh <- report.Name
		}()
	}

	done := make(chan struct{})
	if showProgress {
		go func(total int) {
			completed := 0
			current := "-"
			for name := range progressCh {
				completed++
				current = name
				renderFetchProgress(completed, total, current)
			}
			if completed > 0 {
				fmt.Print("\r" + strings.Repeat(" ", 140) + "\r")
			}
			close(done)
		}(len(auths))
	}

	wg.Wait()
	close(progressCh)
	if showProgress {
		<-done
	}

	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	sort.Slice(reports, func(i, j int) bool {
		return strings.ToLower(reports[i].Name) < strings.ToLower(reports[j].Name)
	})
	return reports, nil
}

func renderFetchProgress(done, total int, current string) {
	if total <= 0 {
		return
	}
	termWidth := 120
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 20 {
		termWidth = w
	}
	left := fmt.Sprintf("Querying %d/%d", done, total)
	name := truncate(current, 40)
	barArea := termWidth - displayWidth(left) - displayWidth(name) - 10
	if barArea < 10 {
		barArea = 10
	}
	pct := float64(done) * 100 / float64(total)
	filled := int((pct / 100 * float64(barArea)) + 0.5)
	if filled > barArea {
		filled = barArea
	}
	bar := "[" + strings.Repeat("█", filled) + strings.Repeat("░", max(0, barArea-filled)) + "]"
	fmt.Printf("\r%s %s %3.0f%% %s", left, bar, pct, name)
}

func querySingleQuota(ctx context.Context, client *http.Client, cfg config, entry authEntry) (quotaReport, error) {
	report := quotaReport{
		Name:      cleanString(firstValue(entry.raw["name"], entry.raw["id"], "unknown")),
		AuthIndex: cleanString(firstValue(entry.raw["auth_index"], entry.raw["authIndex"])),
		AccountID: parseAccountID(entry.raw),
		PlanType:  parsePlanType(entry.raw),
		Disabled:  isAuthDisabled(entry.raw),
		Status:    "unknown",
	}
	if report.Name == "" {
		report.Name = "unknown"
	}
	if report.Disabled {
		report.Status = deriveStatus(report)
		return report, nil
	}
	if report.AuthIndex == "" {
		report.Error = "missing auth_index"
		report.Status = deriveStatus(report)
		return report, nil
	}
	if report.AccountID == "" {
		report.Error = "missing chatgpt_account_id"
		report.Status = deriveStatus(report)
		return report, nil
	}

	payload := map[string]any{
		"auth_index": report.AuthIndex,
		"method":     "GET",
		"url":        whamUsageURL,
		"header": mergeMaps(
			whamHeaders,
			map[string]string{"Chatgpt-Account-Id": report.AccountID},
		),
	}

	var lastErr string
	for attempt := 1; attempt <= cfg.RetryAttempts; attempt++ {
		response, err := postJSON(ctx, client, cfg, cfg.BaseURL+"/v0/management/api-call", payload)
		if err != nil {
			lastErr = err.Error()
			if attempt == cfg.RetryAttempts || !shouldRetryError(lastErr) {
				break
			}
			continue
		}
		statusCode := intFromAny(firstValue(response["status_code"], response["statusCode"]))
		bodyValue := response["body"]
		parsedBody, parseErr := parseBody(bodyValue)
		if isTokenExpiredResponse(statusCode, parsedBody, bodyValue) {
			lastErr = tokenExpiredErrorMessage(statusCode, parsedBody, bodyValue)
			if err := disableAuthEntry(ctx, client, cfg, entry); err != nil {
				lastErr = fmt.Sprintf("%s; disable account failed: %v", lastErr, err)
			} else {
				report.Disabled = true
				lastErr += "; account disabled"
			}
			break
		}
		if statusCode < 200 || statusCode >= 300 {
			lastErr = bodyString(bodyValue)
			if lastErr == "" {
				lastErr = fmt.Sprintf("HTTP %d", statusCode)
			}
			if attempt == cfg.RetryAttempts || !shouldRetryError(lastErr) {
				break
			}
			continue
		}
		if parseErr != nil {
			lastErr = "empty or invalid quota payload"
			if attempt == cfg.RetryAttempts {
				break
			}
			continue
		}

		report.PlanType = firstNonEmpty(normalizePlan(firstValue(parsedBody["plan_type"], parsedBody["planType"])), report.PlanType)
		report.Windows = parseCodexWindows(parsedBody)
		report.AdditionalWindows = parseAdditionalWindows(parsedBody)
		report.Error = ""
		report.Status = deriveStatus(report)
		return report, nil
	}

	report.Error = lastErr
	report.Status = deriveStatus(report)
	return report, nil
}

func mergeMaps(base, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

type tokenUsageResult struct {
	ByAuth          map[string]tokenUsageSummary
	HistoryStart    time.Time
	HistoryEnd      time.Time
	Complete7Hours  bool
	Complete24Hours bool
	Complete7Days   bool
}

func fetchTokenUsageByAuth(ctx context.Context, cfg config, now time.Time) (tokenUsageResult, error) {
	client := &http.Client{Timeout: cfg.Timeout}
	payload, err := fetchJSON(ctx, client, cfg, cfg.BaseURL+"/v0/management/usage")
	if err != nil {
		return tokenUsageResult{}, err
	}
	return parseTokenUsageByAuth(payload, now), nil
}

func fetchJSON(ctx context.Context, client *http.Client, cfg config, url string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if cfg.ManagementKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.ManagementKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeResponse(resp)
}

func postJSON(ctx context.Context, client *http.Client, cfg config, url string, payload map[string]any) (map[string]any, error) {
	return sendJSON(ctx, client, cfg, http.MethodPost, url, payload)
}

func patchJSON(ctx context.Context, client *http.Client, cfg config, url string, payload map[string]any) (map[string]any, error) {
	return sendJSON(ctx, client, cfg, http.MethodPatch, url, payload)
}

func sendJSON(ctx context.Context, client *http.Client, cfg config, method, url string, payload map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if cfg.ManagementKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.ManagementKey)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeResponse(resp)
}

func disableAuthEntry(ctx context.Context, client *http.Client, cfg config, entry authEntry) error {
	name := authIdentifier(entry.raw)
	if name == "" {
		return errors.New("missing auth name or id")
	}
	_, err := patchJSON(ctx, client, cfg, cfg.BaseURL+"/v0/management/auth-files/status", map[string]any{
		"name":     name,
		"disabled": true,
	})
	return err
}

func decodeResponse(resp *http.Response) (map[string]any, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("management API HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func parseBody(body any) (map[string]any, error) {
	switch v := body.(type) {
	case map[string]any:
		return v, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, errors.New("empty")
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, err
		}
		return out, nil
	default:
		return nil, errors.New("invalid")
	}
}

func isAuthDisabled(entry map[string]any) bool {
	if boolFromAny(entry["disabled"]) {
		return true
	}
	return strings.EqualFold(cleanString(entry["status"]), "disabled")
}

func authIdentifier(entry map[string]any) string {
	return cleanString(firstValue(entry["name"], entry["id"]))
}

func isTokenExpiredResponse(statusCode int, parsedBody map[string]any, body any) bool {
	if statusCode != http.StatusUnauthorized && intFromAny(firstValue(parsedBody["status"])) != http.StatusUnauthorized {
		return false
	}
	if strings.EqualFold(cleanString(nested(parsedBody, "error", "code")), "token_expired") {
		return true
	}
	message := strings.ToLower(cleanString(nested(parsedBody, "error", "message")))
	if strings.Contains(message, "provided authentication token is expired") {
		return true
	}
	raw := strings.ToLower(bodyString(body))
	return strings.Contains(raw, "\"code\":\"token_expired\"") || strings.Contains(raw, "provided authentication token is expired")
}

func tokenExpiredErrorMessage(statusCode int, parsedBody map[string]any, body any) string {
	if message := cleanString(nested(parsedBody, "error", "message")); message != "" {
		return message
	}
	if raw := bodyString(body); raw != "" {
		return raw
	}
	if statusCode > 0 {
		return fmt.Sprintf("HTTP %d", statusCode)
	}
	return "token_expired"
}

func parseAccountID(entry map[string]any) string {
	candidates := []any{
		entry["id_token"],
		nested(entry, "metadata", "id_token"),
		nested(entry, "attributes", "id_token"),
	}
	for _, candidate := range candidates {
		payload := parseJWTLike(candidate)
		if payload == nil {
			continue
		}
		if accountID := cleanString(payload["chatgpt_account_id"]); accountID != "" {
			return accountID
		}
		if authInfo, ok := payload["https://api.openai.com/auth"].(map[string]any); ok {
			if accountID := cleanString(authInfo["chatgpt_account_id"]); accountID != "" {
				return accountID
			}
		}
	}
	return ""
}

func parsePlanType(entry map[string]any) string {
	candidates := []any{
		entry["plan_type"],
		entry["planType"],
		nested(entry, "metadata", "plan_type"),
		nested(entry, "metadata", "planType"),
		nested(entry, "attributes", "plan_type"),
		nested(entry, "attributes", "planType"),
	}
	for _, candidate := range candidates {
		if plan := normalizePlan(candidate); plan != "" {
			return plan
		}
	}
	return ""
}

func parseJWTLike(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case string:
		raw := strings.TrimSpace(v)
		if raw == "" {
			return nil
		}
		var out map[string]any
		if json.Unmarshal([]byte(raw), &out) == nil {
			return out
		}
		parts := strings.Split(raw, ".")
		if len(parts) < 2 {
			return nil
		}
		payload, err := decodeBase64URL(parts[1])
		if err != nil {
			return nil
		}
		if json.Unmarshal(payload, &out) != nil {
			return nil
		}
		return out
	default:
		return nil
	}
}

func decodeBase64URL(v string) ([]byte, error) {
	switch len(v) % 4 {
	case 2:
		v += "=="
	case 3:
		v += "="
	}
	return base64.URLEncoding.DecodeString(v)
}

func parseCodexWindows(payload map[string]any) []quotaWindow {
	rateLimit, _ := firstValue(payload["rate_limit"], payload["rateLimit"]).(map[string]any)
	fiveHour, weekly := findQuotaWindows(rateLimit)
	mainLimitReached := anyFromMap(rateLimit, "limit_reached", "limitReached")
	mainAllowed := anyFromMap(rateLimit, "allowed")
	var windows []quotaWindow
	if window := buildWindow("code-5h", "Code 5h", fiveHour, mainLimitReached, mainAllowed); window != nil {
		windows = append(windows, *window)
	}
	if window := buildWindow("code-7d", "Code 7d", weekly, mainLimitReached, mainAllowed); window != nil {
		windows = append(windows, *window)
	}
	return windows
}

func parseAdditionalWindows(payload map[string]any) []quotaWindow {
	raw, ok := firstValue(payload["additional_rate_limits"], payload["additionalRateLimits"]).([]any)
	if !ok {
		return nil
	}
	var windows []quotaWindow
	for i, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rateLimit, ok := firstValue(entry["rate_limit"], entry["rateLimit"]).(map[string]any)
		if !ok {
			continue
		}
		name := cleanString(firstValue(entry["limit_name"], entry["limitName"], entry["metered_feature"], entry["meteredFeature"]))
		if name == "" {
			name = fmt.Sprintf("additional-%d", i+1)
		}
		primary, _ := firstValue(rateLimit["primary_window"], rateLimit["primaryWindow"]).(map[string]any)
		secondary, _ := firstValue(rateLimit["secondary_window"], rateLimit["secondaryWindow"]).(map[string]any)
		if window := buildWindow(name+"-primary", name+" 5h", primary, anyFromMap(rateLimit, "limit_reached", "limitReached"), anyFromMap(rateLimit, "allowed")); window != nil {
			windows = append(windows, *window)
		}
		if window := buildWindow(name+"-secondary", name+" 7d", secondary, anyFromMap(rateLimit, "limit_reached", "limitReached"), anyFromMap(rateLimit, "allowed")); window != nil {
			windows = append(windows, *window)
		}
	}
	return windows
}

func findQuotaWindows(rateLimit map[string]any) (map[string]any, map[string]any) {
	if rateLimit == nil {
		return nil, nil
	}
	primary, _ := firstValue(rateLimit["primary_window"], rateLimit["primaryWindow"]).(map[string]any)
	secondary, _ := firstValue(rateLimit["secondary_window"], rateLimit["secondaryWindow"]).(map[string]any)
	candidates := []map[string]any{primary, secondary}
	var fiveHour, weekly map[string]any
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		duration := numberFromAny(firstValue(candidate["limit_window_seconds"], candidate["limitWindowSeconds"]))
		if duration == window5HSeconds && fiveHour == nil {
			fiveHour = candidate
		}
		if duration == window7DSeconds && weekly == nil {
			weekly = candidate
		}
	}
	if fiveHour == nil && primary != nil {
		fiveHour = primary
	}
	if weekly == nil && secondary != nil {
		weekly = secondary
	}
	return fiveHour, weekly
}

func buildWindow(id, label string, window map[string]any, limitReached, allowed any) *quotaWindow {
	if window == nil {
		return nil
	}
	usedPercent := deduceUsedPercent(window, limitReached, allowed)
	var remaining *float64
	if usedPercent != nil {
		v := clampFloat(100.0-*usedPercent, 0, 100)
		remaining = &v
	}
	exhausted := usedPercent != nil && *usedPercent >= 100
	return &quotaWindow{
		ID:               id,
		Label:            label,
		UsedPercent:      usedPercent,
		RemainingPercent: remaining,
		ResetLabel:       formatResetLabel(window),
		Exhausted:        exhausted,
	}
}

func deduceUsedPercent(window map[string]any, limitReached, allowed any) *float64 {
	if used := numberPtr(firstValue(window["used_percent"], window["usedPercent"])); used != nil {
		v := clampFloat(*used, 0, 100)
		return &v
	}
	exhaustedHint := boolFromAny(limitReached) || isFalse(allowed)
	if exhaustedHint && formatResetLabel(window) != "-" {
		v := 100.0
		return &v
	}
	return nil
}

func formatResetLabel(window map[string]any) string {
	if ts := numberFromAny(firstValue(window["reset_at"], window["resetAt"])); ts > 0 {
		return time.Unix(int64(ts), 0).Local().Format("01-02 15:04")
	}
	if secs := numberFromAny(firstValue(window["reset_after_seconds"], window["resetAfterSeconds"])); secs > 0 {
		return time.Now().Add(time.Duration(secs) * time.Second).Local().Format("01-02 15:04")
	}
	return "-"
}

func deriveStatus(report quotaReport) string {
	if report.Disabled {
		return "disabled"
	}
	if report.Error != "" {
		return "error"
	}
	if report.AuthIndex == "" || report.AccountID == "" {
		return "missing"
	}
	window7d := findWindow(report.Windows, "code-7d")
	if window7d == nil || window7d.RemainingPercent == nil {
		return "unknown"
	}
	remaining := *window7d.RemainingPercent
	if remaining <= 0 {
		return "exhausted"
	}
	if remaining <= 30 {
		return "low"
	}
	if remaining <= 70 {
		return "medium"
	}
	if remaining < 100 {
		return "high"
	}
	return "full"
}

func shouldRetryError(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return false
	}
	markers := []string{
		"request failed",
		"timed out",
		"timeout",
		"temporarily unavailable",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
		"connection reset",
		"remote end closed connection",
		"operation not permitted",
	}
	for _, marker := range markers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func parseTokenUsageByAuth(payload map[string]any, now time.Time) tokenUsageResult {
	usage, _ := payload["usage"].(map[string]any)
	apis, _ := usage["apis"].(map[string]any)
	if len(apis) == 0 {
		return tokenUsageResult{ByAuth: map[string]tokenUsageSummary{}}
	}

	if now.IsZero() {
		now = time.Now()
	}
	last7Hours := now.Add(-7 * time.Hour)
	last24Hours := now.Add(-24 * time.Hour)
	last7Days := now.Add(-7 * 24 * time.Hour)

	out := make(map[string]tokenUsageSummary)
	var historyStart time.Time
	var historyEnd time.Time
	for _, apiValue := range apis {
		apiEntry, ok := apiValue.(map[string]any)
		if !ok {
			continue
		}
		models, _ := apiEntry["models"].(map[string]any)
		for _, modelValue := range models {
			modelEntry, ok := modelValue.(map[string]any)
			if !ok {
				continue
			}
			details, _ := modelEntry["details"].([]any)
			for _, detailValue := range details {
				detail, ok := detailValue.(map[string]any)
				if !ok {
					continue
				}
				authIndex := cleanString(firstValue(detail["auth_index"], detail["authIndex"]))
				timestampText := cleanString(detail["timestamp"])
				if authIndex == "" || timestampText == "" {
					continue
				}
				timestamp, err := time.Parse(time.RFC3339Nano, timestampText)
				if err != nil {
					continue
				}
				if historyStart.IsZero() || timestamp.Before(historyStart) {
					historyStart = timestamp
				}
				if historyEnd.IsZero() || timestamp.After(historyEnd) {
					historyEnd = timestamp
				}
				totalTokens := tokenTotalFromDetail(detail)
				current := out[authIndex]
				current.Available = true
				if !timestamp.Before(last7Hours) {
					current.Last7Hours += totalTokens
				}
				if !timestamp.Before(last24Hours) {
					current.Last24Hours += totalTokens
				}
				if !timestamp.Before(last7Days) {
					current.Last7Days += totalTokens
				}
				out[authIndex] = current
			}
		}
	}

	result := tokenUsageResult{
		ByAuth:       out,
		HistoryStart: historyStart,
		HistoryEnd:   historyEnd,
	}
	if historyStart.IsZero() {
		return result
	}
	result.Complete7Hours = !historyStart.After(last7Hours)
	result.Complete24Hours = !historyStart.After(last24Hours)
	result.Complete7Days = !historyStart.After(last7Days)
	return result
}

func formatTokenUsageHistoryTimestamp(value time.Time, loc *time.Location) string {
	if value.IsZero() {
		return ""
	}
	if loc == nil {
		loc = time.Local
	}
	return value.In(loc).Format(time.RFC3339Nano)
}

func tokenTotalFromDetail(detail map[string]any) int64 {
	tokens, _ := detail["tokens"].(map[string]any)
	if tokens == nil {
		return 0
	}
	if raw := firstValue(tokens["total_tokens"], tokens["totalTokens"]); raw != nil && isNumberish(raw) {
		return int64(numberFromAny(raw))
	}
	var total int64
	for _, raw := range []any{
		firstValue(tokens["input_tokens"], tokens["inputTokens"]),
		firstValue(tokens["output_tokens"], tokens["outputTokens"]),
		firstValue(tokens["reasoning_tokens"], tokens["reasoningTokens"]),
	} {
		if raw == nil || !isNumberish(raw) {
			continue
		}
		total += int64(numberFromAny(raw))
	}
	return total
}

func filterReports(reports []quotaReport, plan, status string) []quotaReport {
	var out []quotaReport
	plan = strings.ToLower(strings.TrimSpace(plan))
	status = strings.ToLower(strings.TrimSpace(status))
	for _, report := range reports {
		if plan != "" && strings.ToLower(report.PlanType) != plan {
			continue
		}
		if status != "" && strings.ToLower(report.Status) != status {
			continue
		}
		out = append(out, report)
	}
	return out
}

func summarize(reports []quotaReport) summary {
	sum := summary{
		Accounts:     len(reports),
		StatusCounts: map[string]int{},
		PlanCounts:   map[string]int{},
	}
	for _, report := range reports {
		sum.StatusCounts[report.Status]++
		plan := report.PlanType
		if plan == "" {
			plan = "unknown"
		}
		sum.PlanCounts[plan]++
		if report.Status == "exhausted" {
			sum.ExhaustedAccounts++
			sum.ExhaustedNames = append(sum.ExhaustedNames, report.Name)
		}
		if report.Status == "low" {
			sum.LowAccounts++
			sum.LowNames = append(sum.LowNames, report.Name)
		}
		if report.Status == "error" || report.Status == "missing" {
			sum.ErrorAccounts++
			sum.ErrorNames = append(sum.ErrorNames, report.Name)
		}
		sum.AdditionalWindows += len(report.AdditionalWindows)

		window7d := findWindow(report.Windows, "code-7d")
		if window7d != nil && window7d.RemainingPercent != nil {
			switch strings.ToLower(strings.TrimSpace(report.PlanType)) {
			case "free":
				sum.FreeEquivalent7D += *window7d.RemainingPercent
			case "plus":
				sum.PlusEquivalent7D += *window7d.RemainingPercent
			}
		}

		if report.tokenUsage.Available {
			if !sum.TokenUsage.Available {
				sum.TokenUsage.Available = true
				sum.TokenUsage.HistoryStart = report.tokenUsage.HistoryStart
				sum.TokenUsage.HistoryEnd = report.tokenUsage.HistoryEnd
				sum.TokenUsage.Complete7Hours = report.tokenUsage.Complete7Hours
				sum.TokenUsage.Complete24Hours = report.tokenUsage.Complete24Hours
				sum.TokenUsage.Complete7Days = report.tokenUsage.Complete7Days
			}
			sum.TokenUsage.Last7Hours += report.tokenUsage.Last7Hours
			sum.TokenUsage.Last24Hours += report.tokenUsage.Last24Hours
			sum.TokenUsage.Last7Days += report.tokenUsage.Last7Days
		}
	}
	return sum
}
