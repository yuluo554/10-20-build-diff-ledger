# bdiff · 构建产物差异归因账本

> `diffoscope` 回答"哪里不同"，`reprotest` 回答"是否稳定"，`strip-nondeterminism` 回答"怎么抹平"，`buildinfo` 回答"怎么重建"——**没人回答"这是哪一类差异、有几处、归一化后还剩多少真差异、最少改哪几项就一致"**。
> `bdiff` 把两份**已产出**的构建产物转成可复算的分类账本 + 最小原因集 + CI 门禁。

`bdiff` 是**纯本地 CLI**：单二进制、零第三方依赖、**不跑构建、不联网、不写输入目录**。
输入只有两份现成的产物（目录 / tar / tar.gz / zip / whl / jar / OCI layer tar），输出是确定性报告与一个退出码。

**状态**：`v0.1.0-dev` · M3（attribute）。14 组合成样张、`bdiff align` / `bdiff facts` / 归因（`bdiff compare` 的 JSON 输出）已落地；minimize / report 从 M4 起逐棒落地。
当前状态的真值在 `plan/05-里程碑与验收.md` §一；发布状态只以 Release 页与 `plan/` 为准。

## 一、里程碑状态

| 里程碑 | 主题 | 状态 |
|---|---|---|
| M0 | 计划先行 + 方法论装订 + 可运行骨架 | ✅ 完成（2026-10-20） |
| M1 | 数据先行：样张生成器 + 13 组样张 + align 模块与 golden | ✅ 完成（2026-10-20） |
| M2 | facts 模块：tar/zip/gzip + ELF/PE/Mach-O + `raw_header_sha256` | ✅ 完成（2026-10-20） |
| M3 | attribute 模块：T1–T12 分类 + 幂等归一化 + 树摘要 + 残留字节 | ✅ 完成（2026-10-20） |
| M4 | minimize 模块：最小原因集穷举搜索 | ⬜ 未开始 |
| M5 | report 模块：固定键序报告 + 退出码矩阵 + `--budget` + `ci/example.yml` | ⬜ 未开始 |
| M6 | 干净环境验证 + 脱敏 + 发布（tag + Release 附产物 + 留档） | ⬜ 未开始 |

## 二、安装

需要 Go 1.22+（只构建需要；**运行不需要任何运行时**）：

```bash
git clone https://github.com/yuluo554/10-20-build-diff-ledger.git
cd 10-20-build-diff-ledger
CGO_ENABLED=0 go build -trimpath -o bdiff ./cmd/bdiff
./bdiff version
./bdiff selftest
```

发布版另提供 `dist/bdiff-<os>-<arch>`（见 Release 附件），下载后直接运行，无需 Go。

## 三、三个最小场景

> ⚠️ **当前是 M3**：`version` / `selftest` / `align` / `facts` 与 `compare` 的**归因 JSON + 退出码**已可用；
> `compare --out`（`report.json`/`report.md`）、`--format=md|both`、最小原因集与 `--fail-on` 随 M4–M5 补齐。
> 未落地的参数会**显式报错并 exit 1**，不会返回假结果。下面三条 `compare` 命令是**目标形态**。

### 场景一：只差 mtime

```bash
# 两侧 tar 只差 mtime
./bdiff compare --left old/pkg-0.1.0.tar.gz --right new/pkg-0.1.0.tar.gz --format both
# 期望：counts.T1 > 0，minimal_cause_set = ["T1"]，residue_bytes = 0，退出码 2
```

### 场景二：CI 门禁（放行时间戳与属主）

```bash
./bdiff compare --left a.tar --right b.tar --budget T1,T2 --out report/
# 归一化后一致 → 退出码 0；报告里仍逐条列出被放行的差异（放行 ≠ 消失）
```

`ci/example.yml` 是可直接粘进 GitHub Actions 的门禁示例。

### 场景三：真有内容差异

```bash
./bdiff compare --left a.whl --right b.whl
# 期望：counts.T12 > 0，minimal_cause_set = null，residue_bytes > 0，退出码 3
```

## 四、退出码

| 码 | 含义 | 你会看到的场景 |
|---|---|---|
| `0` | 完全一致（或被 `--budget` 放行且归一化后一致） | 同一次可复现构建的两个产物 |
| `1` | 用法错误 / 输入不可解析 | 路径写错、归档损坏、不支持的格式 |
| `2` | **仅元数据差异** | 只有 mtime/属主/权限这类字段不同 |
| `3` | **有残留** | 归一化后仍有真内容差异 |

口径的唯一权威是 `data/README.md` §4.6。**`1` 是补定的**：否则"读不了"会被当成"一致"（静默通过的经典漏洞）。

## 五、字段说明（归因输出）

M3 的 `bdiff compare` 输出归因 JSON：`counts` → `diffs` → `residue_bytes` → `tree_digest` → `notes`。
M5 起的 `report.json` 在其前面加 `schema_version`/`tool_version`/`left`/`right`，并补 `renamed`/`added`/`removed`/
`minimal_cause_set`/`residual_items`；键序口径见 `data/README.md` §4.8。

- `counts`：T1–T12 逐类计数 + `total`；分类法定义见 `data/README.md` §4.1。
- `diffs`：逐差异点 `(path, key, rule_id, left, right, bytes_delta)`，按 `(path, key, rule_id)` 排序。
- `residue_bytes`：归一化（S = T1–T11）后两侧条目字节流的**实际不同字节数**（§4.5）。
- `tree_digest`：`left`/`right`（S = ∅）与 `normalized_all`（左侧 S = T1–T11）。
- `notes`：口径性提示（如 §4.1 的 `rule_overlap`），不参与判定。
- M5 的 `minimal_cause_set`：使两侧归一化后**树摘要相等**的**基数最小**规则集合；无解为 `null`（且 `residue_bytes > 0`）。
- M5 的 `left` / `right`：只写 `basename` 与 `sha256`——**不写绝对路径**。

## 六、已知边界（v0）

1. **不跑构建**：只处理已产出的产物；"是否稳定"请用 `reprotest`。
2. **不做模糊二进制对齐**：改名判定**仅当两侧唯一剩余条目的 sha256 相等**；不做相似度、不做字节重排。
3. **不重实现 diffoscope 的 comparator**：不做逐行文本 diff、不渲染深层差异树。
4. **不做符号级语义 diff**：只比较头字段与节表事实。
5. **不含** `deb` / `rpm` / `apk`、不含 registry 拉取、不含 `--against-tag`（扩展项，不进首期判定路径）。
6. **不承诺自然语言"为什么"**：只承诺分类计数、残留字节、最小原因集这三件可复算的事。
7. **合成样张上的验证**：分类命中率在程序化生成的 14 组样张上验证；真实发行版产物的覆盖不作为承诺（`plan/06` R4）。
8. **跨平台**：本机只在 Windows + Git Bash 验证过；Linux/macOS 行为由 CI 矩阵验证（尚无远端，见 `plan/05` §三 U1/U2）。

## 七、隐私与确定性

- **零网络**：源码不 import `net` / `net/http`；CI 与布局门双重静态断言。
- **不读时钟**：不 import `time`；报告里没有"运行时刻"。
- **不读主机身份**：不 import `os/user`，不读 `os.Hostname`。
- **确定性**：同输入双跑 `report.json` / `report.md` 逐字节一致；条目按 `relpath` 的 UTF-8 字节序排序。

## 八、开发

```bash
node tools/check-sprint-layout.mjs   # 布局门（方法论装订），必须先绿
go vet ./... && go test ./...        # 全量测试
go run ./tools/gen_fixtures --out data/fixtures --check   # 样张重建校验（M1 起）

# 守门的变异验证：证明上面那些门不是"恒绿装饰"
node tools/verify-layout-mutations.mjs   # 布局门，11 组变异必须全红
bash tools/verify-go-guards.sh           # Go 侧守门，9 组变异必须全红
```

本仓库遵循 `ai-tool-project-sprint` 方法论：计划与验收口径全在 `plan/`（入口 `plan/00-README总览.md`），
`AGENTS.md` 是 agent 入口。新对话第一件事是跑上面那条布局门。

## 九、许可证

MIT（见 `LICENSE`）。
