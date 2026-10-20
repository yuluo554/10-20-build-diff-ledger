// Package version 是产品版本号的唯一来源。
//
// 纪律（plan/02 §三）：`bdiff version`、`report.json.tool_version`、发布留档
// 三处都读这里的常量，定版只改这一行。
package version

// Version 是当前产品版本。M6 已定版为发布版 0.1.0（2026-10-20）；
// 同步 README 顶部与 plan/00 的状态行。
const Version = "0.1.0"

// SchemaVersion 是 report.json 的结构版本；报告键序或字段语义变更时 +1。
const SchemaVersion = 1
