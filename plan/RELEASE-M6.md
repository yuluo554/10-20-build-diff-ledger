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
- `dist/bdiff-windows-amd64.exe` sha256（本地留副本，2026-10-20 实测，R9 终版修复后重建）：
  `05e7a4efee1519004e8734dc432afdff74c17842168c65146b54abddc7f80488`
  （`dist/SHA256SUMS` 同内容；`bdiff version` → `0.1.0`）。
- 公开仓库：https://github.com/yuluo554/10-20-build-diff-ledger （2026-10-20 建，首推 main + tag v0.1.0）。
- 首推 CI（修复后全绿）：run id `39448714961`（sha `30de957`；layout / determinism / Test 2×OS × 2×Go 六 job 全 ✓）。
  最终 run（收尾提交 e6e0c6e）：`39455459880` 全绿。修复过程失败 run 据实留痕：`39442868080`/`39442866112`（R9：deflate 漂移 + buildvcs）、`39443952006`/`39448230444`（R9 残余 + forbidden-import 扫描误报 guard.go 字符串字面量）。
- GitHub Release：https://github.com/yuluo554/10-20-build-diff-ledger/releases/tag/v0.1.0
  附件：`bdiff-windows-amd64.exe`（API digest 与本地 `05e7a4ef…` 一致）、`SHA256SUMS`。

## 三、阶段 7 覆盖对账表

| 承诺项（plan/00 §六/台账） | 状态 | 证据 |
|---|---|---|
| 干净环境逐条跑通 README 命令 | 实现 | `git archive` 解包树实测（§一） |
| 脱敏四步零命中 | 实现 | §一扫描记录 + `TestReportMarkdownNoLeak` |
| 单二进制在无 Go 工具链机器运行 | 实现 | Release 附件 `bdiff-windows-amd64.exe`（静态编译、零运行时；`GOOS` 交叉构建待后续版本按需） |
| CI 实跑（U1） | 实现 | 首推 run `39448714961` 全绿（layout / determinism / 2×OS × 2×Go 矩阵） |
| 跨平台 CI 矩阵（U2） | 实现 | ubuntu-latest + windows-latest × go1.22/1.24 四组合全绿 |
| 真实发行版产物覆盖（U5/R4） | 不适用 | 边界承诺：仅合成样张，不承诺真实发行版 |
