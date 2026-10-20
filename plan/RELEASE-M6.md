# plan/RELEASE-M6 · 发布留档

> 两栏纪律：§一 **tag 前落盘**（本提交），§二 tag 后据实追加。所有 sha256 在 §二 tag 后复核回填。

## 一、tag 前事实

| 项 | 值 |
|---|---|
| 版本号 | `0.1.0`（`internal/version.Version`，2026-10-20 定版；M0–M5 期为 `0.1.0-dev`） |
| schema | `report.json` schema = 1（`SchemaVersion`） |
| 预定 tag | `v0.1.0`（tag 后追加实际 tag 对象 sha 于 §二） |
| 样张数 | 14 组（`TestCaseMatrixIs14AndCoversT1ToT12`；`gen_fixtures --check` → `14 cases, 0 diff`） |
| Go 测试计数 | 10 个包含测试全绿（`go test ./...`）；`go vet ./...` 干净；`gofmt -l .` 为空 |
| 布局门 | `node tools/check-sprint-layout.mjs` 全绿（含 `LAYOUT.RELEASE` 本文件判据） |
| 布局门变异 | `node tools/verify-layout-mutations.mjs` 11/11 全红 |
| Go 守门变异 | `bash tools/verify-go-guards.sh` 13/13 全红 |
| CI 本地等价 | `bash tools/check-ci-example.sh` 全 PASS（0/1/2/3 四分支 + 放行 ≠ 消失 + 落盘） |
| 干净环境验证 | `git archive HEAD` 解包树（无 `.git`）：`go build ./...`、`bdiff version`、`bdiff selftest`、README 三场景（exit 0/2/2/3）、`gen_fixtures --check`、两个变异脚本 —— 全部通过（2026-10-20） |
| 脱敏扫描 | 绝对路径 / 主机名 / 用户名 / 邮箱：零真实命中（`example.invalid` 为文档占位；`yuluo554` 为仓库署名，属发布身份） |
| golden 定版核对 | `TestReportGolden -update` 后 `git diff data/golden/report/` 仅 14 份 `.json` 各 1 行 `tool_version` 变化 |
| 交付物 | `dist/bdiff-windows-amd64.exe`（sha256 见 §二）；Release 页 ≥1 手工上传附件 |
| 远端/CI | tag 前无远端（U1）；首推后据实补记 run id 于 §二 |

## 二、tag 后追加（发布后回填）

- tag `v0.1.0`（annotated）已建，指向 commit `8591ba5`（M6 定版提交，含 `plan/RELEASE-M6.md`，
  `git cat-file -e v0.1.0:plan/RELEASE-M6.md` 通过）；建 tag 时间 2026-10-20（本地）。
- `dist/bdiff-windows-amd64.exe` sha256（本地留副本，2026-10-20 实测，R9 修复后重建）：
  `2172a1e97fb370be8f05c5334002ad103e0f24a03ab2c641e7822999034b3021`
  （`dist/SHA256SUMS` 同内容；`bdiff version` → `0.1.0`）。
- （待回填）GitHub Release URL 与附件清单：
- （待回填）公开仓库 URL 与首推 CI run id：

## 三、阶段 7 覆盖对账表

| 承诺项（plan/00 §六/台账） | 状态 | 证据 |
|---|---|---|
| 干净环境逐条跑通 README 命令 | 实现 | `git archive` 解包树实测（§一） |
| 脱敏四步零命中 | 实现 | §一扫描记录 + `TestReportMarkdownNoLeak` |
| 单二进制在无 Go 工具链机器运行 | 待首推 | 产物随 Release 分发后据实验证 |
| CI 实跑（U1） | 待首推 | 首推后据实补记 run id |
| 跨平台 CI 矩阵（U2） | 待首推 | `ci.yml` ubuntu 矩阵已配置，待远端验证 |
| 真实发行版产物覆盖（U5/R4） | 不适用 | 边界承诺：仅合成样张，不承诺真实发行版 |
