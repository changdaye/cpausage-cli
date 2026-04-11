package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

func renderPlain(reports []quotaReport, sum summary, summaryOnly bool) {
	fmt.Printf("Accounts: %d\n", sum.Accounts)
	fmt.Printf("Status counts: %v\n", sum.StatusCounts)
	fmt.Printf("Plan counts: %v\n", sum.PlanCounts)
	fmt.Printf("Exhausted: %d\n", sum.ExhaustedAccounts)
	fmt.Printf("Low: %d\n", sum.LowAccounts)
	fmt.Printf("Errors: %d\n", sum.ErrorAccounts)
	fmt.Printf("Free Equivalent 7d: %.0f%%\n", sum.FreeEquivalent7D)
	fmt.Printf("Plus Equivalent 7d: %.0f%%\n", sum.PlusEquivalent7D)
	if summaryOnly {
		return
	}
	for _, report := range reports {
		fmt.Printf("\n%s [%s] %s\n", report.Name, defaultString(report.PlanType, "unknown"), report.Status)
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
	themeTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FDE68A"))
	themeSub := lipgloss.NewStyle().Foreground(lipgloss.Color("#FDBA74"))
	themeDim := lipgloss.NewStyle().Foreground(lipgloss.Color("#A8A29E"))
	fullStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#84CC16"))
	highStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#22C55E"))
	mediumStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981"))
	lowStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B"))
	errStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444"))
	planPlus := lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))
	planTeam := lipgloss.NewStyle().Foreground(lipgloss.Color("#FACC15"))
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
	wName, wPlan, wStatus, wBar, wReset, wExtra := computeColumnWidths(termWidth)

	header := padRight("File", wName) + " " +
		padRight("Plan", wPlan) + " " +
		padRight("Status", wStatus) + " " +
		padRight("Code 5h", wBar) + " " +
		padRight("Reset 5h", wReset) + " " +
		padRight("Code 7d", wBar) + " " +
		padRight("Reset 7d", wReset) + " " +
		padRight("Extra", wExtra)
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
		default:
			statusStyled = errStyle.Render(statusStyled)
		}

		planText := defaultString(report.PlanType, "-")
		planStyled := padRight(planText, wPlan)
		switch strings.ToLower(strings.TrimSpace(planText)) {
		case "plus":
			planStyled = planPlus.Render(planStyled)
		case "team":
			planStyled = planTeam.Render(planStyled)
		}

		row := padRight(truncate(report.Name, wName), wName) + " " +
			planStyled + " " +
			statusStyled + " " +
			padRight(prettyBar(code5, wBar, cfg.ASCIIBars), wBar) + " " +
			padRight(resetLabel(code5), wReset) + " " +
			padRight(prettyBar(code7, wBar, cfg.ASCIIBars), wBar) + " " +
			padRight(resetLabel(code7), wReset) + " " +
			padRight(truncate(extraSummary(report.AdditionalWindows), wExtra), wExtra)

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
	overviewHeader := padRight("Accounts", 10) + " " + padRight("Full", 8) + " " + padRight("High", 8) + " " + padRight("Medium", 8) + " " + padRight("Low", 8) + " " + padRight("Exhausted", 10) + " " + padRight("Errors", 8)
	fmt.Println(tableHeader.Render(overviewHeader))
	fmt.Println(themeDim.Render(strings.Repeat("-", lipgloss.Width(overviewHeader))))
	overviewLine := padRight(strconv.Itoa(sum.Accounts), 10) + " " +
		fullStyle.Render(padRight(strconv.Itoa(sum.StatusCounts["full"]), 8)) + " " +
		highStyle.Render(padRight(strconv.Itoa(sum.StatusCounts["high"]), 8)) + " " +
		mediumStyle.Render(padRight(strconv.Itoa(sum.StatusCounts["medium"]), 8)) + " " +
		lowStyle.Render(padRight(strconv.Itoa(sum.StatusCounts["low"]), 8)) + " " +
		errStyle.Render(padRight(strconv.Itoa(sum.ExhaustedAccounts), 10)) + " " +
		errStyle.Render(padRight(strconv.Itoa(sum.ErrorAccounts), 8))
	fmt.Println(rowBase.Render(overviewLine))

	fmt.Println()
	fmt.Println(themeTitle.Render("Summary"))
	fmt.Println(themeDim.Render("plan_counts: " + formatCountMap(sum.PlanCounts)))
	fmt.Println(themeDim.Render("status_counts: " + formatCountMap(sum.StatusCounts)))
	fmt.Println(themeDim.Render(fmt.Sprintf("free_equivalent_7d: %.0f%%", sum.FreeEquivalent7D)))
	fmt.Println(themeDim.Render(fmt.Sprintf("plus_equivalent_7d: %.0f%%", sum.PlusEquivalent7D)))
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

func computeColumnWidths(total int) (int, int, int, int, int, int) {
	if total < 96 {
		total = 96
	}
	wPlan, wStatus, wReset := 8, 10, 12
	wName, wExtra, wBar := 28, 18, 22
	switch {
	case total >= 170:
		wName, wExtra, wBar = 36, 24, 28
	case total >= 150:
		wName, wExtra, wBar = 32, 22, 25
	case total >= 130:
		wName, wExtra, wBar = 28, 18, 21
	case total >= 110:
		wName, wExtra, wBar = 24, 12, 16
	default:
		wName, wExtra, wBar = 20, 8, 12
	}
	for {
		current := wName + wPlan + wStatus + wBar + wReset + wBar + wReset + wExtra + 7
		if current <= total {
			break
		}
		switch {
		case wExtra > 8:
			wExtra--
		case wName > 18:
			wName--
		case wBar > 10:
			wBar--
		case wPlan > 6:
			wPlan--
		case wStatus > 8:
			wStatus--
		case wReset > 10:
			wReset--
		default:
			return wName, wPlan, wStatus, wBar, wReset, wExtra
		}
	}
	return wName, wPlan, wStatus, wBar, wReset, wExtra
}

func prettyBar(window *quotaWindow, width int, ascii bool) string {
	if window == nil || window.RemainingPercent == nil {
		return "-"
	}
	if width < 8 {
		return fmt.Sprintf("%3.0f%%", clampFloat(*window.RemainingPercent, 0, 100))
	}
	v := clampFloat(*window.RemainingPercent, 0, 100)
	percent := fmt.Sprintf(" %3.0f%%", v)
	barArea := width - displayWidth(percent) - 2
	if barArea < 4 {
		return fmt.Sprintf("%3.0f%%", v)
	}
	filled := int((v / 100 * float64(barArea)) + 0.5)
	if filled > barArea {
		filled = barArea
	}
	unfilledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#57534E"))
	percentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAtPercent(v))).Bold(true)
	if ascii {
		var b strings.Builder
		for i := 0; i < filled; i++ {
			posPct := (float64(i+1) / float64(max(1, barArea))) * 100
			segStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAtPercent(posPct)))
			b.WriteString(segStyle.Render("="))
		}
		body := b.String()
		if filled < barArea {
			arrowPct := (float64(max(1, filled)) / float64(max(1, barArea))) * 100
			body += lipgloss.NewStyle().Foreground(lipgloss.Color(colorAtPercent(arrowPct))).Render(">")
			body += unfilledStyle.Render(strings.Repeat(".", max(0, barArea-filled-1)))
		}
		return "[" + body + "]" + percentStyle.Render(percent)
	}
	var b strings.Builder
	for i := 0; i < filled; i++ {
		posPct := (float64(i+1) / float64(max(1, barArea))) * 100
		segStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAtPercent(posPct)))
		b.WriteString(segStyle.Render("█"))
	}
	b.WriteString(unfilledStyle.Render(strings.Repeat("░", max(0, barArea-filled))))
	return "[" + b.String() + "]" + percentStyle.Render(percent)
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
