# HANDOFF-M6 · 发布（干净环境验证 + 脱敏 + tag/Release + 留档）

> 交接单（第 M5 棒收尾 → 第 M6 棒开工）。**新对话第一件事：`node tools/check-sprint-layout.mjs`。**
> 上一棒：M5（report + 门禁），2026-10-20 收尾。

## 一、当前进度 + 证据

| 项 | 状态 | 证据（可指认） |
|---|---|---|
| `compare` 接线 | ✅ | `internal/cli/cli.go` 的 `buildReport`：`artifact.LoadDocument` → `facts.Extract` → `attribute.Attribute` → `attribute.Entries` → `minimize.Search` → `align.Align` → `report.Build` |
| 报告唯一构造入口 | ✅ | `report.Build(Input)`：补零 counts、区分 `null`/`[]`、拒绝"无解但 residue==0"；`cli` 不逐字段赋值 |
| `report.json` 键序 | ✅ | `report.Report` struct 字段序 = §4.8；`TestReportKeyOrder` + `TestReportCountsKeyOrder` |
| `report.md` 模板 | ✅ | `report.RenderMarkdown`：输入 / 分类计数 / 改名·新增·删除 / 残留 / 最小原因集 / 判定；`TestMarkdownTemplateSections` |
| `--out DIR` 落盘 | ✅ | `writeReportDir` 写 `report.json` + `report.md`（目录自建、0644、`\n`）；`TestReportOutDir` |
| 写盘错误 → exit 1 | ✅ | `TestReportOutDirWriteError`（拿文件当目录，断言 stderr 含"写盘失败"） |
| `--format=json\|md\|both` | ✅ | `both` = report.json + `\n` + report.md（`compareBoth` 按固定分界切） |
| `--fail-on=any\|residue` | ✅ | `parseFailOn`；只把 2 降为 0，3 不降级（`TestCompareFailOnResidue`） |
| `report` golden | ✅ **14 × 2 份** | `data/golden/report/<case>.json` + `.md`；`TestReportGolden` 逐字节 + 双跑 |
| `residual_items` 与 residue 同源 | ✅ | `attribute.Residuals` 是唯一实现，`residueBytes` = 其 `BytesDelta` 之和；`TestReportShape` 断言和相等 |
| 退出码矩阵 4 行 | ✅ | `TestExitCodeMatrix`：0 = 自比 / 1 = `broken/truncated.tar` / 2 = `c01` / 3 = `c11`，且断言四码全被覆盖 |
| `--budget` 三形态 | ✅ | `TestCompareBudgetThreeFormsE2E`（单值 / 逗号多值 / `all`，含"不降级残留"）+ `TestBudgetCoversActualCategories` |
| `ci/example.yml` 本地等价 | ✅ | `bash tools/check-ci-example.sh`：0/1/2/3 四分支 + "放行 ≠ 消失" + 落盘断言，全 PASS |
| README 三场景可执行 | ✅ | `TestReadmeCommandsParse` 抽 README 的 fenced bash 块、**真执行** 6 条 `./bdiff` 命令、逐条比对 `# 期望 exit=N` |
| 布局门 | ✅ 全绿 | `node tools/check-sprint-layout.mjs` |
| 布局门变异 | ✅ 11/11 | `node tools/verify-layout-mutations.mjs`（V4 已改打 M6、V10 改打 M6 DoD 的脱敏行） |
| Go 守门变异 | ✅ **13/13** | `bash tools/verify-go-guards.sh`（新增 G11 null/[]、G12 budget 看实际类别、G13 残留优先级） |
| 本机测试 | ✅ | `go build ./...` / `go vet ./...` / `go test ./...` 全绿；`gofmt -l .` 为空 |
| 样张重建 | ✅ | `go run ./tools/gen_fixtures --out data/fixtures --check` → `14 cases, 0 diff` |
| CI | 🟡 配置就位，**未实跑** | 仍无远端（U1）；新增 `check-ci-example.sh` 步骤已写进 `ci.yml` |

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **`compare` 的 stdout 已是 `report.json`**（M3–M4 的归因 JSON 不再是它）。要逐差异点请走
   `data/golden/attribute/` 或 `buildAttributeResult`；`TestAttributeGolden` 已改为直接调该函数。
2. **报告字段口径只在 `data/README.md` §4.8**（含 `left/right.sha256` 的文件/目录两种算法、
   `residual_items` 与 `residue_bytes` 的"和恒等"关系、`--out`/`--format` 的分工）。改语义必须同提交改 §4.8 + golden。
3. **`report.Build` 是唯一构造入口**：`cli` 不得逐字段赋值。它承担三条纪律：counts 补零、
   `null`（无解）vs `[]`（完全一致）、拒绝"无解但 residue==0"的自相矛盾。
4. **`null` 与 `[]` 不可混**：`Input.MinimalCauseSetFound` 是唯一开关；`found=false` → `nil` → JSON `null`。
   变异 G11 证明 `TestBuildDistinguishesNullAndEmpty` 会因此变红。
5. **退出码判定唯一实现在 `internal/cli/exitcode.go`**，且**只读 `report.Report`**（不再读 `attribute.Result`）——
   否则"报告写的"与"退出码依据的"会分叉。顺序固定：`residue>0 || T12>0 → 3` → `S==∅ → 0` → budget → 2。
6. **budget 覆盖的是实际类别（counts>0），不是最小集**。14 组真实样张的最小集恰好都覆盖了实际类别，
   所以拿样张验这条会恒绿——判据必须打在 `TestBudgetCoversActualCategories` 上（变异 G12 即此靶）。
7. **`--fail-on=residue` 只降 2 → 0**，永不降 3；与 `--budget` 同一条纪律（§4.6）。
8. **`report.md` 必须无时间性文案/绝对路径/主机名、只用 `\n`**：`TestReportMarkdownNoLeak` + `TestMarkdownTemplateSections`。
9. **`report.MarshalCanonical` 是 `report.json` 唯一编码器**（内部即 `MarshalValue`）；`align`/`facts` 也走它。
10. **`residual_items` 的 `rule_id` 恒为 `T12`**：残留按定义就是 T1–T11 解释不掉的部分。
11. **`attribute.Residuals` 与 `residue_bytes` 是同一实现**：不要另写一份"逐条目残留"（K30 同族）。
12. **目录输入的 `left/right.sha256` 含目录条目**（`kind=dir` 取 §4.9 空内容常量），
    不含路径前缀、不含时钟。手工复算脚本在 `data/golden/report/README.md`（已实测对上 `d9d230…`）。
13. **`TestReadmeCommandsParse` 是真执行**：README 每个 fenced `bash` 块里 `./bdiff` 命令数
    必须与 `# 期望 exit=N` 标注数 1:1，否则测试 fatal。改 README 的命令就要同步改标注。
14. **变异 marker 随状态翻转**：V4 现打 M6（plan/05 §一），V10 现打 M6 DoD 的脱敏行。
    **M6 收尾时**：V4 无处可打（M6 是最后一个里程碑）→ 改为"在 plan/00 把 M6 翻成 ✅ 而其它副本仍 ⬜"，
    或新增一个哨兵里程碑；V10 改打 `plan/RELEASE-M6.md` 里的一行（届时该文件才存在）。
15. **状态三副本**（`plan/00` §六 / `plan/05` §一 / `README.md`）同提交翻转。K8。
16. **`plan/06` 缺口列只留 G4（CI 未实跑）/ G5（Release 未建）**，且不许点名已 ✅ 的里程碑。K9。
17. **CI 新增一步 `bash tools/check-ci-example.sh`**（ubuntu + go1.24 才跑）：它自己构建二进制，
    不依赖前面的步骤产物。
18. **M6 起布局门要求 `plan/RELEASE-M6.md`**（`LAYOUT.RELEASE`：两栏 + 阶段 7 覆盖对账表，≥3 行三态）。
    M6 收尾前它不存在也没关系；**一旦 M6 翻 ✅ 就必须有**。

## 三、下一棒待办（M6）

按 `plan/05` §二 M6 DoD：

1. [ ] 干净环境验证：`git archive HEAD | tar -x -C <干净目录>`（无 `.git`），按 README 逐条跑
       （`go build` / `version` / `selftest` / 三场景 / `gen_fixtures --check` / 两个变异脚本）。
2. [ ] 脱敏四步 + 产物扫描：绝对路径 / 主机名 / 用户名 / 邮箱（源码、文档、golden、`report.md` 全扫）。
3. [ ] `plan/RELEASE-M6.md` **tag 前落盘**：§一 tag 前事实（版本号、样张数、测试计数、sha256 待填）/
       §二 tag 后追加 / §三 阶段 7 覆盖对账表（≥3 行 `实现|不适用|待首推` 三态）。
4. [ ] 与用户确认后**建公开仓库 + 首推**（外发动作，`plan/06` §二 的决策：不代办）；据实补记 run id。
5. [ ] tag（`v0.1.0`）+ Release 附产物（至少 `dist/bdiff-windows-amd64.exe`，本地留副本 + sha256）。
6. [ ] `version.Version` 定版（`0.1.0-dev` → `0.1.0`）并同步 README 顶部与 `plan/00` 状态行；
       改完重跑 `TestReportGolden -update`（`tool_version` 进 golden）。
7. [ ] 回写三处状态（M6 → ✅）+ 写 `plan/HANDOFF-M7.md`（若还有下一棒）或收尾说明。
8. [ ] 变异脚本 marker 翻新（见 §二.14）。

**M6 的已知难点**：

- **`tool_version` 在 golden 里**：定版会改 14 份 `report.json` 的 `tool_version`。必须 `-update` 后人工核对
  "只有这一处变"（`git diff data/golden/report/`）。
- **干净环境没有 Go 缓存**：`go build` 首次会稍慢；本项目零依赖，不需要 GOPROXY（`plan/00` §八）。
- **tag 之后不得改交付面代码**（AGENTS.md 禁忌）：定版与 golden 重生成必须在 tag **之前**。
- **脱敏**：`report.md` 只写 basename（已由 `TestReportMarkdownNoLeak` 钉住），但 `plan/` 与
  `data/golden/*/README.md` 里有 `/tmp/...` 之类的**示例命令**，扫描时要区分"示例路径"与"真实泄漏"。

## 四、既定口径清单（K 编号 —— 动了会打挂测试）

| K | 口径 | 打挂什么 |
|---|---|---|
| K1 | 规则全集 = `model.RuleIDs`（12 条，T12 末尾） | `guard.TestRuleSetClosed`、`LAYOUT.CONTRACT` |
| K2 | 退出码 0/1/2/3 语义固定（2 不得被用法错误占用） | `cli.TestExitCodeContract`、`cli.TestCompareUsageErrors` |
| K3 | 受限 import 黑名单（含 `time`），`cmd/`+`internal/`+`tools/` 三处都扫 | `guard.TestNoForbiddenImports`、`LAYOUT.NO_NET` |
| K4 | 依赖方向 = `LayerOrder`（`minimize` 只可 import `model`+`normalize`；`report` 只 `model`+`version`） | `guard.TestDependencyDirection` |
| K5 | `tree_digest` 唯一实现 = `normalize.TreeDigest`，分隔符 `0x00`/`0x0A` | `normalize.TestTreeDigest*`、变异 G5 |
| K6 | `report.json` 键序 = `Report` struct 字段序 | `report.TestReportKeyOrder`、变异 G4 |
| K7 | `--budget` 三形态：单值 / 逗号多值 / `all`；覆盖的是**实际类别** | `report.TestParseBudgetThreeForms`、`cli.TestCompareBudgetThreeFormsE2E`、`cli.TestBudgetCoversActualCategories`、变异 G6/G12 |
| K8 | 状态三副本同提交翻转 | `LAYOUT.STATUS`、变异 V4 |
| K9 | 缺口列不点名已 ✅ 里程碑 | `LAYOUT.GAPS`、变异 V9 |
| K10 | 改名仅同 sha256 且**该哈希两侧各出现 1 次** | `TestRenameOnlyOnEqualHash`、`data/golden/align/dup-hash.json`、变异 G7 |
| K11 | 部分生成（`--only`）不写全集清单 | `TestOnlyWritesNoManifest`、变异 G8 |
| K12 | 报告不写绝对路径/主机名/用户名；错误只报 basename | `cli.TestReportMarkdownNoLeak`、`cli.TestUsageErrorIsNotMetaOnly`、`artifact.TestErrorDoesNotLeakAbsolutePath` |
| K13 | 目录条目 sha256 = `align.EmptySHA256`（§4.9 常量） | `align.TestEmptyDigestConstant` |
| K14 | 样张归档字节 = 生成器手写输出 | `gen_fixtures --check`、`TestCheckIsFalsifiable` |
| K15 | 变异脚本 marker 必须匹配当前源码空白/文本 | 两个变异脚本的"marker 不存在"报错 |
| K16 | facts key 集合 = `facts.Keys()`，每 key 必须有单测 | `facts.TestEveryKeyHasTest`、`facts.TestNoOrphanKeyTests` |
| K17 | `raw_header_sha256` 的字节构造 = `data/README.md` §4.3 | `facts.TestRawHeaderUnknownPAXIsHandRecomputed`、`data/golden/facts/*` |
| K18 | 容器级事实 `path=""` | `data/golden/facts/c06-*`、`c07-*` |
| K19 | facts 取值格式（十进制/hex/`hex:`/`zip.mtime=YYYYMMDDHHMMSS`/节 key 双点）= §4.10 | 全部 facts golden |
| K20 | 样张数 = `caseDefs()` 长度（当前 14） | `TestCaseMatrixIs14AndCoversT1ToT12` |
| K21 | facts 的未知头部不得静默 | `TestRawHeaderUnknownPAXIsHandRecomputed`、`cli.TestFactsRejectsUnreadableInput` |
| K22 | `entry_stream` 编码唯一实现 = `normalize.EntryStream` | `TestEntryStreamHasIndexAndFacts`、`TestTreeDigestKnownAnswer` |
| K23 | op 施加顺序 = 数值序（`T1<T2<…<T11`），不是字符串序 | `TestExpectedCauseSetEqualizesDigests` |
| K24 | `tar.size` = 派生字段 | `TestFixtureMatrix/c13-signature` |
| K25 | ELF 节 offset/size 值差异 = 派生；分类只看存在性 | `TestFixtureMatrix/c10-toolchain-path` |
| K26 | T4 前缀按侧计算、存 `RootPrefix`、op 剥离 | `TestT4StripsPrefixBeforeContent`、`TestFixtureMatrix/c04-prefix` |
| K27 | 内容字节差异只由"内容型"ELF 事实解释 | `TestFixtureMatrix/c12-mixed` |
| K28 | `attribute` 不 import `artifact`；`SideInput` 由 cli 构造 | `guard.TestDependencyDirection` |
| K29 | M3/M4 的 `compare` 只输出归因 JSON → **M5 已翻转**：stdout = report.json；归因 JSON 仅作 golden | `TestAttributeGolden`（改为直调 `buildAttributeResult`）、`TestReportShape` |
| K30 | golden 逐字节 + 双跑一致；改行为必须 `-update` 并人工核对 | `TestAttributeGolden`、`TestMinimizeGolden`、`TestReportGolden` |
| K31 | `Search` 返回口径：空集解 = 非 nil 空切片；无解 = `nil,false,nil`；nil 摘要 = error | `TestSearchIdenticalReturnsEmptySet`、`TestSearchNoSolutionIsExplicit`、`TestSearchNilDigestIsExplicit` |
| K32 | 枚举顺序只由 `Combinations()`；不剪枝 | `TestCombinationsOrder`、`TestSearchIsExhaustiveWithoutPruning` |
| K33 | minimize golden 的 `digest_set`：有解 = S，无解 = T1–T11 | `TestMinimizeGolden` |
| **K34** | 报告构造唯一入口 = `report.Build`；`null`（无解）≠ `[]`（完全一致）；无解必须 `residue>0` | `TestBuildDistinguishesNullAndEmpty`、`TestBuildRejectsContradiction`、变异 G11 |
| **K35** | 退出码判定唯一实现 = `cli/exitcode.go`，只读 `report.Report`；顺序 `3 → 0 → budget → 2` | `TestExitCodeMatrix`、变异 G13 |
| **K36** | `--fail-on=residue` 只降 2 → 0，不降 3 | `TestCompareFailOnResidue` |
| **K37** | `residual_items` 与 `residue_bytes` 同源（`attribute.Residuals`），和恒等 | `TestReportShape` |
| **K38** | `left/right.sha256`：文件 = 文件字节；目录 = 条目表拼接（含目录条目） | `data/golden/report/README.md` 的手工复算 + `TestReportShape`（只断言形态） |
| **K39** | README 每条 `./bdiff` 命令带 `# 期望 exit=N`，且被真执行 | `TestReadmeCommandsParse` |

## 五、本机环境坑（只写本棒实测过的）

| 坑 | 实测 | 处置 |
|---|---|---|
| `go run` 把非零退出码折成 1 | `go run ./cmd/bdiff compare ...` 在 exit 2 时 shell 看到 `exit status 2` 且 `$? = 1` | 要断言退出码就**先 build 再跑二进制**（`tools/check-ci-example.sh` 就是这么做的） |
| 变异靶子选在"恒绿"的测试上 | G12 首版打 `TestCompareBudgetThreeFormsE2E`，变异后仍通过——14 组样张的最小集恰好覆盖了全部实际类别 | 改打 `TestBudgetCoversActualCategories`（构造 counts ⊋ 最小集的构型）；这条已写进脚本注释 |
| 目录输入的 sha256 漏掉目录条目 | README 首版手工复算脚本只 `find -type f`，算出 `eb0489…`，与实现（`d9d230…`）不符 | 复算脚本改用 `find -mindepth 1` 并给目录填 §4.9 的空内容常量 |
| `--out` 在 Windows 上指向已存在的文件 | `MkdirAll` 报错，但错误串里带绝对路径 | 统一改写成 `"无法创建输出目录 %q：写盘失败"`（只报 basename），并由 `TestReportOutDirWriteError` 钉住 |
| `go` 不在 PATH | `go: command not found` | `export PATH="$(cygpath -u "$LOCALAPPDATA")/Programs/Go/bin:$PATH"` |
| Git Bash 的 `/tmp` 与 Windows exe 的 `/tmp` 不是同一处 | 二进制写到 `C:\tmp`，bash 读 `/tmp/out.json` 找不到 | 临时文件放仓库内相对目录（如 `.tmpout/`、`.tmp-ci-example/`），收尾删除 |
| `python` 是商店占位符 | 读 stdin 的管道立刻关闭 | 一律 `py -3.12` |

## 六、DoD 逐项 checklist（M6 出口时逐条打勾）

- [ ] 干净环境（`git archive` 解包树、无 `.git`）逐条跑通 README 命令
- [ ] 脱敏扫描零命中（绝对路径 / 主机名 / 用户名 / 邮箱）
- [ ] `plan/RELEASE-M6.md` 三节齐（tag 前事实 / tag 后追加 / 覆盖对账表 ≥3 行）
- [ ] 公开仓库 + 首推 CI 绿（据实补记 run id）
- [ ] tag `v0.1.0` + Release 附产物（≥1 手工上传附件，本地副本 + sha256 留档）
- [ ] `version.Version` 定版且 `data/golden/report/` 已 `-update` 并人工核对"只有 tool_version 变"
- [ ] `go test ./...` 全绿、`go vet ./...` 干净、`gofmt -l .` 为空
- [ ] `go run ./tools/gen_fixtures --out data/fixtures --check` → `14 cases, 0 diff`
- [ ] `node tools/check-sprint-layout.mjs` 全绿
- [ ] `node tools/verify-layout-mutations.mjs` 11/11
- [ ] `bash tools/verify-go-guards.sh` 13/13
- [ ] `bash tools/check-ci-example.sh` 全 PASS
- [ ] 三处状态副本 M6 → ✅ 且互相一致
- [ ] `plan/06` 缺口表只剩未关闭项

## 七、关键命令速查

```bash
# 环境
export PATH="$(cygpath -u "$LOCALAPPDATA")/Programs/Go/bin:$PATH"

# 门（每次开工第一条）
node tools/check-sprint-layout.mjs
node tools/verify-layout-mutations.mjs
bash tools/verify-go-guards.sh
bash tools/check-ci-example.sh

# Go
go build ./... && go vet ./... && go test ./...
gofmt -l .

# 样张
go run ./tools/gen_fixtures --out data/fixtures --check    # 必须 "14 cases, 0 diff"

# 报告（M5 已可用）
go test ./internal/cli -run 'TestReportGolden|TestReportShape|TestReportOutDir|TestExitCodeMatrix' -count=1
go test ./internal/cli -run TestReportGolden -update       # 改行为后重生成（人工核对 diff）
go test ./internal/report -count=1

# 真跑一条 compare（注意 go run 会把 exit 2 折成 1）
CGO_ENABLED=0 go build -trimpath -o .tmp-bdiff/bdiff ./cmd/bdiff
.tmp-bdiff/bdiff compare \
  --left  data/fixtures/c01-mtime/pair/left/synthpkg-0.1.0.tar \
  --right data/fixtures/c01-mtime/pair/right/synthpkg-0.1.0.tar \
  --format md
# → exit 2（仅元数据），minimal_cause_set = [T1]，residue = 0
```
