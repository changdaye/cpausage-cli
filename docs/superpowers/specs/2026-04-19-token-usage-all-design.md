# token_usage_all Design

## Goal

在 `cpausage` 的汇总输出中增加 `token_usage_all`，与现有 `token_usage_7h`、`token_usage_24h`、`token_usage_7d` 一起展示。

## Scope

- 复用现有 `wham/usage` 返回的明细和历史边界。
- `token_usage_all` 表示当前接口可返回历史范围内的累计 token 总量。
- 保持现有 7h / 24h / 7d 统计逻辑不变。

## Design

- 在 `tokenUsageSummary` 中新增 `AllTime int64` 字段，并让 `summary.TokenUsage` 一并聚合该值。
- 在 `parseTokenUsageByAuth` 中，遍历历史明细时同步累计每个 `auth_index` 的全量 token。
- 在 `summarize` 中累计所有账号的 `AllTime`。
- 在渲染层统一支持一个新的窗口标识 `all`：
  - `renderPlain` 输出 `Token Usage All`
  - `renderPrettyReportStyle1` 底部摘要输出 `token_usage_all`
  - `renderSummaryCards` 增加 `Tokens All` 卡片
- 保持 JSON 输出与 `summary` / `reports` 结构一致，新增字段自动可见。

## Testing

- 为 `parseTokenUsageByAuth` / `summarize` 补充或更新测试，验证 `AllTime` 聚合正确。
- 为渲染输出补一个最小测试，验证摘要中出现 `token_usage_all`。
