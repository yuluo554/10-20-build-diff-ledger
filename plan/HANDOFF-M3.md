# HANDOFF-M3 · attribute（归因）

> 交接单（第 M2 棒收尾 → 第 M3 棒开工）。**新对话第一件事：`node tools/check-sprint-layout.mjs`。**
> 上一棒：M2（facts），2026-10-20 收尾。

## 一、当前进度 + 证据

| 项 | 状态 | 证据（可指认） |
|---|---|---|
| `internal/facts.Extract` / `ExtractDocument` | ✅ | `internal/facts/facts.go`（tar/zip/gzip 头字段 + 内容格式分派 + 排序） |
| 每 key 一个表驱动单测 | ✅ | `internal/facts/facts_test.go` 的 `keyTests`；`TestEveryKeyHasTest` / `TestNoOrphanKeyTests` / `TestEachKeyHasTest` |
| ELF 事实 | ✅ | `internal/facts/elf.go`：`.comment`、`.note.gnu.build-id`（desc hex）、DWARF `producer`/`comp_dir`、6 个跟踪节的 offset/size |
| PE 抽样 | ✅ | `internal/facts/pe.go`：`pe.timestamp` / `pe.checksum` / `pe.rich.present` / `pe.rich.digest`（测试内合成 PE） |
| Mach-O 抽样 | ✅ | `internal/facts/macho.go`：`uuid`/`minos`/`sdk`/`tools`（thin 32/64 + fat 第一架构；测试内合成） |
| `raw_header_sha256` + 未知扩展头反例 | ✅ | `c14-pax-unknown`（PAX 未知 keyword `SCHILY.xattr.user.marker`）；`TestRawHeaderUnknownPAXIsHandRecomputed` 用**独立字节扫描**重算并比对 |
| `artifact.Raw` 缺口 | ✅ | `Raw.Index`（T5，目录输入 = -1）、`Raw.RawHeader`、`Raw.Tar`/`Raw.Zip`、`Document.Container`（容器级 facts）；`internal/artifact/tar.go` 手写 512 块解析 + `zip.go` 中央目录/本地头原文 |
| 样张 | ✅ **14 组** | `data/fixtures/c01-mtime` … `c14-pax-unknown`（每组 `pair/left`、`pair/right`、`expect.json`、`README.md`） |
| `--check` 位级一致 | ✅ | `go run ./tools/gen_fixtures --out data/fixtures --check` → `14 cases, 0 diff`，exit 0 |
| `data/golden/facts/` 逐字节 | ✅ | 5 份：`c01-tar-left.json` / `c06-gzip-left.json` / `c07-zip-left.json` / `c08-elf-left.json` / `c14-pax-left.json` + `README.md`（生成 + 手工复现命令）；`TestFactsGoldenMatchesCLI` |
| `bdiff facts` 接线 | ✅ | `--artifact`、`--format json`、`--layer-kind=oci`、`--max-entry-bytes`；退出码 1 的用例外延有测试 |
| 布局门 | ✅ 全绿 | `node tools/check-sprint-layout.mjs`（39 个 Go 文件 / 0 受限 import） |
| 布局门变异 | ✅ **11/11 全红** | `node tools/verify-layout-mutations.mjs`（V4/V7/V10 的 marker 已随本棒源码更新） |
| Go 守门变异 | ✅ **8/8 全红** | `bash tools/verify-go-guards.sh` |
| 本机测试 | ✅ | `go build ./...` / `go vet ./...` / `go test ./...`（12 个包全绿）；`gofmt -l .` 为空 |
| CI | 🟡 配置就位，**未实跑** | `.github/workflows/ci.yml` 的 `gen_fixtures --check` 会看到 14 组；仍无远端（U1） |

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **`data/README.md` §四是唯一数值口径**。M2 新增/澄清：§4.3 的 raw_header **字节构造表**、§4.10 的 facts 取值格式。
   改 facts 语义必须同提交改这两节；golden 会红。
2. **容器级事实的 `path` 是空串 `""`**：`gzip.mtime/xfl/os`、`zip.create_system`、gzip 的 `raw_header_sha256`。
   attribute 做差异集合时**不能跳过 path=""**，否则 T6/T7 永远命中不了。
3. **`raw_header_sha256` 只描述条目自身的容器头**：tar 里的 ELF 条目 = tar 头（不是 ELF 头）；
   只有目录装载的裸文件才用文件格式头（ELF/PE/Mach-O）。见 §4.3 的"嵌套规则"。
4. **tar 解析器是手写的**（`artifact/tar.go`）：校验和必须通过、PAX `x/g` 与 GNU `L/K` 原文进 RawHeader；
   未知 PAX keyword 不解释但不丢弃。改这里等于改 T7 的判定输入。
5. **zip 的 RawHeader 是"逐分量 le32 长度前缀"拼接**（`artifact/zip.go`）；本地头 extra 与中央目录 extra
   是两份，都收。`zip.create_system` 取**第一条**中央目录记录的 version-made-by 高字节。
6. **`Raw.Index` 是归档内序号（含目录条目，扩展头不算）**；目录输入 = -1。T5 只能用归档输入判。
7. **目录条目不参与 facts**（plan/03 §二.5），`facts.Extract` 对 dir 返回空。attribute 不要因为
   "dir 没有 facts" 判它残差。
8. **`facts.Keys()` = `FixedKeys` + `TrackedSections` 派生**（`.text`/`.comment`/`.note.gnu.build-id`/
   `.debug_info`/`.debug_abbrev`/`.sig`）。节 key 形如 `elf.section..text.offset`（双点，§4.10 已写明）。
9. **T10 的"签名段存在性"** 目前的可观测形态是 `elf.section..sig.size` 这个 key 是否出现；
   没有独立的 `elf.sig.present`。T9 的"DWARF 段存在性"同理看 `elf.section..debug_info.size`。
10. **14 组样张的期望不能回填**：`expect.json` 只写 inputs 与 expect（plan/04 §四）。
    c14 的期望是 `T7`、`[T7]`、exit 2（PAX 扩展头差异 = 归档头字段差异）。
11. **状态三副本**（`plan/00` §六 / `plan/05` §一 / `README.md`）必须同提交翻转；布局门逐副本采集。K8。
12. **变异脚本 marker 必须匹配当前源码**。本棒因 M2 状态翻转与 import 行变化重定向了
    V4（改打 M3）、V7（`bufio` 行）、V10（`HANDOFF-M4` 行）。K15。
13. **`plan/06` 缺口列不许出现已 ✅ 的里程碑编号**（M0/M1/M2 都照判）。K9。

## 三、下一棒待办（M3）

按 `plan/05` §二 M3 DoD 与 `plan/03` §四：

1. [ ] `internal/attribute.Attribute` 落地：输入 = 两侧 `facts` + `align` 条目表；输出 `counts`/`diffs`/
       `residue_bytes`/`tree_digest`（键序按 plan/03 §四 的示例）。
2. [ ] **分类顺序 T1→T12 取首个命中**，命中两个以上时 `notes` 记 `rule_overlap`（§4.1 闭合性）。
       逐类映射建议（用现有 key 即可，不要再造 key）：
       - T1：`tar.mtime` / `zip.mtime` / `gzip.mtime`
       - T2：`tar.uid/gid/uname/gname`
       - T3：`tar.mode` / `zip.external_attr` 的权限位
       - T4：relpath 公共根前缀（c04 两侧全部路径不同，align 会出全量 ADDED/REMOVED——T4 必须在内容比对之前处理）
       - T5：`Raw.Index` 序列（条目集合与哈希相同、仅顺序不同）
       - T6：`gzip.xfl` / `gzip.os`
       - T7：`raw_header_sha256` / `zip.create_system`（c07、c14）
       - T8：`elf.comment` / `elf.producer`
       - T9：`elf.build_id` + `elf.section..debug_info.size`/`.debug_abbrev` 存在性
       - T10：`elf.section..sig.*` 存在性
       - T11：`elf.comp_dir` + 内嵌绝对路径
       - T12：其余（必须让 `residue_bytes > 0`）
3. [ ] 归一化 op 的逐 op 幂等断言 `normalize(normalize(x))==normalize(x)`（T1–T11；T12 无 op）。
4. [ ] `residue_bytes` 用**逐字节 cmp** 计数，不用长度差；与 c11/c12 的 `expect.residue_bytes` 对齐。
5. [ ] 树摘要唯一实现仍在 `normalize.TreeDigest`（K5），attribute 只调用不复制。
6. [ ] `data/golden/attribute/` 逐字节 + `README.md`（生成 + 手工复现）。
7. [ ] 回写三处状态（M3 → ✅）+ 缺口表；写 `plan/HANDOFF-M4.md`。

**M3 的已知难点（先想清楚再写代码）**：

- **T4 与 align 的交互**：c04 两侧所有 relpath 都带不同前缀，align 输出是"全删 + 全增"。
  attribute 必须先剥离公共根前缀再匹配，否则 T4 样张会被记成大量 T12/增删。
- **T5 需要原始序号**：align 的结果按 relpath 排序，序号不在 `model.Entry` 里；
  attribute 需要自己 `artifact.LoadDocument` 两侧（或让 align 结果携带 Index——改 align 要同提交改 §4.9 与 golden）。
- **T10 的归一化**（R8）：删 `.sig` 段会移动后续节表，归一化必须按固定节布局重建；
  生成器的 ELF 布局固定（`tools/gen_fixtures/write_elf.go`），可按同一布局重写。
- **PE/Mach-O 没有样张**：本棒只做了抽样单测；M3 不要把它们写进 14 组分类命中率承诺（plan/06 R4）。

## 四、既定口径清单（K 编号 —— 动了会打挂测试）

| K | 口径 | 打挂什么 |
|---|---|---|
| K1 | 规则全集 = `model.RuleIDs`（12 条，T12 末尾） | `guard.TestRuleSetClosed`、`LAYOUT.CONTRACT` |
| K2 | 退出码 0/1/2/3 语义固定（2 不得被用法错误占用） | `cli.TestExitCodeContract`、`cli.TestFactsUsageErrors` |
| K3 | 受限 import 黑名单（含 `time`），`cmd/`+`internal/`+`tools/` 三处都扫 | `guard.TestNoForbiddenImports`、`LAYOUT.NO_NET` |
| K4 | 依赖方向 = `LayerOrder`（`facts` 只可 import `model`+`artifact`；测试同样受扫描） | `guard.TestDependencyDirection` |
| K5 | `tree_digest` 唯一实现 = `normalize.TreeDigest`，分隔符 `0x00`/`0x0A` | `normalize.TestTreeDigest*` |
| K6 | `report.json` 键序 = `Report` struct 字段序 | `report.TestReportKeyOrder` |
| K7 | `--budget` 三形态：单值 / 逗号多值 / `all` | `report.TestParseBudgetThreeForms`、`cli.TestBudgetParsedBeforeWork` |
| K8 | 状态三副本同提交翻转 | `LAYOUT.STATUS` |
| K9 | 缺口列不点名已 ✅ 里程碑 | `LAYOUT.GAPS` |
| K10 | 改名仅同 sha256 且**该哈希两侧各出现 1 次** | `TestRenameOnlyOnEqualHash`、`data/golden/align/dup-hash.json`、变异 G7 |
| K11 | 部分生成（`--only`）不写全集清单 | `TestOnlyWritesNoManifest`、变异 G8 |
| K12 | 报告不写绝对路径/主机名/用户名；错误只报 basename | `cli.TestUsageErrorIsNotMetaOnly`、`artifact.TestErrorDoesNotLeakAbsolutePath` |
| K13 | 目录条目 sha256 = `align.EmptySHA256`（§4.9 常量） | `align.TestEmptyDigestConstant` |
| K14 | 样张归档字节 = 生成器手写输出（zip `Store`、tar 手写 ustar、gzip 成员头手写、PAX 手写） | `gen_fixtures` 的 `--check` 与 `TestCheckIsFalsifiable` |
| K15 | 变异脚本 marker 必须匹配当前源码空白/文本 | 两个变异脚本的"marker 不存在"报错 |
| K16 | facts key 集合 = `facts.Keys()`（`FixedKeys` + `TrackedSections` 派生），每 key 必须有单测 | `facts.TestEveryKeyHasTest`、`facts.TestNoOrphanKeyTests` |
| K17 | `raw_header_sha256` 的字节构造 = `data/README.md` §4.3 的表 | `facts.TestRawHeaderUnknownPAXIsHandRecomputed`、`data/golden/facts/*` |
| K18 | 容器级事实 `path=""`（gzip.* / zip.create_system / gzip raw_header） | `data/golden/facts/c06-gzip-left.json`、`c07-zip-left.json` |
| K19 | facts 取值格式（十进制/hex/`hex:`/`zip.mtime=YYYYMMDDHHMMSS`/节 key 双点）= §4.10 | 全部 facts golden |
| K20 | 样张数 = `caseDefs()` 长度（当前 14），`--check` 文案随之 | `gen_test.TestCaseMatrixIs14AndCoversT1ToT12`、`TestFullRunWritesManifest14` |
| K21 | `facts` 的未知头部不得静默：PAX 未知 keyword 进 RawHeader；损坏输入 exit 1 | `TestRawHeaderUnknownPAXIsHandRecomputed`、`cli.TestFactsRejectsUnreadableInput` |

## 五、本机环境坑（只写本棒实测过的）

| 坑 | 实测 | 处置 |
|---|---|---|
| 嗅探后读指针没拨回 0（第二次踩） | tar.gz 的 gzip 头读成 `dbfe`（实际读到了第 512 字节处） | 每个解析器入口先 `f.Seek(0, io.SeekStart)`；本棒在 `LoadDocument` 的 gzip 分支补上 |
| 子切片用绝对下标 | `rec := cd[pos:end]` 后又用 `rec[46+...:end]` → `slice bounds out of range [:312] with capacity 196` | 子切片内部的切片一律用**相对长度** `recLen`；新增 `readZipCentral` 后先跨记录跑一遍 |
| `debug/elf` 不导出 `e_phoff`/`e_shoff` | 编译期没有 `ef.Phoff` 字段 | 按 ELF 规范手工读（类/字节序仍用 `ef.Class`/`ef.ByteOrder`） |
| Go 形参分组写错 | `func f(t *testing.T, want, parts []string, ...)` 把 want 也变成了 []string | 类型不同的相邻形参必须逐一声明类型 |
| `archive/tar` 不暴露原始块 | 无法取 PAX/GNU 扩展头原文 | 手写 tar 解析器（`artifact/tar.go`）；校验和双口径（signed/unsigned）都接受 |
| 变异 marker 随状态翻转失效 | V4 打 M2（已 ✅）→ "marker 不存在"；V7 打 `archive/tar` 行（已删）；V10 打 `- [ ] HANDOFF-M3`（已改 `[x]`） | 变异靶子挑**仍未完成**的里程碑/当前源码真实文本；本棒已改打 M3、`bufio` 行、`HANDOFF-M4` 行 |
| `go` 不在 PATH | `go: command not found` | `export PATH="$(cygpath -u "$LOCALAPPDATA")/Programs/Go/bin:$PATH"` |
| `python` 是商店占位符 | 退出 49，脚本静默不执行 | 一律 `py -3.12`；改文件后重新读回验证 |

## 六、DoD 逐项 checklist（M3 出口时逐条打勾）

- [ ] `go test ./...` 全绿（含 attribute 的分类/幂等/残留/摘要断言）
- [ ] `go vet ./...` 干净、`gofmt -l .` 为空
- [ ] `go run ./tools/gen_fixtures --out data/fixtures --check` → `14 cases, 0 diff`
- [ ] 14 组样张分类命中率 100%（`expect.json` 驱动；c14 命中 T7）
- [ ] 逐 op 幂等断言（T1–T11 各一条）
- [ ] `residue_bytes` 与 c11/c12 的手工 `cmp` 计数一致
- [ ] `data/golden/attribute/` 逐字节 + README（生成 / 手工复现）
- [ ] `node tools/check-sprint-layout.mjs` 全绿
- [ ] `node tools/verify-layout-mutations.mjs` 11/11
- [ ] `bash tools/verify-go-guards.sh` 8/8（新增判据 → 同步新增变异）
- [ ] 三处状态副本 M3 → ✅ 且互相一致
- [ ] `plan/06` 缺口表更新（不点名已 ✅ 的里程碑）
- [ ] `plan/HANDOFF-M4.md` 七节齐

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
go run ./tools/gen_fixtures --out data/fixtures            # 全量重生成（含 manifest）
go run ./tools/gen_fixtures --out data/fixtures --check    # 必须 "14 cases, 0 diff"
go run ./tools/gen_fixtures --out /tmp/fx --only c14-pax-unknown

# facts（M2 已可用）
go run ./cmd/bdiff facts --artifact data/fixtures/c01-mtime/pair/left/synthpkg-0.1.0.tar
go run ./cmd/bdiff facts --artifact data/fixtures/c06-gzip-opts/pair/left/synthpkg-0.1.0.tar.gz
go run ./cmd/bdiff facts --artifact data/fixtures/c07-archive-hdr/pair/left/synthpkg-0.1.0.zip
go run ./cmd/bdiff facts --artifact data/fixtures/c14-pax-unknown/pair/left/synthpkg-0.1.0.tar

# facts golden 重生成（改 facts 输出形状后）
go run ./cmd/bdiff facts --artifact data/fixtures/c08-toolchain/pair/left/synthpkg-0.1.0.tar \
  > data/golden/facts/c08-elf-left.json

# align（M1 已可用）
go run ./cmd/bdiff align --left data/fixtures/c01-mtime/pair/left --right data/fixtures/c01-mtime/pair/right
```
