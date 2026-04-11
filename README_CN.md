# CLIProxyAPI Quota Inspector

---

![CLIProxyAPI Quota Inspector](./img.png)

基于 CPA 管理接口的实时配额查询工具。

该项目通过已运行的 CPA 服务读取真实配额窗口，并输出终端报表，支持计划类型排序、状态着色、配额进度条和汇总统计。

## 作用

- 使用在线数据，不做离线估算。
- 展示每个账号的 `code 5h` 与 `code 7d` 配额窗口。
- 汇总不同计划的等效百分比（`free`、`plus`）。
- 查询大量账号时显示实时进度（含当前凭证文件名）。

## 数据来源

工具复用 CPA 当前支持提供方的查询链路：

1. `GET /v0/management/auth-files`
2. `POST /v0/management/api-call`

## 状态规则

状态按 `code-7d` 剩余额度计算：

- `0` -> `exhausted`
- `0-30` -> `low`
- `30-70` -> `medium`
- `70-100` -> `high`
- `100` -> `full`

## 功能特性

- 默认静态报表输出（非交互模式）
- 表格宽度自适应终端
- 默认 Unicode 渐变进度条，可切换 ASCII
- 可选实时查询进度条
- 支持 JSON 输出，便于自动化
- 支持失败重试

## 运行要求

- Go `1.25+`
- CPA 服务已启动
- 管理密钥（若 CPA 开启鉴权）

## 构建

```bash
go build -o cpa-quota-inspector .
```

## 快速使用

```bash
./cpa-quota-inspector -k YOUR_MANAGEMENT_KEY
```

## 参数说明

- `--cpa-base-url`: CPA 地址
- `--management-key`, `-k`: 管理密钥
- `--concurrency`: 并发查询数
- `--timeout`: 请求超时秒数
- `--retry-attempts`: 临时失败重试次数
- `--version`: 输出版本/构建信息
- `--filter-plan`: 按计划类型过滤
- `--filter-status`: 按状态过滤
- `--json`: 输出 JSON
- `--plain`: 输出纯文本
- `--summary-only`: 仅输出汇总
- `--ascii-bars`: 使用 ASCII 进度条
- `--no-progress`: 关闭查询进度显示

## 示例

JSON 输出：

```bash
./cpa-quota-inspector \
  --json \
  --cpa-base-url http://127.0.0.1:8317 \
  -k YOUR_MANAGEMENT_KEY
```

关闭查询进度：

```bash
./cpa-quota-inspector \
  --no-progress \
  --cpa-base-url http://127.0.0.1:8317 \
  -k YOUR_MANAGEMENT_KEY
```

使用 ASCII 进度条：

```bash
./cpa-quota-inspector \
  --ascii-bars \
  --cpa-base-url http://127.0.0.1:8317 \
  -k YOUR_MANAGEMENT_KEY
```

查看版本信息：

```bash
./cpa-quota-inspector --version
```

## 排序与汇总

- 默认排序：先按计划优先级（`free`、`team`、`plus`、其他），再按 `code-7d` 剩余额度升序。
- 汇总包含：
  - `plan_counts`
  - `status_counts`
  - `free_equivalent_7d`
  - `plus_equivalent_7d`

## 代码结构

- `main.go`: 入口与流程编排
- `types.go`: 常量与数据模型
- `fetch.go`: 请求、解析、状态计算
- `render.go`: 终端报表渲染
- `helpers.go`: 通用辅助函数

## 开发

```bash
gofmt -w *.go
go test ./...
```

## 发布

创建并推送语义化标签：

```bash
git checkout main
git pull
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

使用 GoReleaser 构建多平台产物：

```bash
goreleaser release --clean
```

## 说明

- 当前不展示 code review 配额。
