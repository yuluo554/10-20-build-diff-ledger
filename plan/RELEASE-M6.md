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

## 四、v0.1.1 发布记录（2026-10-23 二次迭代）

### 4.1 为什么发 0.1.1 而不是重发 0.1.0

`v0.1.0` 有四处已发布面缺陷，全部实测确认（诊断见 `update/00-诊断报告.md`）：

| # | 缺陷 | 实测 |
|---|---|---|
| D1 | `v0.1.0` **自己那棵树的 CI 是 failure** | run `39442868080`：6 job 里 5 红（Test 四腿 + determinism）。`git archive v0.1.0` 导出树用 go1.22 跑 `gen_fixtures --check` → `14 cases, 5 diff`；HEAD 树同版本 → `0 diff`。根因：tag 的样张是 `flate.HuffmanOnly` 时代生成的，deflate 末块编码随 Go 版本漂；修复（手写 RFC 1951 stored 块）在 tag 之后 |
| D2 | Release **正文**的 sha256 与实际附件 digest 不一致 | 正文写 `9dcb6ab6…`；附件 API digest 与本地文件都是 `05e7a4ef…`；`grep -rn 9dcb6ab6` 全仓 0 命中 |
| D3 | `SHA256SUMS` 带 `dist/` 前缀 | 内容为 `05e7a4ef… *dist/bdiff-windows-amd64.exe`；按标准方式在下载目录里 `sha256sum -c` → `No such file or directory`，exit 1 |
| D4 | 发布件由**失败 commit + dirty 树**构建 | `go version -m` → `v0.1.0-0.20261020145702-0051572deb9a+dirty`，`vcs.modified=true`；`0051572` 正是 CI go test 四腿全红的那个提交 |

处置（用户决策 DEC1 = A）：**升 0.1.1 发新版，`v0.1.0` 的 tag 与 Release 原样保留、不下架**，
本文 §一/§二 保持原样作为历史，缺陷与替代关系记录在本节。
**任何读到 `05e7a4ef…` 的地方都应改用下表的 `0.1.1` 值。**

### 4.2 0.1.1 事实

| 项 | 值 |
|---|---|
| 版本号 | `0.1.1`（`internal/version.Version`；`bdiff version` 与 14 份 `report.json.tool_version` / `data/golden/report/*.json` 三处一致） |
| schema | `report.json` schema = 1（未变） |
| 构建命令 | `CGO_ENABLED=0 go build -trimpath -buildvcs=false -o dist/bdiff-windows-amd64.exe ./cmd/bdiff` |
| Go 版本 | `go1.27.0 windows/amd64`（另用 `go1.22.12` 跑了 vet / test / `--check`） |
| 源 commit | 见 §4.3（tag `v0.1.1` 指向的提交；`-buildvcs=false` 故 buildinfo 无 `vcs.revision`，按 DEC2-A 改由本表显式记录） |
| 产物 | `dist/bdiff-windows-amd64.exe`，5,711,360 B |
| 产物 sha256 | `1cf09b1686261fc9fe14f86b02f693c39d9497e44f9d1b4a184961e9e382cee1` |
| `SHA256SUMS` | 90 B，单行 `1cf09b16… *bdiff-windows-amd64.exe`（**无 `dist/` 前缀**） |
| 可复现 | 同一命令连打两次 → `cmp` 逐字节相同（两次 sha256 均为 `1cf09b16…`），与 `dist/` 里的发布件同值 |

**发布承诺边界（DEC3 = A）**：只承诺**「回下载件 == 发布件」**（`sha256sum -c SHA256SUMS`）。
**不承诺**"任意机器重建出同一字节"——跨机/跨 Go 版本未实测，不写进承诺。

### 4.3 发布后回填

| 项 | 实测值（2026-10-23） |
|---|---|
| tag `v0.1.1` | annotated 对象 `49eb970`，指向 commit `ce0d82a`（`git rev-parse v0.1.1^{}`） |
| Release | https://github.com/yuluo554/10-20-build-diff-ledger/releases/tag/v0.1.1 —— 非 draft、非 prerelease，`published_at` 2026-10-23T04:11:31Z |
| 附件 1 | `bdiff-windows-amd64.exe`，5,711,360 B，API digest `sha256:1cf09b16…e382cee1` |
| 附件 2 | `SHA256SUMS`，90 B，API digest `sha256:5b5bc42224c174ca28c3daff8db3eedfa4e5092d87f8d9de4abc2739920be081` |
| 三方 sha256 | 本地 `dist/` 文件 / `go build` 重出件 / GitHub 附件 digest / 回下载件的 sha256 **四个值逐字符相同**（`1cf09b16…`） |
| 回下载校验 | 在空目录 `gh release download v0.1.1` 后直接 `sha256sum -c SHA256SUMS` → `bdiff-windows-amd64.exe: OK`，exit 0（D3 已修） |
| 回下载件可跑 | `./bdiff-windows-amd64.exe version` → `bdiff 0.1.1` exit 0；`selftest` → 全部通过 exit 0；与本地 `dist/` 件 `cmp` 逐字节相同 |
| CI（push main） | run `39898533419`（sha `ce0d82a`）6 job success / 66 step success / 0 failure |
| CI（tag `v0.1.1`） | run `39898759354`（sha `ce0d82a`）同样 6 job success / 66 step / 0 failure |
| `v0.1.0` | tag 与 Release **均未改动**（Release `published_at` 仍 2026-10-20T14:50:38Z，附件仍是 `05e7a4ef…`） |
| GitHub topics | `["build-reproducibility","ci","diff"]`（去掉误带的 `python`） |
| CI 残留注解 | 仅 `Node.js 20 is deprecated`（`actions/checkout@v4` / `actions/setup-go@v5`）；本轮未升级 actions，据实登记、不算已修 |

**执行记录**：`update/03-执行记录.md`（仓库外，不入仓；本仓 `.gitignore` 已排除 `update/`）。

