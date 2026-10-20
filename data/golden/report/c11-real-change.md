# bdiff 差异归因账本

## 输入

| 侧 | basename | sha256 |
|---|---|---|
| left | `synthpkg-0.1.0.tar.gz` | `13a3d5834f93f0e20654bdbf74e5621fb1f2543be7b58d91d143e64bf32dcb22` |
| right | `synthpkg-0.1.0.tar.gz` | `10339821e71d9e719bef50dfb4299937fc37b9d7c48daf2d37b93cfb4dc6c05e` |

## 分类计数

| rule | 名称 | 计数 |
|---|---|---|
| T1 | 时间戳 | 0 |
| T2 | 属主 | 0 |
| T3 | 权限 | 0 |
| T4 | 路径前缀 | 0 |
| T5 | 条目顺序 | 0 |
| T6 | 压缩器参数 | 0 |
| T7 | 归档头字段 | 0 |
| T8 | 工具链串 | 0 |
| T9 | 调试符号与 build-id | 0 |
| T10 | 签名 blob | 0 |
| T11 | 构建路径与环境 | 0 |
| T12 | 未解释 | 1 |
| **total** | — | **1** |

## 改名 / 新增 / 删除

### 改名

（无）

### 新增

（无）

### 删除

（无）

## 残留

- `residue_bytes`: 1

| path | rule | 不同字节 |
|---|---|---|
| `synthpkg/data/table.csv` | T12 | 1 |

## 最小原因集

- `minimal_cause_set`: `null`（无解：T1–T11 全施加后两侧仍不同）

## 判定

- 结论：**有残留**（存在归一化后仍不同的字节）。
