package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const quotaBarCells = 20

func renderPlain(reports []quotaReport, sum summary, summaryOnly bool) {
	fmt.Printf("Accounts: %d\n", sum.Accounts)
	fmt.Printf("Exhausted: %d\n", sum.ExhaustedAccounts)
	fmt.Printf("Disabled: %d\n", sum.StatusCounts["disabled"])
	fmt.Printf("Low: %d\n", sum.LowAccounts)
	fmt.Printf("Errors: %d\n", sum.ErrorAccounts)
	fmt.Printf("Free Equivalent 7d: %.0f%%\n", sum.FreeEquivalent7D)
	fmt.Printf("Token Usage 7h: %s\n", formatTokenUsageValue(sum.TokenUsage, tokenUsageWindow7Hours))
	fmt.Printf("Token Usage 24h: %s\n", formatTokenUsageValue(sum.TokenUsage, tokenUsageWindow24Hours))
	fmt.Printf("Token Usage 7d: %s\n", formatTokenUsageValue(sum.TokenUsage, tokenUsageWindow7Days))
	fmt.Printf("Token Usage All: %s\n", formatTokenUsageValue(sum.TokenUsage, tokenUsageWindowAll))
	if summaryOnly {
		return
	}
	for _, report := range reports {
		fmt.Printf("\n%s [%s] %s switch=%s\n", report.Name, defaultString(report.PlanType, "unknown"), report.Status, accountSwitchLabel(report.Disabled))
		if report.Error != "" {
			fmt.Printf("  error: %s\n", report.Error)
		}
		for _, window := range report.Windows {
			fmt.Printf("  %s: %s reset=%s\n", window.Label, asciiProgress(window.RemainingPercent, 18), window.ResetLabel)
		}
		for _, window := range report.AdditionalWindows {
			fmt.Printf("  %s: %s reset=%s\n", window.Label, asciiProgress(window.RemainingPercent, 18), window.ResetLabel)
		}
	}
}

func renderPrettyReport(reports []quotaReport, sum summary, cfg config) {
	switch cfg.Style {
	case "2":
		renderPrettyReportStyle2(reports, sum, cfg)
	default:
		renderPrettyReportStyle1(reports, sum, cfg)
	}
}

func renderPrettyReportStyle1(reports []quotaReport, sum summary, cfg config) {
	themeTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FDE68A"))
	themeSub := lipgloss.NewStyle().Foreground(lipgloss.Color("#FDBA74"))
	themeDim := lipgloss.NewStyle().Foreground(lipgloss.Color("#A8A29E"))
	fullStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#84CC16"))
	highStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#22C55E"))
	mediumStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981"))
	lowStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B"))
	errStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444"))
	tableHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FED7AA"))
	rowAlt := lipgloss.NewStyle().Foreground(lipgloss.Color("#F5F5F4"))
	rowBase := lipgloss.NewStyle().Foreground(lipgloss.Color("#E7E5E4"))

	fmt.Println(themeTitle.Render("CPA Quota Inspector (Static Report)"))
	fmt.Println(themeSub.Render(fmt.Sprintf("source=%s  timeout=%s  retry=%d", cfg.BaseURL, cfg.Timeout.String(), cfg.RetryAttempts)))
	fmt.Println()

	if len(reports) == 0 {
		fmt.Println(themeDim.Render("No rows match current filters."))
		return
	}

	termWidth := detectTerminalWidth()
	wName, wStatus, wSwitch, wBar, wReset := computeColumnWidths(termWidth)

	header := padRight("File", wName) + " " +
		padRight("Code 5h", wBar) + " " +
		padRight("Reset 5h", wReset) + " " +
		padRight("Code 7d", wBar) + " " +
		padRight("Reset 7d", wReset) + " " +
		padRight("Status", wStatus) + " " +
		padRight("Switch", wSwitch)
	fmt.Println(tableHeader.Render(header))
	fmt.Println(themeDim.Render(strings.Repeat("-", lipgloss.Width(header))))

	for i, report := range reports {
		code5 := findWindow(report.Windows, "code-5h")
		code7 := findWindow(report.Windows, "code-7d")

		statusStyled := padRight(report.Status, wStatus)
		switch report.Status {
		case "full":
			statusStyled = fullStyle.Render(statusStyled)
		case "high":
			statusStyled = highStyle.Render(statusStyled)
		case "medium":
			statusStyled = mediumStyle.Render(statusStyled)
		case "low":
			statusStyled = lowStyle.Render(statusStyled)
		case "disabled":
			statusStyled = themeDim.Render(statusStyled)
		default:
			statusStyled = errStyle.Render(statusStyled)
		}

		switchStyled := renderAccountSwitch(report.Disabled, wSwitch)

		row := padRight(truncate(report.Name, wName), wName) + " " +
			padRight(prettyBar(code5, wBar, cfg.ASCIIBars), wBar) + " " +
			padRight(resetLabel(code5), wReset) + " " +
			padRight(prettyBar(code7, wBar, cfg.ASCIIBars), wBar) + " " +
			padRight(resetLabel(code7), wReset) + " " +
			statusStyled + " " +
			switchStyled

		if i%2 == 0 {
			fmt.Println(rowBase.Render(row))
		} else {
			fmt.Println(rowAlt.Render(row))
		}
		if report.Error != "" {
			fmt.Println(themeDim.Render("  error: " + report.Error))
		}
	}

	fmt.Println()
	fmt.Println(themeTitle.Render("Status Overview"))
	overviewHeader := padRight("Accounts", 10) + " " + padRight("Full", 8) + " " + padRight("High", 8) + " " + padRight("Medium", 8) + " " + padRight("Low", 8) + " " + padRight("Exhausted", 10) + " " + padRight("Disabled", 10)
	fmt.Println(tableHeader.Render(overviewHeader))
	fmt.Println(themeDim.Render(strings.Repeat("-", lipgloss.Width(overviewHeader))))
	overviewLine := padRight(strconv.Itoa(sum.Accounts), 10) + " " +
		fullStyle.Render(padRight(strconv.Itoa(sum.StatusCounts["full"]), 8)) + " " +
		highStyle.Render(padRight(strconv.Itoa(sum.StatusCounts["high"]), 8)) + " " +
		mediumStyle.Render(padRight(strconv.Itoa(sum.StatusCounts["medium"]), 8)) + " " +
		lowStyle.Render(padRight(strconv.Itoa(sum.StatusCounts["low"]), 8)) + " " +
		errStyle.Render(padRight(strconv.Itoa(sum.ExhaustedAccounts), 10)) + " " +
		themeDim.Render(padRight(strconv.Itoa(sum.StatusCounts["disabled"]), 10))
	fmt.Println(rowBase.Render(overviewLine))

	fmt.Println()
	fmt.Println(themeTitle.Render("Summary"))
	fmt.Println(themeDim.Render(fmt.Sprintf("free_equivalent_7d: %.0f%%", sum.FreeEquivalent7D)))
	fmt.Println(themeDim.Render("token_usage_7h: " + formatTokenUsageValue(sum.TokenUsage, tokenUsageWindow7Hours)))
	fmt.Println(themeDim.Render("token_usage_24h: " + formatTokenUsageValue(sum.TokenUsage, tokenUsageWindow24Hours)))
	fmt.Println(themeDim.Render("token_usage_7d: " + formatTokenUsageValue(sum.TokenUsage, tokenUsageWindow7Days)))
	fmt.Println(themeDim.Render("token_usage_all: " + formatTokenUsageValue(sum.TokenUsage, tokenUsageWindowAll)))
}

func renderPrettyReportStyle2(reports []quotaReport, sum summary, cfg config) {
	termWidth := detectTerminalWidth()
	themeTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FDE68A"))
	themeSub := lipgloss.NewStyle().Foreground(lipgloss.Color("#FDBA74"))
	themeDim := lipgloss.NewStyle().Foreground(lipgloss.Color("#A8A29E"))
	tableHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FED7AA"))
	rowAlt := lipgloss.NewStyle().Foreground(lipgloss.Color("#F5F5F4"))
	rowBase := lipgloss.NewStyle().Foreground(lipgloss.Color("#E7E5E4"))

	fmt.Println(themeSub.Render(fmt.Sprintf("source=%s  timeout=%s  retry=%d", cfg.BaseURL, cfg.Timeout.String(), cfg.RetryAttempts)))

	if len(reports) == 0 {
		fmt.Println()
		fmt.Println(themeDim.Render("No rows match current filters."))
		fmt.Println()
		fmt.Println(renderSummaryCards(sum, termWidth))
		return
	}

	fmt.Println()
	fmt.Println(themeTitle.Render("Accounts"))
	wName, wStatus, wSwitch, wBar, wReset := computeColumnWidths(termWidth)

	header := padRight("Account", wName) + " " +
		padRight("Code 5h", wBar) + " " +
		padRight("Reset 5h", wReset) + " " +
		padRight("Code 7d", wBar) + " " +
		padRight("Reset 7d", wReset) + " " +
		padRight("Status", wStatus) + " " +
		padRight("Switch", wSwitch)
	fmt.Println(tableHeader.Render(header))
	fmt.Println(themeDim.Render(strings.Repeat("-", lipgloss.Width(header))))

	for i, report := range reports {
		code5 := findWindow(report.Windows, "code-5h")
		code7 := findWindow(report.Windows, "code-7d")

		statusStyled := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(statusColor(report.Status))).
			Render(padRight(report.Status, wStatus))

		switchStyled := renderAccountSwitch(report.Disabled, wSwitch)

		row := padRight(truncate(report.Name, wName), wName) + " " +
			padRight(prettyBar(code5, wBar, cfg.ASCIIBars), wBar) + " " +
			padRight(resetLabel(code5), wReset) + " " +
			padRight(prettyBar(code7, wBar, cfg.ASCIIBars), wBar) + " " +
			padRight(resetLabel(code7), wReset) + " " +
			statusStyled + " " +
			switchStyled

		if i%2 == 0 {
			fmt.Println(rowBase.Render(row))
		} else {
			fmt.Println(rowAlt.Render(row))
		}
		if report.Error != "" {
			fmt.Println(themeDim.Render("  error: " + report.Error))
		}
	}

	fmt.Println()
	fmt.Println(renderSummaryCards(sum, termWidth))
}

func renderSummaryCards(sum summary, termWidth int) string {
	summaryCards := []string{
		renderMetricCard("Total", fmt.Sprintf("%d", sum.Accounts), "", "#FDE68A", 16),
		renderMetricCard("Free", fmt.Sprintf("%.0f%%", sum.FreeEquivalent7D), "", "#34D399", 16),
	}

	tokenCards := []string{
		renderMetricCard("Tokens 7h", formatTokenUsageValue(sum.TokenUsage, tokenUsageWindow7Hours), "", "#F97316", 18),
		renderMetricCard("Tokens 24h", formatTokenUsageValue(sum.TokenUsage, tokenUsageWindow24Hours), "", "#FB923C", 18),
		renderMetricCard("Tokens 7d", formatTokenUsageValue(sum.TokenUsage, tokenUsageWindow7Days), "", "#F59E0B", 18),
		renderMetricCard("Tokens All", formatTokenUsageValue(sum.TokenUsage, tokenUsageWindowAll), "", "#D97706", 18),
	}

	return renderCardRows(summaryCards, termWidth, 2) + "\n\n" +
		renderCardRows(tokenCards, termWidth, 2)
}

func renderMetricCard(title, value, subtitle, accent string, minWidth int) string {
	contentWidth := max(minWidth, max(displayWidth(title), max(displayWidth(value), displayWidth(subtitle))))
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(accent)).
		Padding(0, 1).
		Width(contentWidth)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E7E5E4"))
	valueStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accent))
	subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#A8A29E"))

	if strings.TrimSpace(subtitle) == "" {
		contentWidth = max(minWidth, displayWidth(title)+1+displayWidth(value)+4)
		cardStyle = cardStyle.Width(contentWidth)
		return cardStyle.Render(titleStyle.Render(title) + " " + valueStyle.Render(value))
	}

	body := titleStyle.Render(title) + "\n" + valueStyle.Render(value) + "\n" + subtitleStyle.Render(subtitle)
	return cardStyle.Render(body)
}

func renderCardRows(cards []string, totalWidth, gap int) string {
	if len(cards) == 0 {
		return ""
	}

	limit := max(40, totalWidth-8)
	rows := make([]string, 0, len(cards))
	current := make([]string, 0, len(cards))
	currentWidth := 0

	for _, card := range cards {
		cardWidth := displayWidth(card)
		nextWidth := cardWidth
		if len(current) > 0 {
			nextWidth = currentWidth + gap + cardWidth
		}
		if len(current) > 0 && nextWidth > limit {
			rows = append(rows, joinCardRow(current, gap))
			current = []string{card}
			currentWidth = cardWidth
			continue
		}
		if len(current) == 0 {
			current = []string{card}
			currentWidth = cardWidth
			continue
		}
		current = append(current, card)
		currentWidth = nextWidth
	}

	if len(current) > 0 {
		rows = append(rows, joinCardRow(current, gap))
	}
	return strings.Join(rows, "\n")
}

func joinCardRow(cards []string, gap int) string {
	if len(cards) == 0 {
		return ""
	}

	parts := make([]string, 0, len(cards)*2-1)
	space := strings.Repeat(" ", max(1, gap))
	for i, card := range cards {
		if i > 0 {
			parts = append(parts, space)
		}
		parts = append(parts, card)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func statusDisplayName(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "full":
		return "Full"
	case "high":
		return "High"
	case "medium":
		return "Medium"
	case "low":
		return "Low"
	case "exhausted":
		return "Exhausted"
	case "disabled":
		return "Disabled"
	case "error":
		return "Errors"
	case "missing":
		return "Missing"
	default:
		if status == "" {
			return "Unknown"
		}
		return strings.ToUpper(status[:1]) + status[1:]
	}
}

func statusColor(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "full":
		return "#84CC16"
	case "high":
		return "#22C55E"
	case "medium":
		return "#10B981"
	case "low":
		return "#F59E0B"
	case "disabled":
		return "#A8A29E"
	case "exhausted", "error", "missing", "unknown":
		return "#EF4444"
	default:
		return "#E7E5E4"
	}
}

func planColor(plan string) string {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case "free":
		return "#34D399"
	case "plus":
		return "#60A5FA"
	case "team":
		return "#FACC15"
	default:
		return ""
	}
}

func detectTerminalWidth() int {
	fd := int(os.Stdout.Fd())
	if term.IsTerminal(fd) {
		if w, _, err := term.GetSize(fd); err == nil && w > 0 {
			return w
		}
	}
	return 140
}

func isStdoutTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func computeColumnWidths(total int) (int, int, int, int, int) {
	wSwitch, wStatus, wReset := 6, 10, 12
	wName := 28
	wBar := quotaBarDisplayWidth()
	switch {
	case total >= 180:
		wName = 38
	case total >= 160:
		wName = 32
	case total >= 140:
		wName = 26
	case total >= 120:
		wName = 22
	case total >= 112:
		wName = 18
	default:
		wName = 18
	}
	for {
		current := wName + wBar + wReset + wBar + wReset + wStatus + wSwitch + 6
		if current <= total {
			break
		}
		switch {
		case wName > 12:
			wName--
		case wStatus > 8:
			wStatus--
		case wSwitch > 4:
			wSwitch--
		case wReset > 10:
			wReset--
		default:
			return wName, wStatus, wSwitch, wBar, wReset
		}
	}
	return wName, wStatus, wSwitch, wBar, wReset
}

func accountSwitchLabel(disabled bool) string {
	if disabled {
		return "off"
	}
	return "on"
}

func renderAccountSwitch(disabled bool, width int) string {
	label := padRight(accountSwitchLabel(disabled), width)
	if disabled {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444")).Render(label)
	}
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#22C55E")).Render(label)
}

func prettyBar(window *quotaWindow, width int, ascii bool) string {
	if window == nil || window.RemainingPercent == nil {
		return "-"
	}
	v := clampFloat(*window.RemainingPercent, 0, 100)
	percent := fmt.Sprintf(" %3.0f%%", v)
	filled := quotaBarFilledCells(v)
	unfilledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#57534E"))
	percentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAtPercent(v))).Bold(true)
	if ascii {
		var b strings.Builder
		for i := 0; i < filled; i++ {
			posPct := (float64(i+1) / float64(quotaBarCells)) * 100
			segStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAtPercent(posPct)))
			b.WriteString(segStyle.Render("#"))
		}
		b.WriteString(unfilledStyle.Render(strings.Repeat("-", max(0, quotaBarCells-filled))))
		return "[" + b.String() + "]" + percentStyle.Render(percent)
	}
	var b strings.Builder
	for i := 0; i < filled; i++ {
		posPct := (float64(i+1) / float64(quotaBarCells)) * 100
		segStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAtPercent(posPct)))
		b.WriteString(segStyle.Render("█"))
	}
	b.WriteString(unfilledStyle.Render(strings.Repeat("░", max(0, quotaBarCells-filled))))
	return "[" + b.String() + "]" + percentStyle.Render(percent)
}

func quotaBarFilledCells(value float64) int {
	value = clampFloat(value, 0, 100)
	filled := int((value / 100 * float64(quotaBarCells)) + 0.5)
	if filled > quotaBarCells {
		return quotaBarCells
	}
	return filled
}

func quotaBarDisplayWidth() int {
	return quotaBarCells + 2 + len(" 100%")
}

func progressBar(window *quotaWindow) string {
	if window == nil || window.RemainingPercent == nil {
		return "-"
	}
	return compactProgress(*window.RemainingPercent, 10)
}

func compactProgress(value float64, width int) string {
	value = clampFloat(value, 0, 100)
	filled := int((value / 100 * float64(width)) + 0.5)
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + fmt.Sprintf("] %3.0f%%", value)
}

func asciiProgress(value *float64, width int) string {
	if value == nil {
		return "-"
	}
	v := clampFloat(*value, 0, 100)
	filled := int((v / 100 * float64(width)) + 0.5)
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + fmt.Sprintf("] %3.0f%%", v)
}

const (
	tokenUsageWindow7Hours  = "7h"
	tokenUsageWindow24Hours = "24h"
	tokenUsageWindow7Days   = "7d"
	tokenUsageWindowAll     = "all"
)

func formatTokenUsageValue(usage tokenUsageSummary, window string) string {
	if !usage.Available {
		return "-"
	}

	value := int64(0)
	switch window {
	case tokenUsageWindowAll:
		value = usage.AllTime
	case tokenUsageWindow7Hours:
		value = usage.Last7Hours
	case tokenUsageWindow24Hours:
		value = usage.Last24Hours
	case tokenUsageWindow7Days:
		value = usage.Last7Days
	default:
		return "-"
	}

	return formatInt64WithCommas(value)
}
