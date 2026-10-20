# bdiff 差异归因账本

## 输入

| 侧 | basename | sha256 |
|---|---|---|
| left | `synthpkg-0.1.0.tar` | `f16f0a5a84bb2c525bd0390befbe4242cf3ee186ab4fbe6fe0b6ff98d5a9ba23` |
| right | `synthpkg-0.1.0.tar` | `41c8514223f14569c3676acc6ce0e2907d2e25d60b33e560e68b8f0574bd454d` |

## 分类计数

| rule | 名称 | 计数 |
|---|---|---|
| T1 | 时间戳 | 0 |
| T2 | 属主 | 0 |
| T3 | 权限 | 0 |
| T4 | 路径前缀 | 1 |
| T5 | 条目顺序 | 0 |
| T6 | 压缩器参数 | 0 |
| T7 | 归档头字段 | 0 |
| T8 | 工具链串 | 0 |
| T9 | 调试符号与 build-id | 0 |
| T10 | 签名 blob | 0 |
| T11 | 构建路径与环境 | 0 |
| T12 | 未解释 | 0 |
| **total** | — | **1** |

## 改名 / 新增 / 删除

### 改名

| from | to |
|---|---|
| `build-a/synthpkg/METADATA` | `build-b/synthpkg/METADATA` |
| `build-a/synthpkg/README.md` | `build-b/synthpkg/README.md` |
| `build-a/synthpkg/__init__.py` | `build-b/synthpkg/__init__.py` |
| `build-a/synthpkg/core.py` | `build-b/synthpkg/core.py` |
| `build-a/synthpkg/data/table.csv` | `build-b/synthpkg/data/table.csv` |

### 新增

- `build-b/synthpkg`
- `build-b/synthpkg/data`

### 删除

- `build-a/synthpkg`
- `build-a/synthpkg/data`

## 残留

- `residue_bytes`: 0
- 残留条目：（无）

## 最小原因集

- `minimal_cause_set`: `T4`（路径前缀）

## 判定

- 结论：**仅元数据差异**（全部差异已被分类规则解释）。
