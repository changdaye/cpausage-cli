# CLIProxyAPI Quota Inspector

[中文版本](./README_CN.md)

---

![CLIProxyAPI Quota Inspector](./img.png)

Live quota inspector for CPA management APIs.

This project queries real quota windows from a running CPA instance and renders a terminal report with plan-aware sorting, status coloring, and quota bar visualization.

## Why this tool

- Uses live data from CPA management routes instead of offline estimation.
- Shows account-level `code 5h` and `code 7d` quota windows.
- Aggregates equivalent quota percentages per plan (`free`, `plus`).
- Supports progress display while querying many auth files.

## Data source

The tool mirrors CPA management flow for currently supported providers:

1. `GET /v0/management/auth-files`
2. `POST /v0/management/api-call`
3. CPA forwards upstream request to `https://chatgpt.com/backend-api/wham/usage`

## Status model

Status is derived from `code-7d` remaining percentage:

- `0` -> `exhausted`
- `0-30` -> `low`
- `30-70` -> `medium`
- `70-100` -> `high`
- `100` -> `full`

## Features

- Static report output (default) with colored plan and status.
- Terminal-width adaptive table layout.
- Unicode gradient quota bars with `--ascii-bars` fallback.
- Optional real-time fetch progress with current auth file name.
- JSON mode for automation.
- Retry for transient query failures.

## Requirements

- Go `1.25+`
- Running CPA service
- CPA management key (if enabled)

## Build

```bash
go build -o cpa-quota-inspector .
```

## Quick start

```bash
./cpa-quota-inspector -k YOUR_MANAGEMENT_KEY
```

## CLI flags

- `--cpa-base-url`: CPA base URL
- `--management-key`, `-k`: management bearer key
- `--concurrency`: concurrent quota workers
- `--timeout`: HTTP timeout seconds
- `--retry-attempts`: transient retry count
- `--version`: print version/build metadata
- `--filter-plan`: filter by `plan_type`
- `--filter-status`: filter by status
- `--json`: print JSON payload
- `--plain`: plain text output
- `--summary-only`: summary only
- `--ascii-bars`: ASCII quota bars instead of Unicode bars
- `--no-progress`: disable fetch progress line

## Examples

JSON output:

```bash
./cpa-quota-inspector \
  --json \
  --cpa-base-url http://127.0.0.1:8317 \
  -k YOUR_MANAGEMENT_KEY
```

Disable progress line:

```bash
./cpa-quota-inspector \
  --no-progress \
  --cpa-base-url http://127.0.0.1:8317 \
  -k YOUR_MANAGEMENT_KEY
```

ASCII bars:

```bash
./cpa-quota-inspector \
  --ascii-bars \
  --cpa-base-url http://127.0.0.1:8317 \
  -k YOUR_MANAGEMENT_KEY
```

Print version metadata:

```bash
./cpa-quota-inspector --version
```

## Sorting and summary

- Default order: plan rank (`free`, `team`, `plus`, others) then ascending `code-7d` remaining.
- Summary includes:
  - `plan_counts`
  - `status_counts`
  - `free_equivalent_7d`
  - `plus_equivalent_7d`

## Project structure

- `main.go`: CLI entrypoint and orchestration
- `types.go`: constants and data models
- `fetch.go`: API calls, parsing, status derivation
- `render.go`: terminal report rendering
- `helpers.go`: shared helpers and formatting utilities

## Development

Format and test:

```bash
gofmt -w *.go
go test ./...
```

## Release

Create and push a semantic tag:

```bash
git checkout main
git pull
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

Build multi-platform artifacts with GoReleaser:

```bash
goreleaser release --clean
```

## Notes

- Code review quota is intentionally not displayed.
