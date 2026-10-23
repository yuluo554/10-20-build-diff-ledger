# plan/00-README 总览

> 方法论：`ai-tool-project-sprint`（装订 2026-10-20）｜布局门：`node tools/check-sprint-layout.mjs`｜续接对话第一件事 = 跑它

仓库 `10-20-build-diff-ledger`（构建产物差异归因账本）出题阶段总览。v2（2026-10-20，M0 装订 + 骨架落盘）。

**项目**：把两份**已产出**的构建产物（目录 / tar / tar.gz / zip / whl / jar / OCI layer tar）转成可复算的差异分类账本 → 给出每类差异的计数、归一化后的残留字节、最小原因集 → 输出确定性报告并以退出码当 CI 门禁。
**方法论**：数据先行 + 闭合分类法 + 纯函数幂等归一化 + 反事实实验 + 合成真值 golden + 确定性产物。
**交付形态**：**纯本地 CLI（单二进制，零运行时依赖、不跑构建、不联网）**。单一形态，不设备选。
**命名约定**：本地目录与 GitHub 仓库名均为 `10-20-build-diff-ledger`；命令名 `bdiff`；署名 `yuluo554`。
**定位宣言**："diffoscope 回答'哪里不同'，reprotest 回答'是否稳定'，strip-nondeterminism 回答'怎么抹平'，buildinfo 回答'怎么重建'——**没人回答'这是哪一类差异、有几处、归一化后还剩多少真差异、最少改哪几项就一致'**。本工具把两份已产出的构建产物转成可复算的分类账本 + 最小原因集 + CI 门禁。"
**品牌隔离**：本题只做构建产物字节层归因。文档不提名本系列任何其他项目，与其他项目零代码复制、零共享模块。

## 一、文档索引表

| 文档 | 内容 | 状态 |
|---|---|---|
| `plan/00-README总览.md` | 索引 / 决策记录（含弃选项）/ 承诺-证据台账 / 招牌动词表索引 / 里程碑 / 纪律 | ✅ v2（2026-10-20，M0） |
| `plan/01-题目文档.md` | 十段式题目定稿（头部 + 介绍 + 任务要求 + 提交材料 + 评分 + 锚点 + 边界） | ✅ v1（2026-10-20） |
| `plan/02-架构与技术选型.md` | 目录结构 / 依赖方向 / 退出码契约 / 确定性设计 | ✅ v1（2026-10-20，M0） |
| `plan/03-模块详设.md` | align / facts / attribute / minimize / report 五模块详设 + CLI 契约总表 | ✅ v1（2026-10-20，M0） |
| `plan/04-数据计划.md` | 样张矩阵 / 生成器设计 / golden 口径 / 重建判据 | ✅ v1（2026-10-20，M0） |
| `plan/05-里程碑与验收.md` | M0–M6 DoD / 布局门变异清单 / 固定收尾流程 / 已知未验证项 | ✅ v1（2026-10-20，M0） |
| `plan/06-交付对标与决策.md` | 提交材料落点 / 评分维度证据 / 风险登记 / 缺口 / 诚实口径表 | ✅ v1（2026-10-20，M0） |
| `AGENTS.md` | 方法论锚点（门命令 + 强制布局 + 三条硬约束 + 环境事实） | ✅ v1（2026-10-20，M0） |
| `tools/check-sprint-layout.mjs` | 布局门（11 条判据） | ✅ v1（2026-10-20，M0） |
| `tools/verify-layout-mutations.mjs` | 布局门变异验证（11 组，全红） | ✅ v1（2026-10-20，M0） |
| `tools/verify-go-guards.sh` | Go 守门变异验证（13 组，全红） | ✅ v1+M5（2026-10-20） |
| `tools/check-ci-example.sh` | `ci/example.yml` 的本地等价校验（0/1/2/3 四分支 + 放行≠消失） | ✅ M5（2026-10-20） |
| `data/README.md` | 依据查证（三态 + URL + 基准日）/ 数据源可获取性 / 合成数据纪律 / 数值口径（T1–T12、归一化 op、树摘要、退出码、`\|S\|≤12`） | ✅ v1（2026-10-20，阶段 4） |
| `.gitignore` | Go 标准模板 + 生成物输出目录 + 发布产物目录 | ✅ v1 |
| `go.mod` / `cmd/bdiff/` / `internal/` | 可运行骨架（version / selftest 可用，五模块待 M1–M5） | ✅ v1（2026-10-20，M0） |
| `tools/gen_fixtures/` | 样张生成器（14 组样张 + align golden 输入 + 反例；手写 tar/gzip/zip/ELF/PAX 扩展头） | ✅ v1+M2（2026-10-20） |
| `.github/workflows/ci.yml` / `ci/example.yml` | CI 矩阵 + 门禁示例 | ✅ v1（2026-10-20，M0） |
| `README.md` | 安装 / 三场景 / 字段说明 / 已知边界 / 状态表 | ✅ v1（2026-10-20，M0） |
| `data/golden/` | align / facts / attribute / minimize / report 五组 golden（M1–M5 逐段产出） | ✅ 五组齐（`report/` 为 14 × 2 份 json+md） |
| `plan/RELEASE-M6.md` | 发布留档（tag 前落盘；发布后事实分栏追加；§四 为 `v0.1.1` 二次迭代记录） | ✅ M6 + 0.1.1（2026-10-23） |

## 二、决策记录表

> 本节同时是**弃选项记录**。新增决策追加到表尾，不改历史行。

| 日期 | 决策 | 结论与理由 |
|---|---|---|
| 2026-10-20 | 选题：构建产物差异归因账本 v1（不参赛，发 GitHub 公开仓库） | 生态覆盖矩阵实测显示"哪里不同/是否稳定/怎么抹平/怎么重建"四件都有工具，而"分类计数 + 归一化残留 + 最小原因集 + CI 门禁"未检索到。差异化押在**闭合分类法 + 反事实归一化实验 + 最小原因集**，不押"能看出差异" |
| 2026-10-20 | 交付形态：**纯本地 CLI，单二进制，零运行时依赖** | 输入只有两份产物、输出只有 JSON/MD；无需服务、无需构建系统、无需网络。单形态才能给出形态化验收（退出码语义 + 字节确定性 + 零网络静态可证） |
| 2026-10-20 | 语言定案：**Go（标准库）** | `archive/tar`、`archive/zip`、`compress/gzip`、`debug/elf`、`debug/pe`、`debug/macho` 全在标准库；单二进制分发与"零运行时依赖"一致；不需要 Python/Node 运行时即可在干净机器上复核 |
| 2026-10-20 | **弃「OCI 镜像可复现检查器」** | `diffoci` **622★**（基准日快照）正在处理 strict/semantic 的核心矛盾（2026-10-13 推送），正面撞车；且镜像 layer 解析会引入 registry 交互，破坏"零网络"。layer tar 只作为**输入格式之一**保留，不解析镜像配置 |
| 2026-10-20 | **弃「rebuilderd 式重建编排」** | `rebuilderd` **431★**（基准日快照，2026-10-18 活跃）与 `oss-rebuild` **716★**（基准日快照）已占该位，且此类方案**必须真跑构建**，与"不跑构建"的形态红线冲突。本题只吃已产出产物 |
| 2026-10-20 | **弃「diffoscope TUI / HTML 阅读器」** | 差异渲染是 diffoscope 的成熟领地；阅读器没有确定性内核，无法进入 golden 逐字节验收，会变成"好看但不可复算"的空壳 |
| 2026-10-20 | **弃「SBOM / 许可证差异」** | 该方向头部已占（`sbom diff` 类检索头部 **549★ / 249★**，基准日快照），且属本题禁区（`plan/01` §六）；与"构建产物字节层"不同层，混入会破坏分类法的闭合性 |
| 2026-10-20 | **弃「静态配方 lint 单独立项」** | 规则型静态检查邻近工具已占（`hadolint` **12,483★**，基准日快照），且只有规则没有产物证据闭环；本题若需要，只作为 `--recipe` 附属模式（扩展项），不立项 |
| 2026-10-20 | **弃「发布 tarball ↔ tag 对账」为二期** | `--against-tag` 需要 git 仓库上下文与 tag 解析，属"另一条输入通路"，会稀释首期"两个产物字节比较"的单一内核；列为扩展项 |
| 2026-10-20 | **弃「模糊二进制对齐 / 相似度改名判定」** | 属 diffoscope 领地，且会引入阈值参数，破坏"可复算 + 无随机"。据此定纪律：**仅同 sha256 判改名**，多哈希重复即不判 |
| 2026-10-20 | 分类法闭合为 **T1–T12**，T12 = 未解释 | 有实证支撑（HN 39967003 列出"版本号未更新 / tag 错 / 属主元数据"等常见类）；闭合 + 显式 T12 避免"打不上标签就静默丢弃" |
| 2026-10-20 | 归一化 op 必须是**纯函数且幂等** | 反事实实验的前提。幂等性可直接断言 `normalize(normalize(x)) == normalize(x)`，无需人工口径解释 |
| 2026-10-20 | 树摘要 `sha256(排序后的 (relpath,hash))`，排序用 UTF-8 字节序 | 与归档内存储顺序解耦；同一份逻辑树在任何容器里摘要一致，才能把"条目顺序差异（T5）"与"内容差异"分开 |
| 2026-10-20 | 最小原因集搜索上限 `\|S\| ≤ 12`，按基数升序 + 字典序破平 | 2^12 上界可穷举；不做启发式剪枝（剪枝会破坏"最小"可复算性）。耗时以 M4 末态回填，不写死数字 |
| 2026-10-20 | 退出码补定为 **0/1/2/3** | 题面给定 0 一致 / 2 仅元数据 / 3 有残留；**1 补为"用法错误或输入不可解析"**——否则"读不了"会被误当成"一致"（静默通过的经典漏洞） |
| 2026-10-20 | 未知头部**必须**落 `raw_header_sha256`，不得静默丢弃 | "解析器没看懂"与"没有差异"必须可区分；这也是 M2 唯一可机械验收的反例形态 |
| 2026-10-20 | 数值口径一处定义（`data/README.md` §四） | T1–T12 定义、归一化 op 语义、树摘要算法、退出码、`\|S\|≤12`、幂等要求只在该处定义；代码常量与文档摘录全部引用它，禁止第二套表述 |
| 2026-10-20 | 发布里程碑采用 **A) Release 附产物** 口径 | M6 DoD 三件套：**git tag + GitHub Release 附产物（≥1 个手工上传附件，本地留副本 + sha256）+ 发布留档文件**。不降级为"源码 zipball 即产物" |
| 2026-10-20 | `align` 的 `SAME` = **结构对齐**（同 relpath 同形态），不表示内容一致 | 内容差异归 attribute（T12 / `residue_bytes`）。否则每处真改都会被重复记成 `ADDED`+`REMOVED`，报告里的 `added`/`removed` 被内容差异污染 |
| 2026-10-20 | §4.4 改名判定的"唯一"取 **每哈希唯一**（该 sha256 两侧各出现 1 次） | `plan/03` §二 的示例（同树 + 1 改名 + 1 新增 → `ADDED:1, RENAMED:1`）在"全局只剩一条 REMOVED/ADDED"的读法下永远不成立。两个 golden 同时钉住两种情形：`basic.json`（改名 + 新增同时算）与 `dup-hash.json`（同哈希多处 → 不判） |
| 2026-10-20 | 样张归档字节**由生成器手写**（tar / gzip 成员头 / zip / ELF），zip 一律 `Store` | `archive/tar.Writer` 会做格式推断与字段补全、`compress/gzip.Writer` 取不到 XFL/OS；`Store` 让 zip 字节不随 Go 版本漂移（R3 的提前处置） |
| 2026-10-20 | `c12-mixed` 的 `minimal_cause_set` 按 §4.5 判 `null` | 真改字节不可归一化 ⇒ 任何 S 都无法让树摘要相等。`plan/04` §二 "只含可解释类别"落为 `expect.json` 的 `counts_allowed` 约束（T1/T3/T9/T12） |
| 2026-10-23 | 已发布 tag / Release 的处置：**升 0.1.1 发新版**，`v0.1.0` 原样保留并标注已知问题 | `v0.1.0` 的 tag 树自跑 CI 是 failure（样张 deflate 跨 Go 版本漂移，修复在其后）、Release 正文 sha 错、`SHA256SUMS` 带 `dist/` 前缀、附件由失败 commit + dirty 树构建。移动已发布 tag 会破坏「已发布 tag 不动」，且任何已下载旧件会与新 tag 不符 |
| 2026-10-23 | 发布件构建命令统一为 `CGO_ENABLED=0 go build -trimpath -buildvcs=false` | `vcs.modified` 随工作树脏不脏翻转 → 产物落仓库内时两次构建不再逐字节相同（实测 `cmp` NOT_EQUAL）。与 CI `determinism` job 同一组 flag；可追溯性改由发布留档显式记录构建命令 + Go 版本 + 源 commit sha + 产物 sha256 |
| 2026-10-23 | 发布承诺边界 = **「回下载件 == 发布件」可校验**，不承诺「任意机器重建同一字节」 | 现有证据只有本机 Windows 两次构建相同；Go 工具链版本与目标 OS 都会影响字节，跨机可重建需先做跨版本/跨 OS 实测，不先写进承诺 |

## 三、承诺-证据台账

> 全项目只此一份。其他文档只可引用，不得复制表格。状态列随里程碑同一次提交翻转。

| 承诺项 | 素材来源 | 交付物内可达性 | 核验方式 | 状态 |
|---|---|---|---|---|
| 结构对齐（目录/tar/tar.gz/zip/whl/jar/OCI layer） | 标准库 `archive/tar`、`archive/zip`、`compress/gzip` | ✅ 单二进制内，无外部依赖 | `data/golden/align/basic.json` 逐字节 | ✅ M1 |
| 仅同哈希判改名 | 自定纪律（写进 `data/README.md` §四.4） | ✅ | 同树 +1 改名样张：恰 1 条 `RENAMED`；同哈希多处 → 0 条 | ✅ M1 |
| 事实抽取（tar/zip/gzip/ELF/PE/Mach-O） | 标准库 `debug/elf` + `debug/dwarf`；PE/Mach-O 手工读头 | ✅ | `internal/facts` 每 key 一个表驱动单测（`TestEveryKeyHasTest`） | ✅ M2 |
| 未知字段落 `raw_header_sha256` | 自定纪律（`data/README.md` §四.3） | ✅ | `c14-pax-unknown` + `TestRawHeaderUnknownPAXIsHandRecomputed` | ✅ M2 |
| 闭合分类法 T1–T12 | 实证：HN 39967003 常见差异类别；`data/README.md` §四.1 一处定义 | ✅ | 14 组样张分类命中率 100% | ✅ M3 |
| 归一化 op 幂等 | 自定纪律（纯函数） | ✅ | 逐 op `normalize(normalize(x))==normalize(x)` 断言 | ✅ M3 |
| 残留字节可复算 | 实际字节比对 | ✅ | golden 记录手工 `cmp` 计数 + 复现命令 | ✅ M3 |
| 最小原因集（非超集） | 自定搜索算法（`data/README.md` §四.5） | ✅ | `[T1]` / `[T1,T2]` 且 `len==2` / `null` 三组断言 | ✅ M4 |
| 确定性报告（固定键序） | 自定 schema | ✅ | 双跑逐字节 + `data/golden/report/`（14 × 2 份） | ✅ M5 |
| 退出码门禁 + `--budget` 白名单 | 自定契约（`data/README.md` §四.6） | ✅ | 退出码矩阵 4 行（0/1/2/3）全覆盖（`TestExitCodeMatrix`） | ✅ M5 |
| CI 可接入 | `ci/example.yml` 示例 | ✅ 纯本地、零网络 | 本地按示例跑通一遍（`tools/check-ci-example.sh`） | ✅ M5 |
| 单二进制零运行时依赖 | Go 标准库 | ✅ | 干净机器（无 Go 工具链）直接运行 | ⬜ 待 M6 验证 |
| 零网络 | 无网络调用 | ✅ 静态可证 | 依赖面审计（标准库白名单）+ 运行期无出网 | ⬜ 待 M6 验证 |
| 开源空白论据 | `gh api` 检索快照 2026-10-19T17:50Z–18:10Z | — | 检索式与 total_count 记录在 `plan/01` §1.4、`data/README.md` §一.2 | ✅ 已记录（⬜ 软论据口径） |

## 四、招牌动词表

> 表体在 `plan/01` §二末（唯一一份）。此处只登记交接要点：**能力名词 → 交付态命令 → 断言基准**三列在定稿时已全部填满，无空格。

| 能力名词 | 交付态命令 | 断言基准 |
|---|---|---|
| 分类计数 | `bdiff compare` 的 `counts` | 14 组样张 golden |
| 残留字节 | `compare` 的 `residue_bytes` | golden 中手工 `cmp` 计数 |
| 最小原因集 | `compare` 的 `minimal_cause_set` | 三组断言（`[T1]` / 非超集 `[T1,T2]` / `null`） |

## 五、提交材料实物表

> 对应 `plan/01` §三。每一项必须能指到一个具体文件；指不出的项此刻删除，不留到评分表里。

| 承诺项 | 应存在的文件路径 | 由哪个里程碑产出 | 用哪条命令核对 |
|---|---|---|---|
| 单二进制产物（`dist/bdiff-<os>-<arch>`，Release 附产物） | `dist/bdiff-<os>-<arch>`（本地副本）+ `plan/RELEASE-M6.md` 内 sha256 记录 | M6 | `sha256sum` 与留档比对；无 Go 工具链机器直接运行 |
| 源码可重建 | `cmd/bdiff/main.go`、`internal/{align,facts,attribute,minimize,report}/`、`go.mod` | M1–M5 | `go build ./...`（干净环境） |
| 样张生成器 | `tools/gen_fixtures/main.go` | M1 | 重跑生成器后 `git status` 无 golden 变更 |
| 14 组样张 + align golden 输入 + 反例 | `data/fixtures/<case>/`（每组含 `pair/`、`README.md`、`expect.json`）、`data/fixtures/align-golden/`、`data/fixtures/broken/` | M1–M2 | `go run ./tools/gen_fixtures --out data/fixtures --check` → `14 cases, 0 diff` |
| golden 集 | `data/golden/{align,facts,attribute,minimize,report}/**` | M1–M5 | 每文件附生成命令 + 手工复现命令；`cmp` 计数一致 |
| README | `README.md` | M6 | 三场景最小复现命令逐条跑通 |
| 发布留档 | `plan/RELEASE-M6.md` | M6（tag 前落盘） | `git cat-file -e <tag>:plan/RELEASE-M6.md` |
| CI 示例 | `ci/example.yml` | M5 | 本地按示例步骤跑一遍 |

## 六、里程碑与状态

> 状态副本三处：本表 / `plan/05` §一 / `README.md` 状态表。**三处逐字一致**由布局门机器断言。

| 里程碑 | 内容 | 状态 |
|---|---|---|
| **M0 计划先行 + 装订 + 骨架** | 方法论装订（`AGENTS.md` + 布局门 + 装订行）；`plan/00`–`06`；`data/README.md`；可运行骨架（`cmd/bdiff` + `internal/*` + `tools/gen_fixtures` 桩）；CI 与门禁示例 | ✅ 完成（2026-10-20） |
| **M1 数据先行 + align** | 样张生成器 `tools/gen_fixtures/`：同源码、不同 `SOURCE_DATE_EPOCH` / `umask` / `uid` / 构建目录，**构建两次**；产出 13 组样张 + `data/golden/align/basic.json`；`internal/artifact` + `internal/align` | ✅ 完成（2026-10-20） |
| **M2 解析（facts）** | tar/zip/gzip 头 + ELF/PE/Mach-O 事实抽取；`raw_header_sha256` 兜底 | ✅ 完成（2026-10-20） |
| **M3 归因（attribute）** | T1–T12 分类 + 幂等归一化 op + 树摘要 + 残留字节 | ✅ 完成（2026-10-20） |
| **M4 最小集（minimize）** | 按基数升序 + 字典序的穷举搜索（`\|S\|≤12`） | ✅ 完成（2026-10-20） |
| **M5 报告与门禁（report）** | `report.json` + `report.md` 固定键序；退出码 0/1/2/3；`--budget` 白名单；`ci/example.yml` | ✅ 完成（2026-10-20） |
| **M6 发布** | 干净环境验证 + 发布 | ✅ 完成（2026-10-20） |
| **M7 哨兵里程碑（变异靶子，实际不存在）** | — | ⬜ 未开始 |

**M6 DoD**：**git tag + GitHub Release 附产物（≥1 个手工上传附件，本地留副本 + sha256）+ 发布留档 `plan/RELEASE-M6.md`** 三件套齐；单二进制在无 Go 工具链的机器直接运行；发布 GitHub **公开仓库**。

## 七、纪律

1. **数值口径一处定义**：T1–T12、归一化 op、树摘要、退出码、`|S|≤12`、幂等要求只在 `data/README.md` §四定义；代码与文档摘录与之同源。
2. **不写漂移硬数字**：星数 / PyPI 月下载 / total_count 一律标注"基准日 2026-10-20 快照"，只进文档，**不写进代码或断言阈值**。
3. **不跑构建、不联网、不写输入目录**：M1 的样张生成器是**唯一**会调用构建/写文件的地方，且只写 `data/fixtures/` 与 `data/golden/`。
4. **未知即显式**：解析器看不懂的头部进 `raw_header_sha256`；归不进 T1–T11 的差异进 T12；读不了的输入 exit 1。三条都不许静默。
5. **形态唯一**：纯本地 CLI 单二进制。任何"再加个桌面/网页"的提议先回本表加一行弃选项。

## 八、本机环境事实（实测，带日期）

| 事实 | 命令 | 实测值 | 日期 |
|---|---|---|---|
| Node | `node --version` | `v24.15.0` | 2026-10-20 |
| Git | `git --version` | `2.54.0.windows.1` | 2026-10-20 |
| gh | `gh --version` | `2.93.0` | 2026-10-20 |
| Go | `go version` | 见下（本机原无 Go，本次安装） | 2026-10-20 |
| 网络 | `curl https://proxy.golang.org/` | 不通（国内网络）；`https://go.dev/dl/` 302，`mirrors.aliyun.com/golang/` 206 | 2026-10-20 |
| 默认分支 | `git branch --show-current` | `main` | 2026-10-20 |
| 远端 | `git remote -v` | `origin` = `https://github.com/yuluo554/10-20-build-diff-ledger` | 2026-10-20 建；2026-10-23 复核 |
| CI 实况 | `gh api .../actions/runs` | 7 条 run（6 绿 1 红）；HEAD `ba22b6a` 的 `39455730067` 六 job 全 success | 2026-10-23 |

**Go 安装记录（本机首次）**：`go.dev` 直连 TLS 失败、`proxy.golang.org` 不通 → 用阿里云镜像
`https://mirrors.aliyun.com/golang/go1.27.0.windows-amd64.zip` 下载并解到 **`C:\Users\ASUS\AppData\Local\Programs\Go`**。

> ⚠ **`C:\Program Files\Go` 不存在**（2026-10-23 实测 `ls -d "C:\Program Files\Go"` → No such file or directory）；
> 且 Go 的 `bin` **不在系统 PATH** 里——不设 PATH 时 `go` 报 `command not found`。Git Bash 下先设：
> `export PATH="/c/Users/ASUS/AppData/Local/Programs/Go/bin:$PATH"`（`tools/verify-go-guards.sh` 会自己按 `$LOCALAPPDATA` 兜底探测）。
> 第二个工具链 `go1.22.12` 已在本机 module 缓存里，用 `GOTOOLCHAIN=go1.22.12 go version` 调起，**不需要下载**。

**本项目零第三方依赖**，因此不需要 GOPROXY；`go build`/`go test` 可离线运行。安装后的实测版本号回填在本行下方：

```
go version → go1.27.0 windows/amd64
GOTOOLCHAIN=go1.22.12 go version → go1.22.12 windows/amd64
```

**M4 最小原因集搜索耗时（实测，2026-10-20；不写进断言）**：

| 项 | 实测 |
|---|---|
| 机器 | 笔记本（CPU：Intel Core i9-14900HX；机型名按脱敏纪律略去） |
| 命令 | `go test ./internal/cli -run '^$' -bench BenchmarkMinimizeSearch -benchtime=5x -count=1` |
| 早期命中（`c01-mtime`：候选 ∅ → `[T1]`） | `669560 ns/op` ≈ 0.67 ms |
| 无解全扫（`c12-mixed`：4096 个候选 × 两侧摘要） | `649155960 ns/op` ≈ 0.65 s |

口径：耗时随样张条目数与 ELF 节重写次数变化；上表是**本机实测值**，只回填文档，
不得写成测试阈值（`plan/06` §五「性能」一行）。

**纪律**：本表任何一行在下一棒引用前先复测（"本机没有 X"这类事实会过期）。
