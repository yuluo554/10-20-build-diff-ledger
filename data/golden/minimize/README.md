# minimize golden

> `c01-mtime.json` … `c14-pax-unknown.json` 是 M4 最小原因集搜索的**逐字节**期望输出，
> 覆盖 14 组样张。口径唯一权威：`data/README.md` §4.5（`tree_digest` / 最小原因集）
> 与 §4.6（退出码）；搜索算法见 `plan/03` §五。
>
> 每个文件钉住五件事：`minimal_cause_set`（`null` = 无解）、`found`、
> `residue_bytes`（走真实 `attribute` 路径，不抄 `expect.json`）、
> `digest_set`（实际代入摘要的规则集合）、`digest_left` / `digest_right`。

## 字段口径

| 字段 | 含义 |
|---|---|
| `minimal_cause_set` | 首个命中的规则集合；`[]` = 两侧完全一致，`null` = 无解 |
| `found` | `true` 当且仅当 `minimal_cause_set` 非 `null` |
| `residue_bytes` | S = T1–T11 归一化后 `entry_stream` 的实际不同字节数（§4.5） |
| `digest_set` | 算摘要时实际代入的集合：有解 = `minimal_cause_set`；无解 = T1–T11 |
| `digest_left` / `digest_right` | 在 `digest_set` 下两侧的 `tree_digest`（§4.5）。有解必须相等，无解必须不等 |

`digest_set` 无解时取 T1–T11，是为了让"无解"有**可观察证据**：不是搜索没跑完，
而是把全部可归一化的 op 都施加后两侧摘要仍然不同。

## 生成命令

```bash
go test ./internal/cli -run TestMinimizeGolden -update
```

该测试走与生产相同的路径：`artifact.LoadDocument` → `facts.Extract` →
`attribute.Entries`（公共根前缀 / 归档内序号）→ `minimize.Search` →
`normalize.TreeDigest`，再用 `report.MarshalValue`（仓库唯一 JSON 编码器）落盘。

## 逐字节校验

```bash
go test ./internal/cli -run 'TestMinimizeGolden|TestMinimizeFixtureMatrix|TestMinimizeDoDCases' -count=1
go test ./internal/minimize -count=1
```

`TestMinimizeGolden` 逐字节比对 14 份 golden，并断言同输入双跑一致。
`TestMinimizeFixtureMatrix` 由 `data/fixtures/*/expect.json` 的 `minimal_cause_set` 驱动，
`TestMinimizeDoDCases` 钉住 `plan/05` M4 DoD 点名的四条（见下表）。

## 结果总表

| 样张 | `minimal_cause_set` | 基数 | `residue_bytes` | 观察 |
|---|---|---|---|---|
| `c01-mtime` | `[T1]` | 1 | 0 | 仅 mtime |
| `c02-owner` | `[T2]` | 1 | 0 | 仅属主 |
| `c03-mode` | `[T3]` | 1 | 0 | 仅权限 |
| `c04-prefix` | `[T4]` | 1 | 0 | 仅根前缀 |
| `c05-order` | `[T5]` | 1 | 0 | 仅条目顺序 |
| `c06-gzip-opts` | `[T6]` | 1 | 0 | 仅 gzip 成员头 |
| `c07-archive-hdr` | `[T7]` | 1 | 0 | 仅 zip create_system |
| `c08-toolchain` | `[T8]` | 1 | 0 | 仅 `.comment` |
| `c09-mtime-owner` | `[T1,T2]` | **2** | 0 | DoD 显式非超集 |
| `c10-toolchain-path` | `[T8,T11]` | 2 | 0 | `.comment` + comp_dir |
| `c11-real-change` | `null` | — | **1** | DoD：无解必须 `residue>0` |
| `c12-mixed` | `null` | — | **1** | 残留只在 `synthpkg/data/table.csv` |
| `c13-signature` | `[T10]` | 1 | 0 | 签名段存在性 |
| `c14-pax-unknown` | `[T7]` | 1 | 0 | 未知 PAX keyword → raw_header |

## 手工复算

### 1) 有解：把 S 代入摘要

`digest_left == digest_right` 是"该集合抹平了全部差异"的直接证据。逐条复算：

```bash
# 断言 expect.json 的集合施加后两侧摘要相等；去掉集合中任一规则后必须不等
go test ./internal/cli -run TestExpectedCauseSetEqualizesDigests -count=1
# 断言搜索的首个命中就是上表的集合（含 c09 的 len==2）
go test ./internal/cli -run 'TestMinimizeFixtureMatrix|TestMinimizeDoDCases' -count=1
```

`TestExpectedCauseSetEqualizesDigests` 已包含"去一规则后不等"的最小性检查；
`minimize.Search` 的"首个命中即最小"由 `TestSearchIsExhaustiveWithoutPruning`
（无解时每侧恰好 4096 次调用、无重复）与
`TestSearchPrefersSmallerCardinality`（{T2} 优先于 {T1,T2}）共同钉住。

### 2) `residue_bytes`：手工 `cmp`

```bash
mkdir -p /tmp/c12-l /tmp/c12-r
tar -xf data/fixtures/c12-mixed/pair/left/synthpkg-0.1.0.tar  -C /tmp/c12-l
tar -xf data/fixtures/c12-mixed/pair/right/synthpkg-0.1.0.tar -C /tmp/c12-r
cmp -l /tmp/c12-l/synthpkg/data/table.csv /tmp/c12-r/synthpkg/data/table.csv | wc -l   # = 1
```

`c12` 的 `digest_set` 是 T1–T11：若 T1/T3/T9 没有把 mtime、权限、build-id/DWARF
的差异抹平，`residue_bytes` 会大于 1，`TestMinimizeDoDCases` 会红。
`c11` 同理（`tar.gz`，残留 = `table.csv` 的 1 字节）。

### 3) 搜索代价（不写进断言）

搜索上界 = 2^12 = 4096 个候选（§4.5）。有解样张在基数 ≤2 时即命中；
`c11`/`c12` 无解，必须跑满 4096 个候选 × 两侧摘要。实测耗时回填在 `plan/00` §八，
本目录不写耗时断言（机器相关的数字不得进断言）。

## 边界

- `T12` 没有归一化 op（§4.1），因此**不可能**成为最小集成员：
  若 `S ∪ {T12}` 成立，则 `S` 也成立且更小，首个命中不会带 `T12`
  （`TestSearchNeverReturnsT12`）。
- 无解（`null`）与完全一致（`[]`）在 JSON 里是不同的值，调用方必须区分：
  `Search` 对"完全一致"返回**非 nil 空切片**（`TestSearchIdenticalReturnsEmptySet`）。
- 本目录只覆盖 14 组合成样张；不声称对真实产物成立（`plan/06` R4）。
