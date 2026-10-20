# HANDOFF-M5 · report（固定键序报告 + 退出码矩阵 + `--budget` + `ci/example.yml`）

> 交接单（第 M4 棒收尾 → 第 M5 棒开工）。**新对话第一件事：`node tools/check-sprint-layout.mjs`。**
> 上一棒：M4（minimize），2026-10-20 收尾。

## 一、当前进度 + 证据

| 项 | 状态 | 证据（可指认） |
|---|---|---|
| `minimize.Search` | ✅ | `internal/minimize/minimize.go`：消费 `Combinations()`（基数升序、同基数字典序），逐个调两侧摘要，**首个命中即停**；不剪枝/不随机/不启发 |
| 返回口径 | ✅ | `(set, true, nil)` 找到（空集 = 完全一致，返回**非 nil** 空切片）；`(nil, false, nil)` 无解；nil 摘要函数 → `ErrNilDigest` |
| 最小性单测 | ✅ | `TestSearchFindsMinimalPairAndNoSuperset`（len==2）、`TestSearchPrefersSmallerCardinality`、`TestSearchLexicographicTieBreak`、`TestSearchNeverReturnsT12` |
| 不剪枝单测 | ✅ | `TestSearchIsExhaustiveWithoutPruning`：无解时每侧**恰好 4096 次**摘要调用、每个候选 1 次 |
| 14 组样张搜索 | ✅ 14/14 | `TestMinimizeFixtureMatrix`（`expect.json` 的 `minimal_cause_set` 驱动：null 与 `[]` 区分） |
| DoD 四条 | ✅ | `TestMinimizeDoDCases`：`c01→[T1]`；`c09→[T1,T2]` 且 `len==2`；`c11→null` 且 residue>0；`c12→null` 且残留只有 `table.csv` 1 字节 |
| `data/golden/minimize/` | ✅ **14 份 + README** | 逐字节；字段 = `minimal_cause_set/found/residue_bytes/digest_set/digest_left/digest_right`；README 含生成、双跑、手工 `cmp` 复算 |
| 搜索耗时实测 | ✅ | `BenchmarkMinimizeSearch`：`c01` 0.67 ms、`c12` 0.65 s（4096 候选 × 两侧）；机器与命令回填 `plan/00` §八 |
| 布局门 | ✅ 全绿 | `node tools/check-sprint-layout.mjs`（V4 已改打 M5、V10 已改打 `HANDOFF-M6`） |
| 布局门变异 | ✅ **11/11** | `node tools/verify-layout-mutations.mjs` |
| Go 守门变异 | ✅ **10/10** | 新增 G10（枚举改基数降序 → `TestMinimizeDoDCases` 红） |
| 本机测试 | ✅ | `go build ./...` / `go vet ./...` / `go test ./...` 全绿；`gofmt -l .` 为空 |
| 样张重建 | ✅ | `go run ./tools/gen_fixtures --out data/fixtures --check` → `14 cases, 0 diff` |
| CI | 🟡 配置就位，**未实跑** | 仍无远端（U1） |

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **`data/README.md` §4.5 是唯一口径**：`entry_stream` 编码、`tree_digest` 拼接、最小集定义与枚举顺序、`|S|≤12`、无解 ⇔ `residue_bytes > 0`。改语义必须同提交改这些节 + 两处 golden。
2. **`Search` 的签名与返回合同**：`Search(leftDigest, rightDigest func([]string) string) ([]string, bool, error)`。空集解返回**非 nil 空切片**（JSON `[]`），无解返回 `nil,false,nil`（JSON `null`）；nil 函数返回 `ErrNilDigest`。不要把"无解"写成 error，也不要把空集写成 nil。
3. **枚举顺序只由 `Combinations()` 决定**：先基数升序、同基数按 `model.RuleIDs` 数值序。`Search` 不重排、不跳过。用 `sort.Strings` 会把 `T10` 排到 `T2` 前（K23 同族）。
4. **不剪枝可被机器证伪**：`TestSearchIsExhaustiveWithoutPruning` 断言无解时每侧 4096 次调用且无重复。任何"先试常见规则/提前退出"都会让它红。
5. **T12 不可能进最小集**：T12 无 op，若 `S∪{T12}` 成立则 `S` 更小且成立；首个命中不会带 T12（`TestSearchNeverReturnsT12`）。
6. **候选循环里不要重新 `facts.Extract`**：两侧 `[]normalize.Entry` 只构造一次（`attribute.Entries`），摘要闭包复用；缓存键若需要 = `(side, S)`，且不得另写 `entry_stream` 编码。
7. **`c09` 的 `[T1,T2]` 依赖分类精确**：若 T1/T2 的差异被误记到别的类（或 op 不生效），搜索会给出超集/空集。`c05` 的 `[T5]` 依赖 `index` 进 `entry_stream`——"优化掉 index"会让最小集退化成 `∅`。
8. **`c11`/`c12` 无解必须与 `residue_bytes > 0` 同真**：搜索返回 `nil,false,nil` 后，调用方必须仍报告残留（§4.5）。`c12` 的残留只来自 `synthpkg/data/table.csv` 的 1 字节（T1/T3/T9 已抹平元数据）。
9. **minimize golden 逐字节**：`go test ./internal/cli -run TestMinimizeGolden -update` 重新生成；`digest_set` 有解时 = `minimal_cause_set`，无解时 = T1–T11（后者让"无解"有摘要证据）。
10. **M4 没有改 `compare` 的输出**：当前仍是 attribute 归因 JSON；`--out`/`--format=md|both` 仍 exit 1（K29）。M5 才把 `minimal_cause_set` 接进 `report.Report`（该 struct 字段已在位，键序 §4.8）。
11. **耗时数字只进文档**：`plan/00` §八的 0.67 ms / 0.65 s 是实测值，不得写进断言或阈值（`plan/06` §五）。
12. **变异 marker 已随 M4 翻转**：V4 打 M5、V10 打 M5 DoD 的 `HANDOFF-M6`、G10 打 `Combinations` 的基数循环。**M5 收尾时**：V4 改打 M6，V10 改打 M6 DoD 的 `- [ ] plan/HANDOFF-M6.md`。
13. **状态三副本**（`plan/00` §六 / `plan/05` §一 / `README.md`）同提交翻转。K8。
14. **`plan/06` 缺口列不许出现已 ✅ 的里程碑编号**（M0–M4 都照判）。K9。
15. **`report.Report` 已存在**：`MinimalCauseSet []string` 字段在 `report.go`，`MarshalCanonical`/`MarshalValue` 是唯一 JSON 编码器，`RenderMarkdown` 有确定性骨架。M5 只填值、补 `--out`、补 golden，不要另写编码器。
16. **退出码唯一出口仍是 `cli.compareExitCode`**：0/2/3 已按 §4.6；`--budget` 只影响 0/2，不降级 3（`TestCompareBudgetAllowsMetadataOnly`）。M5 接入报告时不得改变这条。

## 三、下一棒待办（M5）

按 `plan/05` §二 M5 DoD 与 `plan/03` §六：

1. [ ] `compare` 接线：`attribute.Result` + `minimize.Search` → `report.Report`（字段序 = §4.8）；`minimal_cause_set` 填 `[]`/`null` 语义。
2. [ ] `--out DIR` 落 `report.json` + `report.md`；`--format=json|md|both` 三形态；写盘错误 → exit 1（不得静默）。
3. [ ] `report.json` / `report.md` 双跑逐字节 + `data/golden/report/`（README 写生成与手工复算）。
4. [ ] 退出码矩阵 4 行全覆盖：0/1/2/3 各一条样张（0 用 `c00` 等价物或同文件自比；1 用 broken 输入/用法错误）。
5. [ ] `--budget` 三形态：单值 / 逗号多值 / `all`（`ParseBudget` 已实现，补 CLI 端到端）。
6. [ ] `ci/example.yml` 本地跑通（不联网的部分）。
7. [ ] README 三场景命令全部可执行 + `TestReadmeCommandsParse`（常驻测试）。
8. [ ] 回写三处状态（M5 → ✅）+ 缺口表（G1/G3 关闭）+ 写 `plan/HANDOFF-M6.md`（七节）。
9. [ ] 变异脚本：V4 → M6；V10 → `plan/HANDOFF-M6.md`；新判据（README 命令可解析 / 报告键序）按需新增变异。

**M5 的已知难点**：

- `report.Report.Counts` 是 `report.Counts`（手写 MarshalJSON），attribute 的 `attribute.Counts` 是另一型；转换时保持 T1…T12 + total 键序，别直接 map 序列化。
- `report.md` 必须无时间性文案、无绝对路径、无主机名（K12）；BOM/换行用 `\n`。
- 退出码 0 的"完全一致"要区分 `S=∅` 与无解：先看 `residue>0 || T12>0` → 3，再看 `S==∅` → 0，最后 2（§4.6）。
- `--budget` 白名单覆盖的是**实际类别**（counts>0 的 Tn），不是最小集。

## 四、既定口径清单（K 编号 —— 动了会打挂测试）

| K | 口径 | 打挂什么 |
|---|---|---|
| K1 | 规则全集 = `model.RuleIDs`（12 条，T12 末尾） | `guard.TestRuleSetClosed`、`LAYOUT.CONTRACT` |
| K2 | 退出码 0/1/2/3 语义固定（2 不得被用法错误占用） | `cli.TestExitCodeContract`、`cli.TestCompareUsageErrors` |
| K3 | 受限 import 黑名单（含 `time`），`cmd/`+`internal/`+`tools/` 三处都扫 | `guard.TestNoForbiddenImports`、`LAYOUT.NO_NET` |
| K4 | 依赖方向 = `LayerOrder`（`minimize` 只可 import `model`+`normalize`） | `guard.TestDependencyDirection` |
| K5 | `tree_digest` 唯一实现 = `normalize.TreeDigest`，分隔符 `0x00`/`0x0A` | `normalize.TestTreeDigest*`、变异 G5 |
| K6 | `report.json` 键序 = `Report` struct 字段序 | `report.TestReportKeyOrder`、变异 G4 |
| K7 | `--budget` 三形态：单值 / 逗号多值 / `all` | `report.TestParseBudgetThreeForms`、`cli.TestCompareBudgetAllowsMetadataOnly` |
| K8 | 状态三副本同提交翻转 | `LAYOUT.STATUS`、变异 V4 |
| K9 | 缺口列不点名已 ✅ 里程碑 | `LAYOUT.GAPS`、变异 V9 |
| K10 | 改名仅同 sha256 且**该哈希两侧各出现 1 次** | `TestRenameOnlyOnEqualHash`、`data/golden/align/dup-hash.json`、变异 G7 |
| K11 | 部分生成（`--only`）不写全集清单 | `TestOnlyWritesNoManifest`、变异 G8 |
| K12 | 报告不写绝对路径/主机名/用户名；错误只报 basename | `cli.TestUsageErrorIsNotMetaOnly`、`artifact.TestErrorDoesNotLeakAbsolutePath` |
| K13 | 目录条目 sha256 = `align.EmptySHA256`（§4.9 常量） | `align.TestEmptyDigestConstant` |
| K14 | 样张归档字节 = 生成器手写输出 | `gen_fixtures --check`、`TestCheckIsFalsifiable` |
| K15 | 变异脚本 marker 必须匹配当前源码空白/文本 | 两个变异脚本的"marker 不存在"报错 |
| K16 | facts key 集合 = `facts.Keys()`，每 key 必须有单测 | `facts.TestEveryKeyHasTest`、`facts.TestNoOrphanKeyTests` |
| K17 | `raw_header_sha256` 的字节构造 = `data/README.md` §4.3 | `facts.TestRawHeaderUnknownPAXIsHandRecomputed`、`data/golden/facts/*` |
| K18 | 容器级事实 `path=""`（gzip.* / zip.create_system / gzip raw_header） | `data/golden/facts/c06-*`、`c07-*`；attribute 的容器条目 |
| K19 | facts 取值格式（十进制/hex/`hex:`/`zip.mtime=YYYYMMDDHHMMSS`/节 key 双点）= §4.10 | 全部 facts golden |
| K20 | 样张数 = `caseDefs()` 长度（当前 14） | `TestCaseMatrixIs14AndCoversT1ToT12` |
| K21 | facts 的未知头部不得静默：PAX 未知 keyword 进 RawHeader；损坏输入 exit 1 | `TestRawHeaderUnknownPAXIsHandRecomputed`、`cli.TestFactsRejectsUnreadableInput` |
| K22 | `entry_stream` 编码（index ‖ facts ‖ content）唯一实现 = `normalize.EntryStream` | `TestEntryStreamHasIndexAndFacts`、`TestTreeDigestKnownAnswer` |
| K23 | op 施加顺序 = 数值序（`T1<T2<…<T11`），不是字符串序 | `TestExpectedCauseSetEqualizesDigests`（T8/T9/T10/T11 先后） |
| K24 | `tar.size` = 派生字段，Apply 末尾同步、分类忽略 | `TestFixtureMatrix/c13-signature`（T10 不得带出 T12） |
| K25 | ELF 节 offset/size 值差异 = 派生；分类只看存在性 | `TestFixtureMatrix/c10-toolchain-path`（不得出现 T9） |
| K26 | T4 前缀按侧计算、存 `RootPrefix`、op 剥离；S=∅ 摘要仍带前缀 | `TestT4StripsPrefixBeforeContent`、`TestFixtureMatrix/c04-prefix` |
| K27 | 内容字节差异只由"内容型"ELF 事实解释（tar/zip 头事实不算） | `TestFixtureMatrix/c12-mixed`（T12 仍出现） |
| K28 | `attribute` 不 import `artifact`；`SideInput` 由 cli 构造 | `guard.TestDependencyDirection` |
| K29 | M3/M4 的 `compare` 只输出归因 JSON；`--out`/`md` 明确 exit 1 | `TestCompareUsageErrors`（M5 接线时同步改这条与 plan/03 §一） |
| K30 | golden 逐字节 + 双跑一致；改行为必须 `-update` 并人工核对 | `TestAttributeGolden`、`TestMinimizeGolden` |
| **K31** | `Search` 返回口径：空集解 = 非 nil 空切片；无解 = `nil,false,nil`；nil 摘要 = error | `TestSearchIdenticalReturnsEmptySet`、`TestSearchNoSolutionIsExplicit`、`TestSearchNilDigestIsExplicit` |
| **K32** | 枚举顺序只由 `Combinations()`（基数升序、同基数字典序）；不剪枝 | `TestCombinationsOrder`、`TestSearchIsExhaustiveWithoutPruning` |
| **K33** | minimize golden 的 `digest_set`：有解 = S，无解 = T1–T11 | `TestMinimizeGolden`、`data/golden/minimize/README.md` |

## 五、本机环境坑（只写本棒实测过的）

| 坑 | 实测 | 处置 |
|---|---|---|
| 测试里两侧共用同一个摘要闭包 | `Search(func(s){...}, func(s){...})` 若两闭包语义相同，未满足条件时也是 `"x"=="x"` → 空集恒命中，四条最小性测试全红 | 用 `digestPair(sat)`：未满足时分别返回 `"left"`/`"right"` |
| 想测耗时却不能用 `time` | `internal/`/`tools/` 禁止 import `time`（K3），写计时脚本会打挂 `guard.TestNoForbiddenImports` | 用 `testing.B`：`go test ./internal/cli -run '^$' -bench BenchmarkMinimizeSearch -benchtime=5x`；`time` 由测试框架持有，不进被测包 |
| 无解样张的搜索代价 | `c12` 一次搜索 ≈ 0.65 s（4096 候选 × 两侧 Apply，ELF 节重写是大头） | 不要在每个候选里重新 `facts.Extract`；14 组全跑约 6 s（`go test ./internal/cli`） |
| 变异 marker 随状态翻转失效 | V4 打 M4（已 ✅）、V10 打 `HANDOFF-M5`（已 `[x]`） | 已改打 M5 / `HANDOFF-M6`；新增 G10 |
| `go` 不在 PATH | `go: command not found` | `export PATH="$(cygpath -u "$LOCALAPPDATA")/Programs/Go/bin:$PATH"` |
| Git Bash 的 `/tmp` 与 Windows exe 的 `/tmp` 不是同一处 | 二进制写到 `C:\tmp`，bash 读 `/tmp/out.json` 找不到 | 临时文件放仓库内相对目录（如 `.tmpout/`），收尾删除 |
| `python` 是商店占位符 | 读 stdin 的管道立刻关闭 | 一律 `py -3.12` |

## 六、DoD 逐项 checklist（M5 出口时逐条打勾）

- [ ] `go test ./...` 全绿（含 report golden / 退出码矩阵 / README 命令解析）
- [ ] `go vet ./...` 干净、`gofmt -l .` 为空
- [ ] `go run ./tools/gen_fixtures --out data/fixtures --check` → `14 cases, 0 diff`
- [ ] `report.json` / `report.md` 双跑逐字节 + `data/golden/report/`
- [ ] 退出码 0/1/2/3 各一条样张
- [ ] `--budget` 单值 / 逗号多值 / `all` 三形态
- [ ] `ci/example.yml` 本地跑通；README 三场景命令可执行
- [ ] `node tools/check-sprint-layout.mjs` 全绿
- [ ] `node tools/verify-layout-mutations.mjs` 11/11（V4 已打 M5，**M5 ✅ 后要改打 M6**）
- [ ] `bash tools/verify-go-guards.sh` 10/10（新增判据 → 同步新增变异）
- [ ] 三处状态副本 M5 → ✅ 且互相一致
- [ ] `plan/06` 缺口表更新（G1/G3 关闭，不点名已 ✅ 的里程碑）
- [ ] `plan/HANDOFF-M6.md` 七节齐

## 七、关键命令速查

```bash
# 环境
export PATH="$(cygpath -u "$LOCALAPPDATA")/Programs/Go/bin:$PATH"

# 门（每次开工第一条）
node tools/check-sprint-layout.mjs
node tools/verify-layout-mutations.mjs
bash tools/verify-go-guards.sh

# Go
go build ./... && go vet ./... && go test ./...
gofmt -l .

# 样张
go run ./tools/gen_fixtures --out data/fixtures --check    # 必须 "14 cases, 0 diff"

# 最小原因集（M4 已可用）
go test ./internal/cli -run 'TestMinimizeFixtureMatrix|TestMinimizeDoDCases' -count=1
go test ./internal/cli -run TestMinimizeGolden -count=1
go test ./internal/cli -run TestMinimizeGolden -update     # 改行为后重生成
go test ./internal/minimize -count=1

# 搜索耗时（实测回填，不进断言）
go test ./internal/cli -run '^$' -bench BenchmarkMinimizeSearch -benchtime=5x -count=1

# 归因（M3 形态，M5 前仍是唯一 compare 输出）
go run ./cmd/bdiff compare \
  --left  data/fixtures/c01-mtime/pair/left/synthpkg-0.1.0.tar \
  --right data/fixtures/c01-mtime/pair/right/synthpkg-0.1.0.tar
# → exit 2（仅元数据），T1>0，residue=0
```
