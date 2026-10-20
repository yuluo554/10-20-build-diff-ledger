# HANDOFF-M2 · facts（解析）

> 交接单（第 M1 棒收尾 → 第 M2 棒开工）。**新对话第一件事：`node tools/check-sprint-layout.mjs`。**
> 上一棒：M1（数据先行 + align），2026-10-20 收尾。

## 一、当前进度 + 证据

| 项 | 状态 | 证据（可指认） |
|---|---|---|
| 样张生成器 | ✅ | `tools/gen_fixtures/`（`main.go` / `matrix.go` / `cases.go` / `content.go` / `write_tar.go` / `write_zip.go` / `write_elf.go`） |
| 13 组样张 | ✅ **13 组** | `data/fixtures/c01-mtime` … `c13-signature`（每组含 `pair/left`、`pair/right`、`expect.json`、`README.md`） |
| `--check` 位级一致 | ✅ | `go run ./tools/gen_fixtures --out data/fixtures --check` → `13 cases, 0 diff`，exit 0 |
| 部分运行不写清单（K11） | ✅ | `TestOnlyWritesNoManifest`（含"已有全量树上跑 `--only`，manifest 字节不变"）+ 变异 G8 |
| `internal/artifact.Load` | ✅ | 目录 / tar / tar.gz / zip（`.whl`/`.jar` 同 zip）/ `--layer-kind=oci`；内存流、不落临时目录 |
| `internal/align.Align` | ✅ | 条目表 + 四类计数 + 仅同 sha256 改名；`bdiff align --format json|md` 已可用 |
| align golden | ✅ | `data/golden/align/basic.json`（改名 + 新增同时算）、`dup-hash.json`（同哈希多处 → 不判）+ `README.md`（生成命令 + 手工复现命令） |
| 双跑逐字节 | ✅ | `TestAlignDeterministic`（包内）+ `TestAlignGoldenMatchesCLI`（CLI 全链路） |
| 反例样张 | ✅ | `data/fixtures/broken/truncated.tar`（断言 exit 1）；未知扩展头样张**留给 M2**（见 §三） |
| 布局门 | ✅ 全绿 | `node tools/check-sprint-layout.mjs`（31 个 Go 文件 / 0 受限 import） |
| 布局门变异 | ✅ **11/11 全红** | `node tools/verify-layout-mutations.mjs` |
| Go 守门变异 | ✅ **8/8 全红** | `bash tools/verify-go-guards.sh`（新增 G7 改名纪律、G8 `--only` 清单） |
| 本机测试 | ✅ | `go build ./...` / `go vet ./...` / `go test ./...`（8 个包全绿；含新增 artifact / align / CLI / generator 测试） |
| CI | 🟡 配置就位，**未实跑** | `.github/workflows/ci.yml` 新增 `gen_fixtures --check` 步骤；仍无远端（U1） |

**本地等价校验（未接远端，CI 待首推验证）**：`go vet` / `go test ./...` / `go build -trimpath` / 布局门 / 两个变异脚本 / `gen_fixtures --check` 已跑。

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **`data/README.md` §四是唯一数值口径；§4.9 是条目常量与对齐语义的唯一出处**（目录 sha256 常量、
   `--max-entry-bytes` 默认、`SAME`/`RENAMED` 语义、relpath 规范化）。改语义必须同提交改 §4.9。
2. **`align` 的 `SAME` = 结构对齐（同 relpath 同形态），不是内容一致**。内容差异归 attribute（T12）。
   不要把它改成"内容相同才 SAME"——那会让每处真改都多出 `ADDED`+`REMOVED` 两行。
3. **改名判定是"每哈希唯一"**：该 sha256 在两侧各出现 1 次、且两侧各只有一条未匹配条目携带它。
   `data/golden/align/dup-hash.json` 钉住反例。K10。
4. **样张归档字节由生成器手写**（`tools/gen_fixtures/write_*.go`），zip 一律 `Store`。
   改归档字节 = 改 golden 输入 → `--check` 会红，必须整目录重生成。
5. **`tools/` 与 `cmd/`、`internal/` 同在受限 import 黑名单内**（`time` 也算）。要写时间语义就手写头部
   字段（本棒已示范：tar 512 字节块、gzip 成员头、MS-DOS 时间三元组）。
6. **`--only` 绝不写 `manifest.json`**（清单写盘条件 = `only == ""`）。K11，有测试 + 变异守。
7. **`--check` 比较的是整棵 `data/fixtures` 树**（含 `align-golden/`、`broken/`、`manifest.json`）。
   打印的 `N cases` 只数 13 组分类样张；diff 数包含全部文件。
8. **状态三副本**（`plan/00` §六 / `plan/05` §一 / `README.md`）必须同提交翻转；布局门逐副本采集。K8。
9. **变异脚本的 marker 必须匹配当前源码的空白**。gofmt 会改 struct 对齐列宽，marker 写死旧空白就变成
   "marker 不存在" → 变异没发生、脚本却可能"看起来通过"。本棒已因此重定向 V4 / V7 / V10 / G4，
   新增 G7 / G8 时也踩过一次（删整行导致 `declared and not used` → 编译错，被脚本正确判为"没跑起来"）。
10. **`python` 在 Git Bash 里是商店占位符**（`python --version` 退出 49，静默不执行）。
    跑 Python 一律用 `py -3.12`；`tools/verify-go-guards.sh` 已内置这条探测。
11. **`go` 不在 PATH**：`export PATH="$(cygpath -u "$LOCALAPPDATA")/Programs/Go/bin:$PATH"`。
12. **`plan/06` 缺口列不许出现已 ✅ 的里程碑编号**（M0/M1 都照判）。K9。

## 三、下一棒待办（M2）

按 `plan/05` §二 M2 DoD 与 `plan/03` §三：

1. [ ] `internal/facts.Extract` 落地：tar/zip/gzip 头字段（`facts.Keys` 每条一个表驱动单测；
       key 集合由代码常量驱动，新增 key 漏测即红）
2. [ ] ELF 事实：`.comment`、`.note.gnu.build-id`、DWARF `producer`/`comp_dir`、`elf.section.<name>.offset/size`
       ——样张的 ELF 已由生成器手写并被 `TestGeneratedELFIsParseable` 用 `debug/elf`+`debug/dwarf` 验过
3. [ ] PE（`pe.timestamp`/`pe.checksum`/`pe.rich.*`）与 Mach-O（`macho.uuid`/`minos`/`sdk`/`tools`）抽样
4. [ ] `raw_header_sha256`（§4.3 覆盖范围）：**未知扩展头反例样张**（PAX 未知 keyword 或 zip extra field）
       → 断言该 key 出现且等于手工重算值。**新增样张会让样张数变 14**：同提交改 `plan/04`、`plan/05`
       与 `--check` 文案（打印的 `N cases` 自动跟随 `caseDefs()`，不用改代码）
5. [ ] `data/golden/facts/` 逐字节 + `README.md`（生成命令 + 手工复现命令）
6. [ ] **`artifact.Raw` 的缺口要先补**：M2 需要"归档内条目序号（T5）"与"原始头部字节"（raw_header），
       当前 `Raw` 只有 Path/Kind/Size/Data。扩展点建议放在 `artifact`（叶子包），并让 `align` 继续只用
       四项字段；容器级事实（`gzip.mtime/xfl/os`、`zip.create_system`）不属于任何条目，需要一个显式的
       容器级返回值 —— 设计时先写进 `plan/03` §三再写代码
7. [ ] `bdiff facts` 子命令接线（`--artifact`、`--format json`）
8. [ ] 回写三处状态（M2 → ✅）+ 缺口表；写 `plan/HANDOFF-M3.md`

## 四、既定口径清单（K 编号 —— 动了会打挂测试）

| K | 口径 | 打挂什么 |
|---|---|---|
| K1 | 规则全集 = `model.RuleIDs`（12 条，T12 末尾） | `guard.TestRuleSetClosed`、`LAYOUT.CONTRACT` |
| K2 | 退出码 0/1/2/3 语义固定（2 不得被用法错误占用） | `cli.TestExitCodeContract` |
| K3 | 受限 import 黑名单（含 `time`），`cmd/`+`internal/`+`tools/` 三处都扫 | `guard.TestNoForbiddenImports`、`LAYOUT.NO_NET` |
| K4 | 依赖方向 = `LayerOrder` | `guard.TestDependencyDirection` |
| K5 | `tree_digest` 唯一实现 = `normalize.TreeDigest`，分隔符 `0x00`/`0x0A` | `normalize.TestTreeDigest*` |
| K6 | `report.json` 键序 = `Report` struct 字段序 | `report.TestReportKeyOrder` |
| K7 | `--budget` 三形态：单值 / 逗号多值 / `all` | `report.TestParseBudgetThreeForms`、`cli.TestBudgetParsedBeforeWork` |
| K8 | 状态三副本同提交翻转 | `LAYOUT.STATUS` |
| K9 | 缺口列不点名已 ✅ 里程碑 | `LAYOUT.GAPS` |
| K10 | 改名仅同 sha256 且**该哈希两侧各出现 1 次** | `TestRenameOnlyOnEqualHash`、`data/golden/align/dup-hash.json`、变异 G7 |
| K11 | 部分生成（`--only`）不写全集清单 | `TestOnlyWritesNoManifest`、变异 G8 |
| K12 | 报告不写绝对路径/主机名/用户名；错误只报 basename | `cli.TestUsageErrorIsNotMetaOnly`、`artifact.TestErrorDoesNotLeakAbsolutePath` |
| K13 | 目录条目 sha256 = `align.EmptySHA256`（§4.9 常量） | `align.TestEmptyDigestConstant` |
| K14 | 样张归档字节 = 生成器手写输出（zip `Store`、tar 手写 ustar、gzip 成员头手写） | `gen_fixtures` 的 `--check` 与 `TestCheckIsFalsifiable` |
| K15 | 变异脚本 marker 必须匹配当前源码空白 | 两个变异脚本的"marker 不存在"报错 |

## 五、本机环境坑（只写本棒实测过的）

| 坑 | 实测 | 处置 |
|---|---|---|
| `python` 是 Windows 商店占位符 | `python --version` → 退出 **49**，脚本里静默不执行（本棒两次以为改了文件，实际没改） | 一律 `py -3.12`；改文件后**必须重新读回验证** |
| `go` 不在 PATH | `go: command not found` | `export PATH="$(cygpath -u "$LOCALAPPDATA")/Programs/Go/bin:$PATH"`（`cygpath` 那一步不能省） |
| tar 校验和字段写错位置 | 首版把 `\0`/空格写到 154/156 → 覆盖了 typeflag，`tar` 直接报 "does not look like a tar archive" | chksum 占 **148..155**（6 八进制 + NUL + 空格），**156 是 typeflag** |
| 嗅探后没把读指针拨回 0 | `gzip: invalid header`（前 512 字节已被嗅探读掉） | 每次解析前 `f.Seek(0, io.SeekStart)` |
| 魔数不认识时按 tar 兜底会"成功返回 0 条目" | go.mod 被当成空 tar → 静默通过 | 兜底分支加"0 条目即不支持"判据 |
| gofmt 改 struct 对齐 → 变异 marker 失效 | V7 / G4 都出现 "marker 不存在" | 变异前先核对当前源码原样文本；脚本报错即算失败，不许跳过 |
| 变异方式导致编译错 | G7 首版删整行 → `declared and not used: rightHash`，被判"没跑起来" | 变异要选**仍能编译**的改法（本棒改成 `!= 999`） |
| `.gitignore` 的 `*.tar` 会拦住样张 | 逐条 `git check-ignore -v` 看是否命中 `!data/fixtures/**/*.tar` 否定规则 | 新增归档目录时同提交补否定规则 |

## 六、DoD 逐项 checklist（M2 出口时逐条打勾）

- [ ] `go test ./...` 全绿（含 facts 的表驱动 key 测试）
- [ ] `go vet ./...` 干净、`gofmt -l .` 为空
- [ ] `go run ./tools/gen_fixtures --out data/fixtures --check` → `N cases, 0 diff`（N = 当前样张数）
- [ ] `data/golden/facts/` 逐字节断言（含 `README.md` 的生成 / 手工复现命令）
- [ ] 未知扩展头反例 → `raw_header_sha256` 出现且等于手工重算值
- [ ] `node tools/check-sprint-layout.mjs` 全绿
- [ ] `node tools/verify-layout-mutations.mjs` 11/11
- [ ] `bash tools/verify-go-guards.sh` 8/8（新增判据 → 同步新增变异）
- [ ] 三处状态副本 M2 → ✅ 且互相一致
- [ ] `plan/06` 缺口表更新（不点名已 ✅ 的里程碑）
- [ ] `plan/HANDOFF-M3.md` 七节齐

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
go run ./tools/gen_fixtures --out data/fixtures --check    # 必须 "13 cases, 0 diff"
go run ./tools/gen_fixtures --out /tmp/fx --only c01-mtime # 部分生成（不写清单）

# align（已可用）
go run ./cmd/bdiff align --left data/fixtures/c01-mtime/pair/left --right data/fixtures/c01-mtime/pair/right
go run ./cmd/bdiff align --left data/fixtures/align-golden/rename-add/pair/left \
  --right data/fixtures/align-golden/rename-add/pair/right --format md

# golden 重生成（改 align 输出形状后）
go run ./cmd/bdiff align --left  data/fixtures/align-golden/rename-add/pair/left \
  --right data/fixtures/align-golden/rename-add/pair/right > data/golden/align/basic.json
```
