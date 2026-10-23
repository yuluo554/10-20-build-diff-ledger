# HANDOFF-M1 · 数据先行 + align

> 交接单（第 M0 棒收尾 → 第 M1 棒开工）。**新对话第一件事：`node tools/check-sprint-layout.mjs`。**
> 上一棒：M0（计划 + 装订 + 骨架），2026-10-20 收尾。

## 一、当前进度 + 证据

| 项 | 状态 | 证据（可指认） |
|---|---|---|
| 方法论装订三件 | ✅ | `AGENTS.md` + `tools/check-sprint-layout.mjs`（11 条判据）+ `plan/00` 顶部装订行 |
| 布局门变异验证 | ✅ **11/11 全红** | `node tools/verify-layout-mutations.mjs` → `结论：11/11 组变异如期变红`，还原后基线全绿 |
| Go 守门变异验证 | ✅ **6/6 全红** | `bash tools/verify-go-guards.sh` → `结论：6/6 组 Go 守门变异如期变红，还原后源码字节一致` |
| 计划套件 | ✅ | `plan/00`–`plan/06` 七份，`plan/05` §一 为状态权威之一 |
| 数值口径 | ✅ | `data/README.md` §四（T1–T12 / op / 树摘要 / 退出码 / `\|S\|≤12`） |
| 可运行骨架 | ✅ | `go build ./...`、`go vet ./...`、`go test ./...` 本地通过；`bdiff version` / `bdiff selftest` 实跑通过 |
| selftest 契约自检 | ✅ | 17 条断言全 PASS（规则全集 12 / 子集枚举 4096 / budget 三形态 / 摘要稳定） |
| CI 配置 | ✅ 配置就位，**未实跑** | `.github/workflows/ci.yml`（3 job）、`ci/example.yml`；本地 `yaml.safe_load` 解析通过 |
| 样张与 golden | ⬜ | 全部待 M1 |
| 五模块实现 | ⬜ | `align`/`facts`/`attribute`/`minimize` 均返回 `ErrNotImplemented`；`report` 的序列化/预算/渲染已实现 |

**本地等价校验（未接远端，CI 待首推验证）**：`go vet` / `go test ./...` / `go build -trimpath` / 布局门 / 变异验证 已跑；
**不可本机实跑**：GitHub Actions 矩阵（ubuntu/windows × go 1.22/1.24）——首推时验证。

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **`data/README.md` §四是唯一数值口径**。代码里的 `model.RuleIDs` 与 `normalize.Ops` 是它的镜像；
   改任一处必须同提交改 §4.1/§4.2，否则 `LAYOUT.CONTRACT` 或 `go test ./internal/guard` 会红。
2. **`model.RuleIDs` 是规则全集的唯一代码来源**。`minimize.MaxSetSize`、`report.Counts` 的键序都由它派生；
   不要在任何地方再写一份 `["T1",...,"T12"]` 字面量（会漂）。
3. **退出码 2 是业务语义（仅元数据差异），不是用法错误**。所有参数错误必须 `ExitUsage(1)`。
   `internal/cli` 是唯一出口；深层包禁止 `os.Exit`。
4. **受限 import 黑名单已生效**（`net`/`net/http`/`time`/`math/rand`/`os/exec`/`os/user`），
   双份守门：布局门 `LAYOUT.NO_NET` + `internal/guard` 的 `TestNoForbiddenImports`。
   要读时间就传参数（`--source-date-epoch`），**不要 import `time`**。
5. **依赖方向表在 `internal/guard/guard.go` 的 `LayerOrder`**。新增内部包必须在那里登记，否则
   `TestDependencyDirection` 报"未登记"（这是刻意的：登记 = 声明允许的方向）。
6. **样张生成器的 `--only` 绝不写全集清单**（`tools/gen_fixtures/main.go` 的 `run` 里已有分叉；
   M1 实现清单写盘时必须保住这条，并有测试钉住）。
7. **M0 的 status 三副本**（`plan/00` §六 / `plan/05` §一 / `README.md` 状态表）必须同提交翻转。
   布局门会逐副本采集状态、任一里程碑出现两种状态即红——**不要只改一处**。
8. **`plan/06` 缺口列不许出现已 ✅ 的里程碑编号**（M0 也照判，已无豁免）。
   关闭缺口时写"已关闭：证据见发布留档的某一节"，**不要点文件名**（`RELEASE-M6.md` 里的 `M6` 会撞判据）。
9. **变异验证脚本是常驻资产**：`tools/verify-layout-mutations.mjs` 的 10 组变异必须保持全红；
   新增判据 = 必须新增一组覆盖它的变异（"一条门几个判据就几组变异"）。
10. **本机 Go 装在 `%LOCALAPPDATA%\Programs\Go`**（非 PATH 默认）。Git Bash 里先
    `export PATH="$LOCALAPPDATA/Programs/Go/bin:$PATH"`，否则 `go: command not found`。

## 三、下一棒待办（M1）

按 `plan/05` §二 M1 DoD 逐条：

1. [ ] `tools/gen_fixtures` 实现 13 组合成（`plan/04` §二矩阵；含 `c13-signature` 覆盖 T10）
2. [ ] 生成器 `--check`：生成到临时目录 → 与 `data/fixtures/` 逐字节比对；输出 `13 cases, 0 diff`
3. [ ] 生成器**不读时钟、不读环境变量**；时间戳只取 `{0, 1700000000, 1700003600}`
4. [ ] `internal/artifact.Load`：目录 / tar / tar.gz / zip / whl / jar / OCI layer tar（内存流，不落临时目录）
5. [ ] `internal/align.Align`：relpath 规范化 + 字节序排序 + 四类计数 + **仅同 sha256 判改名**
6. [ ] `data/golden/align/basic.json` + 其 `README.md`（生成命令 + 手工复现命令）
7. [ ] `TestAlignDeterministic`（同输入双跑逐字节）
8. [ ] `TestRenameOnlyOnEqualHash`（同哈希多处出现 → 不判改名，记 ADDED/REMOVED）
9. [ ] 回写三处状态（M1 → ✅）+ 缺口表；写 `plan/HANDOFF-M2.md`
10. [ ] `git status --short` 干净后提交

## 四、既定口径清单（K 编号 —— 动了会打挂测试）

| K | 口径 | 打挂什么 |
|---|---|---|
| K1 | 规则全集 = `model.RuleIDs`（12 条，T12 末尾） | `guard.TestRuleSetClosed`、`LAYOUT.CONTRACT` |
| K2 | 退出码 0/1/2/3 语义固定（2 不得被用法错误占用） | `cli.TestExitCodeContract` |
| K3 | 受限 import 黑名单 | `guard.TestNoForbiddenImports`、`LAYOUT.NO_NET` |
| K4 | 依赖方向 = `LayerOrder` | `guard.TestDependencyDirection` |
| K5 | `tree_digest` 唯一实现 = `normalize.TreeDigest`，分隔符 `0x00`/`0x0A` | `normalize.TestTreeDigest*` |
| K6 | `report.json` 键序 = `Report` struct 字段序 | `report.TestReportKeyOrder` |
| K7 | `--budget` 三形态：单值 / 逗号多值 / `all` | `report.TestParseBudgetThreeForms`、`cli.TestBudgetParsedBeforeWork` |
| K8 | 状态三副本同提交翻转 | `LAYOUT.STATUS` |
| K9 | 缺口列不点名已 ✅ 里程碑 | `LAYOUT.GAPS` |
| K10 | 改名仅同 sha256 且两侧各出现 1 次 | M1 新增测试（本棒建） |
| K11 | 部分生成（`--only`）不写全集清单 | M1 新增测试（本棒建） |
| K12 | 报告不写绝对路径/主机名/用户名；错误只报 basename | `cli.TestUsageErrorIsNotMetaOnly` |

## 五、本机环境坑（只写实测过的）

| 坑 | 实测 | 处置 |
|---|---|---|
| `go` 不在 PATH | `go: command not found`（2026-10-20） | `export PATH="$LOCALAPPDATA/Programs/Go/bin:$PATH"` |
| `go.dev` 直连 TLS 失败、`proxy.golang.org` 不通 | `curl` 实测 000 / TLS handshake failed（2026-10-20） | 阿里云镜像 `mirrors.aliyun.com/golang/`；本项目零第三方依赖，不需要 GOPROXY |
| `C:\Program Files` 不可写 | `mkdir: Permission denied`（2026-10-20） | Go 装到 `%LOCALAPPDATA%\Programs\Go` |
| `winget install` 后台无输出、卡住 | 5 分钟后 `go` 仍不见（2026-10-20） | 别用 winget；直接下载 zip 解压 |
| 表格里的 `\|` 转义会破坏按列解析 | 布局门首版把 `\|S\|≤12` 当列分隔符 → M4 状态"既无 ✅ 也无 ⬜" | 门的 `splitRow` 先把 `\|` 替换成占位符再切（已修） |
| 写成"示例路径"的字面量会被文档工具判据当真 | `plan/05` 曾把一个**不存在的**脚本完整路径写进变异说明 → `LAYOUT.DOC_TOOLS` 红 | 文档里描述变异时不要写出完整的 `tools/<name>.<ext>` 形态；用"某个不存在的 tools 脚本"这类描述（本行也踩过一次） |
| **变异脚本自己是假门**：`go` 不在 PATH 时每条 `go test` 都非零 | 首版 `verify-go-guards.sh` 把 6/6 全判成"如期变红"，而它们只是"命令不存在"（2026-10-20 实测） | ① 脚本先 `go version` 自证可用，否则 exit 1；② 变红判据改为"输出含 `--- FAIL: <TestName>`"，不是只看退出码；③ 编译错单独判 FAIL |
| **"不撞"型断言守不住拼接格式** | `TestTreeDigestSeparators`（不同切分给不同摘要）在删掉 0x00 后**仍通过**——hex 定长，切分本来就撞不上 | 补 `TestTreeDigestKnownAnswer`：期望值由独立实现（Python hashlib）按 §4.5 算出，少一个字节即失配 |

## 六、DoD 逐项 checklist（M1 出口时逐条打勾）

- [ ] `go run ./tools/gen_fixtures --out data/fixtures --check` → `13 cases, 0 diff`，exit 0
- [ ] `go run ./tools/gen_fixtures --out /tmp/x --only c01-mtime` 后 `git status --short` 为空（K11）
- [ ] `go test ./...` 全绿（含新增 align/artifact 测试）
- [ ] `go vet ./...` 干净
- [ ] `node tools/check-sprint-layout.mjs` 全绿
- [ ] `node tools/verify-layout-mutations.mjs` 10/10（新增判据 → 同步新增变异）
- [ ] `data/golden/align/basic.json` 存在且其旁边有 README（生成命令 + 手工复现命令）
- [ ] 三处状态副本 M1 → ✅ 且互相一致
- [ ] `plan/06` 缺口表移除 G2（不点名文件）
- [ ] `plan/HANDOFF-M2.md` 七节齐

## 七、关键命令速查

```bash
# 环境
export PATH="$LOCALAPPDATA/Programs/Go/bin:$PATH"

# 门（每次开工第一条）
node tools/check-sprint-layout.mjs
node tools/verify-layout-mutations.mjs
bash tools/verify-go-guards.sh

# Go
go build ./...
go vet ./...
go test ./...

# 构建与冒烟
CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /tmp/bdiff.exe ./cmd/bdiff
/tmp/bdiff.exe version
/tmp/bdiff.exe selftest

# 样张（M1 起）
go run ./tools/gen_fixtures --out data/fixtures --check
go run ./tools/gen_fixtures --out /tmp/fx --only c01-mtime

# YAML 本地预检（GitHub 不建 job 的第一道防线）
py -3.12 -c "import yaml;[yaml.safe_load(open(f,encoding='utf-8')) for f in ['.github/workflows/ci.yml','ci/example.yml']];print('yaml ok')"
```
