# data · 依据查证、数据源可获取性与数值口径

> 本文件是**数值口径的唯一权威**。`plan/01`、`plan/03` 与代码里的公式均为便于阅读的摘录；若与 §四 冲突，以 §四 为准，并在同一提交内修正摘录。
> 基准日：**2026-10-20**。GitHub API 快照时间戳：**2026-10-19T17:50Z–18:10Z UTC**（即基准日当天）。星数 / PyPI 月下载 / `total_count` 均为**基准日快照**，会随时间漂移，只用于说明生态位，**不得写进代码常量或断言阈值**。

## 一、依据查证记录（三态：✅ 已核对 / ⬜ 软论据或未取得）

### 1.1 生态覆盖矩阵（✅ 实测快照，基准日 2026-10-20）

| 编号 | 工具 | 快照 | 输入 → 输出 | 覆盖位 | 不覆盖 |
|---|---|---|---|---|---|
| ECO-DIFFOSCOPE | diffoscope | GitHub 镜像 `anthraxx/diffoscope` 95★（主仓在 salsa.debian.org，2026-09 报告称 330–332 在更新）；PyPI 6,206 次/月 | 两文件/目录/归档 → 递归逐层差异（`--text/--html/--json/--markdown`；`--exclude`/`--diff-mask`/`--exclude-directory-metadata`） | 哪里不同 | 无原因分类与计数、无反事实归一化实验、无最小原因集；大输入退化/报告超长 |
| ECO-REPROTEST | reprotest | `SantiagoTorres/reprotest` 1★，PyPI 179 次/月，0.7.33 | 源码 + 构建命令 → 变异环境重复构建 pass/fail | 是否稳定 | 必须真跑两次构建；不做产物级归因 |
| ECO-REBUILDERD | rebuilderd | `kpcyrd/rebuilderd` 431★（2026-10-18 活跃） | 包 + 源 → 校验位比对 | 重建校验 | 不解释不一致；#7 原话"只能本地重跑再手工 diffoscope" |
| ECO-DEBREBUILD | debrebuild | 18★ | 构建记录 → 重建指令 | 怎么重建 | 只给指令不比较产物 |
| ECO-RPMREPRODUCE | rpmreproduce | 11★ | 同上 | 怎么重建 | 同上 |
| ECO-BUILDINFO-CRAWLER | rebuilderd-debian-buildinfo-crawler | 6★ | 同上 | 怎么重建 | 同上 |
| ECO-SDE | SOURCE_DATE_EPOCH | 规范，非工具 | 约定时间戳来源 | 怎么约定 | 不检测"抹平后是否一致" |
| ECO-SND | strip-nondeterminism | 6★ | 产物 → 抹平后产物 | 怎么抹平 | 擦除不解释（不产出账本/计数） |
| ECO-ADDDET | add-determinism | 155★ | 产物 → 抹平后产物 | 怎么抹平 | 同上 |
| ECO-DIFFOCI | diffoci | 622★（2026-10-13 推送） | 两镜像 → 差异（strict/semantic 两档） | 镜像差异 | 无分级归因（#266）；仅 OCI 镜像 |
| ECO-ELFDIFF | elf_diff | 273★ | ELF 符号 → 差异 | 符号级比较 | 非构建归因 |
| ECO-CONTAINERDIFF | container-diff | 3,798★（2024-03 停更） | 包管理内容 → 差异 | 包级比较 | 非构建归因 |
| ECO-DISORDERFS | disorderfs | 3★ | 制造非确定性 | 制造差异 | 不解释差异 |
| ECO-DETTRACE | dettrace | 43★ | 拦截环境 | 制造差异 | 同上 |
| ECO-FAKETIME | libfaketime | 3,125★ | 篡改时间 | 制造差异 | 同上 |

**结论（✅ 实测）**：四件（哪里不同 / 是否稳定 / 怎么抹平 / 怎么重建）都有；"分类计数 + 归一化后残留 + 最小原因集 + CI 门禁"未检索到。

### 1.2 开源空白检索（⬜ 软论据：`gh api` 检索式 + total_count，窗口 2026-10-19T17:50Z–18:10Z UTC）

| 检索式 | total_count | 头部 / 说明 |
|---|---|---|
| `reproducible build comparison` | 17 | 头部 7★，无关 |
| `build artifact comparison` | 7 | 头部 29★，无关 |
| `artifact attribution` | 64 | 头部 5★，无关 |
| `build diff gate` | 62 | 头部 86★，前端门禁，非产物 |
| `benign differences build` | 1 | 0★ |
| `nondeterminism classifier` | **0** | — |
| `artifact diff classifier` | **0** | — |
| `nondeterminism taxonomy` | **0** | — |
| `artifact diff taxonomy` | **0** | — |
| `build diff report json` | 7 | 无关 |
| `diffoscope json` | **0** | — |
| `canonical archive hash` | **0** | — |
| `semantic digest archive` | **0** | — |
| `minimal cause set` | 11 | 无关 |
| `subset search normalization` | 1 | 0★ |
| 对照 `elf_diff` | 19 | 头部 273★ |
| 对照 `container-diff` | 177 | 头部 3,798★ |
| 对照 `diffoci` | 4 | 头部 622★ |
| 对照 `topic:reproducible-builds` | 508 | 生态存在 |

**结论（⬜）**：0 结果检索式一律按软论据处理，只写"未检索到"，**不得写"首创 / 无人做过"**。检索是推断，不是市场调查。

### 1.3 需求侧原话（✅ 逐条核对原话，基准日 2026-10-20）

| 编号 | 原话（未改写） | 来源 |
|---|---|---|
| DS-279 | "I'm comparing folders with a huge amount of files. And I am only interested in content differences... I tried `--exclude-directory-metadata` ... with no luck — to be honest, from the manpage I do not really understand what these options will do." | https://salsa.debian.org/reproducible-builds/diffoscope/-/work_items/279 |
| DS-14 | "there's a bit of noise that could be avoided... Ignores all differences in control.tar. Ignores timestamp differences in data.tar. Disables fuzzy matching."（jwilk） | https://salsa.debian.org/reproducible-builds/diffoscope/-/work_items/14 |
| DS-431 | "I sometimes run into situations where diffoscope bails out with: file-specific differences were detected; falling back to a binary diff."（mmdebstrap 作者） | https://salsa.debian.org/reproducible-builds/diffoscope/-/work_items/431 |
| HN-39968243 | "comparing 2 directories of around 3000 JSON/YAML files each ... diffoscope can't seem to process them ... got unstuck after some minutes" | https://news.ycombinator.com/item?id=39968243 |
| DIFFOCI-266 | "the unavoidable difference in image name annotations is the only remaining attribute that diffoci reports on my side." | https://github.com/reproducible-containers/diffoci/issues/266 |
| RB-7 | "Currently it's only possible to investigate why a build was unreproducible by rebuilding it locally and running for example diffoscope." | https://github.com/kpcyrd/rebuilderd/issues/7 |
| HN-39967003 | "Changes vary from forgetting to update an in-source version to tagging the wrong commit to fixing file metadata (file owner in the tarball)."（`_ikke_`） | https://news.ycombinator.com/item?id=39967003 |

> **证据缺口（如实记录）**：**Reddit 直连超时，未取得任何 Reddit 证据**，本题**不使用**任何 Reddit 论据；`plan/01` 与 README 均不出现 Reddit 引用。若后续取得，须单列一行并重新评估分类法覆盖。

## 二、数据源可获取性记录

| 数据源 | 三态 | 获取方式 | 备注 / 阻断原因 |
|---|---|---|---|
| 需求侧原话（7 条） | ✅ 已获取 | 逐条打开链接核对，原话照抄 | 基准日 2026-10-20；链接若失效，按"原话已存档于本文件"处理，不重新编造 |
| 生态快照（星数 / PyPI 月下载 / 最后推送） | ✅ 已获取（一次性快照） | GitHub API 与 PyPI 页面 | 时间戳 2026-10-19T17:50Z–18:10Z UTC；数字会漂移，属 ⬜ 软论据 |
| GitHub 检索 total_count | ✅ 已获取（一次性快照） | `gh api` 检索接口 | 同上窗口；软论据 |
| Reddit 需求讨论 | ⬜ **不可获取** | 直连超时 | 阻断原因：网络超时；**不作底座、不作论据** |
| 外部样本产物（真实发行版二进制） | ⬜ **不使用** | — | 隐私与分发风险；样张一律程序化合成，见 §三 |
| 真实构建环境（多工具链/交叉编译） | ⬜ 待获取 | 尚无跨平台实测 | 首期只在单一开发环境验证；跨平台差异作为已知风险写进 README，不写"已全平台验证" |
| 最小原因集搜索耗时 | ⬜ 待获取 | 尚无实现 | `\|S\|≤12` 的上界是设计选择，实测耗时以 M4 末态回填 |

> **本项目的核心数据依赖为零**：工具运行时只读用户给定的两个产物文件；仓库内所有样张由 `tools/gen_fixtures/` 程序化生成。无网络依赖、无外部 API、无版本库抓取。

## 三、合成数据纪律

- **样张为程序化生成**：`tools/gen_fixtures/` 用同一份合成源码树构建两次，仅改变 `SOURCE_DATE_EPOCH` / `umask` / `uid` / `gid` / 构建目录 / 工具链串等受控输入，从而得到"差异已知、类别已知"的样张对。
- **不引用任何真实发行版二进制、真实公司、真实人员、真实域名**。合成项目名固定 `synthpkg`；版本号固定 `0.1.0`；作者固定 `Fixture Bot <fixture@example.invalid>`（`example.invalid` 只作为**文档占位**出现，绝不作为 git 提交身份）。
- **时间戳白名单**：样张内所有时间戳取自固定表 `{ 0, 1700000000, 1700003600 }`（`SOURCE_DATE_EPOCH` 语义下的秒数），不使用生成时的系统时间；生成器必须显式传入而非读时钟。
- **样张矩阵（14 组，写死）**：单因 8（T1 时间戳 / T2 属主 / T3 权限 / T4 路径前缀 / T5 条目顺序 / T6 压缩器参数 / T7 归档头字段 / T8 工具链串）· 双因 2（T1+T2、T8+T11）· 真改代码 1（内容字节变化，期望 `null` 且 `residue_bytes > 0`）· 混合 1（T1+T3+T9 + 一处真改）· T10 专项 1（`c13-signature`，签名段存在性差异）· 未知扩展头 1（`c14-pax-unknown`，PAX 未知 keyword）。**T9/T10/T11/T12 由哪组样张覆盖必须在 golden README 里点名**，不允许有类别没有样张。
- **可重建性判据**：两台机器（不同路径、不同时区）跑同一生成器，产出样张的每个文件 sha256 与全部 golden 文件**逐字节一致**；生成器不得读系统时钟、不得读环境变量（除显式 `--out` / `--golden` / `--only`）。归档里的时间戳只来自 `{0, 1700000000, 1700003600}`；生成器用 `time.Unix(秒, 0)` 之类**常量构造**也不允许——`tools/` 同样在 `time` 黑名单内，tar 头由生成器手写。
- **反例也要有**：必须存在"未知扩展头"样张（`c14-pax-unknown`，断言 `raw_header_sha256` 出现且与手工重算一致）与"读不了的输入"样张（`data/fixtures/broken/truncated.tar`，断言 `exit 1`），防止工具靠"看不懂就跳过/报一致"骗过验收。前者计入 14 组分类矩阵（归 T7），后者不计入。
- **golden 一物一证**：每个 golden 文件旁边记录**生成命令**与**手工复现命令**（`cmp` 计数、`sha256sum` 重算），不得只放期望值。

## 四、数值口径（唯一权威）

### 4.1 闭合分类法 T1–T12

| rule | 名称 | 触发条件（精确） | 归一化 op 语义（§4.2 引用） |
|---|---|---|---|
| T1 | 时间戳 | 任一条目的 mtime 字段不同（tar `mtime` / zip `date_time` / gzip 成员 `mtime`） | 所有 mtime 置 0 |
| T2 | 属主 | `uid`/`gid`/`uname`/`gname` 任一不同 | `uid=gid=0`，`uname=gname=""` |
| T3 | 权限 | `mode` 或 zip `external_attr` 的权限位不同 | `mode` 权限位置 0644（目录 0755） |
| T4 | 路径前缀 | 条目 relpath 的公共根前缀不同（如 `build-a/usr/…` vs `build-b/usr/…`） | 剥离公共根前缀到首个稳定锚点 |
| T5 | 条目顺序 | 两侧条目集合与哈希相同，仅归档内存储顺序不同 | 按 relpath 字节序重排 |
| T6 | 压缩器参数 | gzip `xfl`/`os` 或压缩级别线索不同 | 解压后以固定参数重压（`gzip -9n`） |
| T7 | 归档头字段 | 归档级版本/格式/尾部字段不同 | 重写头为标准最小头 |
| T8 | 工具链串 | `.comment`、编译器版本串、DWARF `producer` 不同 | 该字段置空串（长度归零） |
| T9 | 调试符号与 build-id | `build-id` 不同，或 DWARF 段存在性不同 | 移除 `.note.gnu.build-id` 与 DWARF 段（保持节表自洽） |
| T10 | 签名 blob | 签名段存在性或内容不同 | 移除签名段 |
| T11 | 构建路径与环境 | `comp_dir`、内嵌绝对路径、locale/环境串不同 | 路径归一为 `<build-root>`（固定占位，写入 §4.7） |
| T12 | 未解释 | 归不进 T1–T11 的任何差异 | **无 op**（T12 不可归一化；出现即 `residue_bytes > 0`） |

**闭合性要求**：任何差异必须恰好落入一个 Tn；同一差异不得同时命中两个 rule（若命中，优先级按 T1→T12 顺序取首个，并在报告 `notes` 记录 `rule_overlap`）。

### 4.2 归一化 op 纪律

- 每个 Tn（T1–T11）对应一个**纯函数**：同输入必同输出，不读时钟、不读环境、不使用随机数。
- **幂等**：`normalize_op(Tn, normalize_op(Tn, x)) == normalize_op(Tn, x)`，逐 op 断言。
- **可组合**：集合 S 的归一化 = 对 S 内所有 op 按 **rule id 升序**依次施加；顺序固定，避免交换性假设。
  "升序"指数值序（`T1 < T2 < … < T11`），**不是**字符串序（字符串序会把 `T10` 排到 `T2` 前面）。
- **不落盘**：归一化在内存流中完成，不写临时文件；输入目录只读。
- **派生字段同步**：`tar.size` 是条目内容长度的派生值。任何会重写内容字节的 op（T8–T11）之后，
  `tar.size` 同步为 `len(归一化后的内容字节)`（对应 §4.10 的十进制口径）；它**不单独参与分类**
  （删签名段后 size 变小是 T10 的结果，不是新的 T12）。

### 4.3 `raw_header_sha256` 覆盖范围

对每个条目，`raw_header_sha256 = sha256(解析器已消费字段之外的原始头部字节)`，覆盖：

1. tar：512 字节块的 `name`、`linkname`、`magic`、`version`、`typeflag`、`uname`、`gname`、`devmajor`、`devminor`、`prefix` 之后的全部保留区与扩展头（PAX/GNU）原始字节；
2. zip：中央目录记录与本地文件头中未被 `facts` 抽取的字段（含 extra field 原文）；
3. gzip：成员头中除 `mtime`/`xfl`/`os` 外的 FEXTRA/FNAME/FCOMMENT/FHCRC 原文；
4. ELF/PE/Mach-O：不属于已抽取 `key` 的头部扩展区。

**硬纪律**：解析器遇到不认识的头部形态时，**必须**使其进入 `raw_header_sha256` 并输出该 key；**禁止**"不认识就跳过"。没有该 key 的条目 = 缺陷。

**字节构造口径（M2 落地，逐条可手工重算）**：

| 容器 | `raw_header_sha256` 的输入字节（按序拼接） |
|---|---|
| tar 条目 | ① 本条目之前全部 PAX（`x`/`g`）与 GNU（`L`/`K`）扩展头的原文：512 字节块 ‖ 数据 ‖ 到块边界的填充；② 本条目头的保留区 `[345:512]`（`prefix` 字段结束处到块尾）。扩展头里的未知 keyword 天然被覆盖 |
| zip 条目 | 分量按序、逐个 `le32(len) ‖ bytes`：本地头 `[4:6]`（version needed）、`[6:8]`（flags）、`[14:26]`（crc ‖ 压缩后大小 ‖ 原始大小）、local extra；中央目录 `[8:10]`（flags）、`[16:28]`（crc ‖ 两个大小）、`[32:38]`（comment 长度 ‖ disk start ‖ internal attrs）、`[42:46]`（本地头偏移）、central extra、comment |
| gzip 容器（path=`""`） | FEXTRA（含 2 字节 XLEN）‖ FNAME（含结尾 NUL）‖ FCOMMENT（含结尾 NUL）‖ FHCRC（2 字节），按出现顺序原样拼接 |
| ELF（裸文件） | ELF 头（52/64 字节）‖ 程序头表原文 ‖ 节表原文 |
| PE（裸文件） | DOS 头 + DOS stub（含 Rich 区）到可选头结束（`e_lfanew + 24 + SizeOfOptionalHeader`） |
| Mach-O（裸文件） | 头（28/32 字节）‖ load commands 原文；fat 取第一个架构的 thin 头 |
| 其他裸文件 | 空字节（`sha256("")`）——文本文件没有头部，不是"跳过" |

**嵌套规则**：`raw_header_sha256` 只描述**条目自身的容器头**。tar 里的 ELF 条目，其 raw_header 是 tar 头；ELF 自身的头部差异由 `elf.*` 事实与条目 sha256 承担。目录装载（裸文件）才用文件格式头。

### 4.4 改名判定

仅当满足全部三条时判 `RENAMED`：① 左侧唯一 `REMOVED` 且右侧唯一 `ADDED`；② 两者 `sha256` 相等；③ 两侧该哈希的出现次数均为 1。任一不满足 → 分别记 `REMOVED`/`ADDED`。**不做相似度、不做模糊对齐、不做字节重排匹配。**

> **「唯一」的口径（M1 落地澄清）**：① 的"唯一"取**每哈希唯一**——该 `sha256` 在两侧各出现 1 次、且两侧各只有一条未匹配条目携带它；**不是**"全局只剩一条 `REMOVED` 与一条 `ADDED`"。后者会让"1 改名 + 1 新增"永远判不出改名（那是 1 条 `REMOVED` + 2 条 `ADDED`），与 `plan/03` §二 的示例输出冲突。两条边界的机器证据：`data/golden/align/basic.json`（改名 + 新增同时成立）与 `data/golden/align/dup-hash.json`（同哈希多处出现 → 不判改名）。

### 4.5 树摘要与最小原因集

- **条目字节流**（`entry_stream`，M3 落地；tree_digest 与 residue 共用）：

  ```
  entry_stream(e) = decimal(e.index) ‖ 0x00
                    ‖ concat_{key 升序}( key ‖ 0x00 ‖ value ‖ 0x0A )
                    ‖ 0x00 ‖ 内容字节
  ```

  `e.index` 是归档内序号（目录输入 = `-1`，见 §4.10 的 `Raw.Index`）；它进入摘要，
  因此 T5 的归一化（序号归零）才有可观测效果。`key`/`value` 是 §4.10 的 facts；
  没有 facts 的条目在第一个 `0x00` 之后直接是内容字节。
- `tree_digest(S) = sha256( concat_{relpath 升序} ( relpath ‖ 0x00 ‖ sha256( entry_stream(normalize_S(e)) ) ‖ 0x0A ) )`，
  `relpath` 用 UTF-8 字节序升序；同 relpath 再用 `index`、`entry_stream` 破平（确定性）。
  分隔符写死为上式的 `0x00` 与 `0x0A`。**唯一实现** = `internal/normalize.TreeDigest`。
- `residue_bytes` = 归一化（S = 全集 T1–T11）后两侧条目的 `entry_stream` **实际不同字节数之和**
  （逐条目按字节比对：共同位置的差异逐字节计数，长度差也计入；不用长度差近似、不由分类推导）。
  T12 出现 ⇒ `residue_bytes > 0`。
- `minimal_cause_set` = 使 `tree_digest(S_left) == tree_digest(S_right)` 成立的**基数最小**的 S；并列时取 rule id 序列**字典序最小**者。
- 搜索上界 `|S| ≤ 12`（即 T1–T12 全集的基数）；候选枚举顺序：先按基数升序，同基数按字典序。**不做启发式剪枝、不做随机化**（会破坏最小性与可复算性）。
- 无解 → `minimal_cause_set = null`，且**必须** `residue_bytes > 0`（两者同时成立才算合法输出）。
- attribute 输出的 `tree_digest.normalized_all` = **左侧**在 S = T1–T11 下的摘要
  （右侧同口径摘要由 `residue_bytes == 0` 与 golden 共同钉住）。

### 4.6 退出码

| 码 | 含义 | 判据 |
|---|---|---|
| 0 | 完全一致 | `residue_bytes == 0` 且 `S == ∅`；或 `--budget` 白名单覆盖了全部差异且归一化后一致 |
| 1 | 用法错误 / 输入不可解析 | 参数非法、路径不存在、归档损坏/格式不支持；**不得与"一致"混淆** |
| 2 | 仅元数据差异 | 全部差异命中 T1–T11，且归一化后 `residue_bytes == 0` |
| 3 | 有残留 | `residue_bytes > 0`，或出现 T12 |

`--budget=Tn,Tm` 只影响"归一化后一致"时的 0/2 判定：实际差异类别 ⊆ 白名单 → 0；否则维持 2。**白名单不得把 3 降为 2 或 0。**

### 4.7 固定占位值

| 占位 | 值 |
|---|---|
| 构建根路径替换 | `<build-root>` |
| 归一化属主 | `uid=0`、`gid=0`、`uname=""`、`gname=""` |
| 归一化 mtime | `0` |
| 归一化权限 | 文件 `0644`，目录 `0755` |
| 重压参数 | `gzip -9n`（不写文件名、不写 mtime） |
| 合成样张时间戳表 | `{0, 1700000000, 1700003600}` |

### 4.8 报告键序（固定）

`schema_version` → `tool_version` → `left` → `right` → `counts` → `renamed` → `added` → `removed` → `residue_bytes` → `minimal_cause_set` → `residual_items` → `notes`。`left`/`right` 各含 `basename` 与 `sha256`（**不写绝对路径**）。`counts` 键序固定为 T1…T12 后接 `total`。

### 4.9 条目常量与对齐语义（M1 落地）

| 项 | 值 / 规则 |
|---|---|
| 目录条目的 `sha256` | `sha256("")` = `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`（`kind=dir` 无内容） |
| 软链条目的 `sha256` | 链接目标字节的 sha256（tar `linkname` / zip 内容 / 目录 `readlink`，三处同口径） |
| `--max-entry-bytes` 默认 | `268435456`（256 MiB）；超限 → exit 1 并**指名条目** |
| 同 relpath、同形态 | align 记 `SAME`。**结构对齐 ≠ 内容一致**：内容差异由 attribute 归因（T12 / `residue_bytes`），不在 align 层重复计成增删 |
| 同 relpath、异形态 | 两侧各记一条：左侧 `REMOVED`、右侧 `ADDED`；不判改名 |
| `RENAMED` 行 | `relpath` = 右侧（新）路径，`renamed_from` = 左侧（旧）路径；`counts.RENAMED` 对一条改名计 1 |
| relpath 规范化 | `\` 视为分隔符 → 折叠重复 `/` → 去前导 `./` → 去尾部 `/`；含 `..` 段或绝对路径 → exit 1（不静默改写） |
| 归档内重复 relpath | 取最后一条（与解包语义一致）；装载器不静默丢弃文件 |

### 4.10 facts 的取值格式（M2 落地）

`facts` 输出是 `(path, key, value)` 扁平记录，按 `(path, key)` 字节序排序；**容器级事实的 `path` 为空串 `""`**（表示容器自身，不伪造文件名）。

| 项 | 口径 |
|---|---|
| 数字 | 十进制字符串（`tar.mtime`、`tar.size`、`elf.section.*.offset/size`、`pe.timestamp`、`pe.checksum`） |
| `tar.mode` | tar 头 `mode` 字段的原值（权限位，八进制解析后写十进制；`0644` → `"420"`） |
| `tar.typeflag` | 该字节的十进制值（`'0'` → `"48"`、`'5'` → `"53"`、`'2'` → `"50"`） |
| `zip.mtime` | DOS `date_time` 格式化为 `YYYYMMDDHHMMSS`（UTC，无时区换算；零值写 `"0"`） |
| `zip.external_attr` | 32 位值原样写十进制（`0644<<16` → `"27525120"`） |
| `zip.create_system` | 中央目录 version-made-by 的高字节（**容器级**，path=`""`；取第一条记录） |
| `gzip.mtime` / `gzip.xfl` / `gzip.os` | 成员头的 4/1/1 字节原值（**容器级**，path=`""`）；`mtime` 按无符号 32 位 |
| ELF 节 key | `elf.section.<name>.<field>`，`<name>` 原样拼入（含前导点）：`.text` → `elf.section..text.offset` / `.size` |
| 二进制串 | lowercase hex（`elf.build_id`、`pe.rich.digest`、`macho.uuid`） |
| 布尔 | `"true"` / `"false"`（`pe.rich.present`） |
| `macho.minos` / `macho.sdk` | nibble 版本号 → `"13.2.0"` |
| `macho.tools` | `LC_BUILD_VERSION` 的工具表，`id:version` 以 `,` 连接（`clang:15.0.0,ld:711.0.0`）；无工具表则不出该 key |
| 字符串 | 原文；含非 UTF-8 字节时写 `hex:` + hex |
| 目录条目 | 不参与 facts（plan/03 §二.5）；`Extract` 对其返回空 |
| 多成员 gzip | v0 只解析**第一个**成员头（容器级 facts 与 gzip raw_header 均取首成员）；后续成员按 deflate 流解压，不重复记账 |
