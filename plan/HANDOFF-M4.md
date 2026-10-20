# HANDOFF-M4 · minimize（最小原因集）

> 交接单（第 M3 棒收尾 → 第 M4 棒开工）。**新对话第一件事：`node tools/check-sprint-layout.mjs`。**
> 上一棒：M3（attribute），2026-10-20 收尾。

## 一、当前进度 + 证据

| 项 | 状态 | 证据（可指认） |
|---|---|---|
| `attribute.Attribute` | ✅ | `internal/attribute/attribute.go`：`SideInput` / `EntryInput` / `Entries()`；输入 = 条目路径 + 归档内序号 + 内容字节 + facts |
| T1–T12 分类（首个命中 + priority） | ✅ | `classifyFact`；章节 key 只按**存在性**归 T8/T9/T10；`notes` 记 `rule_overlap` |
| T1–T11 归一化 op | ✅ | `internal/normalize/normalize.go`（opT1…opT11）；ELF 节表重写在 `internal/normalize/elf.go` |
| 逐 op 幂等 | ✅ T1–T11 各一条 | `TestEachRuleOpIsIdempotent`（每个 rule 取真实触发它的样张侧，且断言 op **确实改动**了输入） |
| `entry_stream` + 树摘要 | ✅ | `normalize.EntryStream` / `normalize.TreeDigest`；KAT 已按新编码更新（`TestTreeDigestKnownAnswer`） |
| `residue_bytes` | ✅ 逐字节，不用长度差 | `attribute.residueBytes`（Apply(T1–T11) 后逐条目 `EntryStream` 比对）；c11/c12 = 1，其余 = 0 |
| 14 组样张分类命中率 | ✅ 100% | `TestFixtureMatrix`（`expect.json` 驱动：正例、allowed、residue、退出码） |
| 树摘要重算 | ✅ | `TestExpectedCauseSetEqualizesDigests`（非 null 集合施加后两侧相等；去掉任一规则后不等；null → 全集后仍不等） |
| `data/golden/attribute/` | ✅ **14 份 + README** | 逐字节 golden；README 含生成命令、`cmp -l` 手工计数、独立 Python 摘要复算、`normalized_all` 边界 |
| `bdiff compare`（M3 形态） | ✅ | 输出归因 JSON；退出码 0/2/3 由 §4.6 判定；`--budget` 已接入 0/2（不降级 3） |
| 布局门 | ✅ 全绿 | `node tools/check-sprint-layout.mjs`（42 个 Go 文件 / 40 个 import 块） |
| 布局门变异 | ✅ **11/11 全红** | V4 已改打 M4、V10 已改打 `HANDOFF-M5` |
| Go 守门变异 | ✅ **9/9 全红** | 新增 G9（把 `tar.mtime` 误分类成 T3 → `TestFixtureMatrix` 红） |
| 本机测试 | ✅ | `go build ./...` / `go vet ./...` / `go test ./...`（12 个包全绿）；`gofmt -l .` 为空 |
| 样张重建 | ✅ | `go run ./tools/gen_fixtures --out data/fixtures --check` → `14 cases, 0 diff` |
| CI | 🟡 配置就位，**未实跑** | 仍无远端（U1） |

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **`data/README.md` §4.1/§4.2/§4.5 是唯一口径**。M3 新增/澄清：`entry_stream` 编码（§4.5）、
   op 顺序为**数值序**（§4.2）、`tar.size` 派生同步（§4.2）。改语义必须同提交改这些节 + golden。
2. **`entry_stream(e) = decimal(index) ‖ 0x00 ‖ (key ‖ 0x00 ‖ value ‖ 0x0A)* ‖ 0x00 ‖ content`**。
   `index` 进摘要，T5 才有可观测效果；facts 进摘要，T1–T3/T6/T7 才能被归一化。
   唯一实现 = `normalize.EntryStream`；`TreeDigest` 与 `residueBytes` 都调它，禁止复制第二套。
3. **op 施加顺序 = `model.RuleIDs` 的数值序**。不要用 `sort.Strings`：字符串序会把 `T10` 排到 `T2` 前，
   T8→T9→T10→T11 的依赖关系会被打乱。
4. **`tar.size` 是派生字段**：`Apply` 末尾把它同步为 `len(归一化后的内容字节)`；分类时忽略它。
   否则 T10 删 `.sig` 后 size 变小会被重复记成 T12（本棒实测 residue=3 就是这个坑）。
5. **ELF 节 key 的 offset/size 值差异是派生的**：分类只看**存在性**。
   `.debug_info.size` 随 comp_dir 长度变化 → 不能记 T9（否则 c10 会多出 T9）。
6. **内容字节差异只有在"内容型"事实差异存在时才被解释**：`elf.comment/producer/build_id/comp_dir`、
   `.sig`/`.note.gnu.build-id`/`.debug_*`/`.comment` 节 key。tar/zip 头部事实**不能**解释条目数据差异
   （否则 c12 的真改会被 T1/T3 吞掉）。
7. **T4 的公共前缀按侧计算**：`commonDirPrefix` 取"所有路径都还有内容"的公共目录前缀；
   目录条目自身会截断前缀（这正好给出"首个稳定锚点"）。前缀存进 `normalize.Entry.RootPrefix`，
   T4 op 才剥离；attribute 用**剥离后的路径**匹配，但摘要用 `Entry.Path`（S=∅ 时前缀仍在，S={T4} 才相等）。
8. **T5 的前提**：条目集合与内容哈希完全相同（`sameEntrySet`）；满足时才按 index 记 T5。
9. **容器级事实**是 `Path=""`、`Index=-1` 的条目；**不能跳过**（T6/T7 的 gzip/zip 容器字段都在这里）。
10. **ELF 重写只处理 ELF64 小端**（样张形态）：按 `tools/gen_fixtures/write_elf.go` 的布局重建
    （头 → 各节按 Addralign → 8 字节对齐 → shstrtab → 8 字节对齐 → 节表）。非 ELF64 原样返回，
    事实仍归一化、内容字节可能残留——这是已登记边界（plan/06 R4），不要假装覆盖真实产物。
11. **T8/T9/T10/T11 的删除语义**：T8 清空 `.comment` 与 producer；T9 删 `.note.gnu.build-id` + DWARF；
    T10 删 `.sig`；T11 把 comp_dir 换成 `<build-root>`。每个 op 都同步删除对应节事实。
12. **`attribute` 不得 import `artifact`（K4）**：`SideInput` 由 `internal/cli.buildAttributeSide` 构造
    （容器事实 + 每条目 `facts.Extract`）。`attribute.Entries` 只做前缀/序号预处理，供 M4 复算摘要。
13. **M3 的 `compare` 只输出归因 JSON**：`--out` 与 `--format=md|both` 明确 exit 1（M5 补齐）；
    退出码 0/2/3 已按 §4.6 生效，`--budget` 只影响 0/2，不降级 3。
14. **golden 是逐字节断言**：改 attribute/op/分类后必须 `go test ./internal/cli -run TestAttributeGolden -update`
    重新生成并人工检查 diff。c13 的 `counts` 是 T10=2（offset+size 两条），不是 1。
15. **变异 marker 会随状态翻转失效**：本棒已改 V4（打 M4）、V10（打 `HANDOFF-M5`）、G3（op 表新行）；
    新增 G9。下一棒翻 M4 状态时必须再改 V4。
16. **状态三副本**（`plan/00` §六 / `plan/05` §一 / `README.md`）同提交翻转。K8。
17. **`plan/06` 缺口列不许出现已 ✅ 的里程碑编号**（M0–M3 都照判）。K9。

## 三、下一棒待办（M4）

按 `plan/05` §二 M4 DoD 与 `plan/03` §五：

1. [ ] `internal/minimize.Search` 落地：消费已实现的 `Combinations()`（先基数升序、同基数字典序），
       对每个 S 调 `normalize.TreeDigest(S, entries)`；**不剪枝、不随机、不启发**。
       签名已定：`Search(leftDigest, rightDigest func([]string) string) ([]string, bool, error)`；
       两侧条目用 `attribute.Entries(side)` 构造（前缀/序号已就位）。
2. [ ] 搜索结果的返回口径：`(set, true, nil)` 找到（空集 = 完全一致）；`(nil, false, nil)` 无解，
       且调用方必须同时看到 `residue_bytes > 0`（§4.5 的"两者同真"）。
3. [ ] `data/golden/minimize/` 逐字节 + README（生成 + 手工复算：把 S 逐个代入 `compare`/测试）。
4. [ ] DoD 四条样张断言：`c01 → [T1]`；`c09 → [T1,T2]` 且 `len==2`（显式非超集）；
       `c11 → null` 且 `residue_bytes > 0`；`c12 → null`（真改不可归一化），并断言 T1/T3/T9
       已把元数据差异抹平（残留只来自 `table.csv` 的 1 字节）。
5. [ ] `attribute.Entries` 不要复制逻辑：若 M4 需要缓存摘要，缓存键 = `(side, S)`，不要重写编码。
6. [ ] 搜索耗时实测回填 `plan/00`（实测值，**不写进断言**）；机器型号一并写。
7. [ ] 回写三处状态（M4 → ✅）+ 缺口表；写 `plan/HANDOFF-M5.md`（七节）。
8. [ ] 变异脚本：V4 改打 M5；若新增判据（如"最小集非超集"）→ 同步新增变异（G10）。
9. [ ] 可选：`compare` 是否在 M4 就输出 `minimal_cause_set`。当前输出是 attribute JSON；
       M5 的 report struct 才有该字段。若要在 M4 接线，必须同步改 plan/03 §一 的分阶段说明。

**M4 的已知难点**：

- `c09` 的 `[T1,T2]` 必须 `len==2`：若把 T1/T2 的差异误记到别的类（或 op 不生效），搜索会给出超集/空集。
  先用 `TestExpectedCauseSetEqualizesDigests` 的同类断言确认。
- `c05` 的 `[T5]` 依赖 index 进 `entry_stream`；若有人"优化"掉 index，T5 的最小集会退化成 `∅`。
- 2^12 = 4096 次候选，每次要对两侧各 Apply 一遍；不要在每个候选里重新 `facts.Extract`。

## 四、既定口径清单（K 编号 —— 动了会打挂测试）

| K | 口径 | 打挂什么 |
|---|---|---|
| K1 | 规则全集 = `model.RuleIDs`（12 条，T12 末尾） | `guard.TestRuleSetClosed`、`LAYOUT.CONTRACT` |
| K2 | 退出码 0/1/2/3 语义固定（2 不得被用法错误占用） | `cli.TestExitCodeContract`、`cli.TestCompareUsageErrors` |
| K3 | 受限 import 黑名单（含 `time`），`cmd/`+`internal/`+`tools/` 三处都扫 | `guard.TestNoForbiddenImports`、`LAYOUT.NO_NET` |
| K4 | 依赖方向 = `LayerOrder`（`attribute` 只可 import `model`+`facts`+`normalize`；`normalize` 只可 import `model`） | `guard.TestDependencyDirection` |
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
| **K22** | `entry_stream` 编码（index ‖ facts ‖ content）唯一实现 = `normalize.EntryStream` | `TestEntryStreamHasIndexAndFacts`、`TestTreeDigestKnownAnswer` |
| **K23** | op 施加顺序 = 数值序（`T1<T2<…<T11`），不是字符串序 | `TestExpectedCauseSetEqualizesDigests`（T8/T9/T10/T11 先后） |
| **K24** | `tar.size` = 派生字段，Apply 末尾同步、分类忽略 | `TestFixtureMatrix/c13-signature`（T10 不得带出 T12） |
| **K25** | ELF 节 offset/size 值差异 = 派生；分类只看存在性 | `TestFixtureMatrix/c10-toolchain-path`（不得出现 T9） |
| **K26** | T4 前缀按侧计算、存 `RootPrefix`、op 剥离；S=∅ 摘要仍带前缀 | `TestT4StripsPrefixBeforeContent`、`TestFixtureMatrix/c04-prefix` |
| **K27** | 内容字节差异只由"内容型"ELF 事实解释（tar/zip 头事实不算） | `TestFixtureMatrix/c12-mixed`（T12 仍出现） |
| **K28** | `attribute` 不 import `artifact`；`SideInput` 由 cli 构造 | `guard.TestDependencyDirection` |
| **K29** | M3 的 `compare` 只输出归因 JSON；`--out`/`md` 明确 exit 1 | `TestCompareUsageErrors` |
| **K30** | golden 逐字节 + 双跑一致；改行为必须 `-update` 并人工核对 | `TestAttributeGolden` |

## 五、本机环境坑（只写本棒实测过的）

| 坑 | 实测 | 处置 |
|---|---|---|
| `python` 是商店占位符 | 读 stdin 的管道立刻关闭 → `write /dev/stdout: The pipe is being closed` | 一律 `py -3.12` |
| Git Bash 的 `/tmp` 与 Windows exe 的 `/tmp` 不是同一处 | 二进制写到 `C:\tmp`，bash 读 `/tmp/out.json` 找不到 | 临时文件放仓库内相对目录（本棒用 `.tmpout/`），收尾删除 |
| `sort.Strings` 排 rule id | `T10` 排在 `T2` 前 | 按 `model.RuleIDs` 下标排序（数值序） |
| `[]normalize.Entry` append 后指针失效 | 先存 `&entries[i]` 再 append 会让 map 里的指针指向旧数组 | map 存**下标**，取值时再索引 |
| `tar.size` 派生差异 | c13 施加 T10 后 residue=3（"864" vs "720"） | Apply 末尾同步 `tar.size = len(Bytes)`；分类忽略 |
| ELF 节 size 派生差异 | c10 的 `.debug_info.size` 随 comp_dir 变长 → 误记 T9 | 节 key 只按存在性分类；值差异视为派生 |
| `debug/elf` 不导出 `e_shoff` | 重建节表时无法直接写回 | ELF64 头偏移固定：`e_shoff=40`、`e_shnum=60`、`e_shstrndx=62`；复制原头后只改这三处 |
| 变异 marker 随状态翻转失效 | V4 打 M3（已 ✅）；V10 打 `HANDOFF-M4`（已 `[x]`）；G3 打 `"T9": nil`（已换实现） | 已改打 M4 / `HANDOFF-M5` / `"T9": opT9`；新增 G9 |
| `go` 不在 PATH | `go: command not found` | `export PATH="$(cygpath -u "$LOCALAPPDATA")/Programs/Go/bin:$PATH"` |

## 六、DoD 逐项 checklist（M4 出口时逐条打勾）

- [ ] `go test ./...` 全绿（含 minimize 的最小性/无解断言）
- [ ] `go vet ./...` 干净、`gofmt -l .` 为空
- [ ] `go run ./tools/gen_fixtures --out data/fixtures --check` → `14 cases, 0 diff`
- [ ] `c01 → [T1]`；`c09 → [T1,T2]` 且 `len==2`；`c11 → null` 且 `residue>0`；`c12 → null`
- [ ] `data/golden/minimize/` 逐字节 + README（生成 / 手工复算）
- [ ] 搜索耗时实测回填 `plan/00`（不写进断言）
- [ ] `node tools/check-sprint-layout.mjs` 全绿
- [ ] `node tools/verify-layout-mutations.mjs` 11/11（V4 已打 M4，**M4 ✅ 后要改打 M5**）
- [ ] `bash tools/verify-go-guards.sh` 9/9（新增判据 → 同步新增变异）
- [ ] 三处状态副本 M4 → ✅ 且互相一致
- [ ] `plan/06` 缺口表更新（不点名已 ✅ 的里程碑）
- [ ] `plan/HANDOFF-M5.md` 七节齐

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

# 归因（M3 已可用：输出 counts/diffs/residue/tree_digest/notes）
go run ./cmd/bdiff compare \
  --left  data/fixtures/c01-mtime/pair/left/synthpkg-0.1.0.tar \
  --right data/fixtures/c01-mtime/pair/right/synthpkg-0.1.0.tar
# → exit 2（仅元数据），T1>0，residue=0

# 真改
go run ./cmd/bdiff compare \
  --left  data/fixtures/c11-real-change/pair/left/synthpkg-0.1.0.tar.gz \
  --right data/fixtures/c11-real-change/pair/right/synthpkg-0.1.0.tar.gz
# → exit 3，T12>0，residue=1

# attribute golden
go test ./internal/cli -run TestAttributeGolden -update      # 改行为后重生成
go test ./internal/cli -run TestAttributeGolden -count=1     # 逐字节校验

# 14 组矩阵 + 最小集复算 + 逐 op 幂等
go test ./internal/cli -run 'TestFixtureMatrix|TestExpectedCauseSet|TestEachRuleOp' -count=1
```
