// Package version 是产品版本号的唯一来源。
//
// 纪律（plan/02 §三）：`bdiff version`、`report.json.tool_version`、发布留档
// 三处都读这里的常量，定版只改这一行。
package version

// Version 是当前产品版本。M6 定版为 0.1.0（2026-10-20）；
// 2026-10-23 二次迭代修发布链路（SHA256SUMS 去 dist/ 前缀、Release 正文 sha、
// 统一构建命令）后升为 0.1.1；v0.1.0 的 tag 与 Release 原样保留并标注已知问题。
// 同步 README 顶部与 plan/00 的状态行。
const Version = "0.1.1"

// SchemaVersion 是 report.json 的结构版本；报告键序或字段语义变更时 +1。
const SchemaVersion = 1
