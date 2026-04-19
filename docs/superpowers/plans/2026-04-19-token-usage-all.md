# Token Usage All Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `token_usage_all` to cpausage summary output and JSON so total token usage across the currently available history is shown alongside 7h, 24h, and 7d values.

**Architecture:** Extend the existing token usage aggregation path instead of introducing a new fetch. Parse all history detail rows into a new `AllTime` counter, carry it through `tokenUsageSummary`, then reuse the same formatter in plain and pretty renderers.

**Tech Stack:** Go 1.25, standard library tests, existing lipgloss-based renderer

---

### Task 1: Add failing aggregation tests

**Files:**
- Modify: `usage_test.go`
- Test: `usage_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestParseTokenUsageByAuth(t *testing.T) {
	// Extend existing assertions to expect AllTime for each auth.
}

func TestSummarizeAggregatesTokenUsage(t *testing.T) {
	// Extend existing assertions to expect summary.TokenUsage.AllTime.
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./...`
Expected: FAIL because `AllTime` does not exist or is not aggregated.

- [ ] **Step 3: Write minimal implementation**

```go
type tokenUsageSummary struct {
	AllTime int64 `json:"all_time"`
}

current.AllTime += totalTokens
sum.TokenUsage.AllTime += report.tokenUsage.AllTime
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./...`
Expected: PASS for aggregation assertions.

- [ ] **Step 5: Commit**

```bash
git add usage_test.go fetch.go types.go
git commit -m "feat: aggregate all-time token usage"
```

### Task 2: Add failing renderer test for summary output

**Files:**
- Modify: `usage_test.go`
- Modify: `render.go`
- Test: `usage_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestRenderPlainIncludesTokenUsageAll(t *testing.T) {
	sum := summary{
		Accounts: 1,
		TokenUsage: tokenUsageSummary{
			Available: true,
			AllTime:   456,
		},
	}
	// Capture stdout and assert it contains "Token Usage All: 456".
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./...`
Expected: FAIL because the renderer does not print the new label yet.

- [ ] **Step 3: Write minimal implementation**

```go
const tokenUsageWindowAll = "all"

fmt.Printf("Token Usage All: %s\n", formatTokenUsageValue(sum.TokenUsage, tokenUsageWindowAll))
fmt.Println(themeDim.Render("token_usage_all: " + formatTokenUsageValue(sum.TokenUsage, tokenUsageWindowAll)))
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./...`
Expected: PASS with the new summary line present.

- [ ] **Step 5: Commit**

```bash
git add usage_test.go render.go
git commit -m "feat: render all-time token usage"
```

### Task 3: Verify full behavior stays green

**Files:**
- Modify: `README.md` only if output examples document exact summary lines
- Test: `go test ./...`

- [ ] **Step 1: Check whether docs need update**

```text
Search README.md for exact token summary examples. Update only if a literal output block would now be stale.
```

- [ ] **Step 2: Run full test suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 3: Inspect diff for scope**

Run: `git diff -- usage_test.go fetch.go types.go render.go README.md`
Expected: Only all-time token aggregation and rendering changes are present.

- [ ] **Step 4: Commit**

```bash
git add usage_test.go fetch.go types.go render.go README.md
git commit -m "feat: add all-time token usage summary"
```
