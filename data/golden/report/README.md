# report golden

> `c01-mtime.json` … `c14-pax-unknown.json` 与同名 `.md` 是 `bdiff compare`（M5 形态：
> 固定键序报告）的**逐字节**期望输出，覆盖 14 组样张。口径唯一权威：
> `data/README.md` §4.5（最小原因集 / 残留）、§4.6（退出码）、§4.8（键序与字段口径）。
>
> 每份 `.json` 钉住十二个键的顺序与取值；每份 `.md` 钉住人读模板（标题 / 输入 /
> 逐类计数 / 改名·新增·删除 / 残留 / 最小原因集 / 结论行），且**无时间性文案、
> 无绝对路径、无主机名**，换行只用 `\n`。

## 字段口径（摘录；以 `data/README.md` §4.8 为准）

| 字段 | 说明 |
|---|---|
| `left` / `right` | 只有 `basename` 与 `sha256`；文件输入取文件字节摘要，目录输入取条目表摘要 |
| `counts` | T1…T12 后接 `total`，键序固定 |
| `renamed` / `added` / `removed` | 来自 align；`renamed` 按 `to` 升序，增删各自升序 |
| `residue_bytes` | S = T1–T11 归一化后 `entry_stream` 的实际不同字节数（§4.5） |
| `minimal_cause_set` | `[]` = 完全一致；`null` = 无解（此时 `residue_bytes > 0`） |
| `residual_items` | 残留条目 `{path, rule_id, bytes_different}`，按 `path` 升序；**和 == `residue_bytes`** |
| `notes` | 口径提示（`rule_overlap`），不参与判定 |

## 生成命令

```bash
go test ./internal/cli -run TestReportGolden -update
```

测试内走与生产相同的路径：`artifact.LoadDocument` → `facts.Extract` →
`attribute.Attribute` → `attribute.Entries` → `minimize.Search` → `align.Align` →
`report.Build` → `report.MarshalCanonical` / `report.RenderMarkdown`。

等价的 shell 生成方式（逐组）：

```bash
for c in data/fixtures/c*/; do
  name=$(basename "$c")
  left=$(ls "$c/pair/left"/*)
  right=$(ls "$c/pair/right"/*)
  go run ./cmd/bdiff compare --left "$left" --right "$right" --out "data/golden/report/.tmp-$name" >/dev/null
  mv "data/golden/report/.tmp-$name/report.json" "data/golden/report/$name.json"
  mv "data/golden/report/.tmp-$name/report.md"   "data/golden/report/$name.md"
  rmdir "data/golden/report/.tmp-$name"
done
```

## 逐字节校验

```bash
go test ./internal/cli -run 'TestReportGolden|TestReportShape|TestReportOutDir|TestReportCountsKeyOrder|TestReportMarkdownNoLeak' -count=1
go test ./internal/report -count=1
```

`TestReportGolden` 逐字节比对 14 × 2 份 golden，并断言同输入双跑一致。
`TestReportShape` 断言键序、`null`/`[]` 区分、`left/right` 不含路径分隔符、
以及 `sum(residual_items[].bytes_different) == residue_bytes`。
`TestReportOutDir` 断言 `--out` 落盘内容与 stdout 一致、可重复写、目录自建。
`TestReportMarkdownNoLeak` 断言 md 不含绝对路径 / 主机名 / 时间性文案。

## 结果总表

| 样张 | `minimal_cause_set` | `residue_bytes` | `residual_items` | 退出码 |
|---|---|---|---|---|
| `c01-mtime` | `[T1]` | 0 | `[]` | 2 |
| `c02-owner` | `[T2]` | 0 | `[]` | 2 |
| `c03-mode` | `[T3]` | 0 | `[]` | 2 |
| `c04-prefix` | `[T4]` | 0 | `[]` | 2 |
| `c05-order` | `[T5]` | 0 | `[]` | 2 |
| `c06-gzip-opts` | `[T6]` | 0 | `[]` | 2 |
| `c07-archive-hdr` | `[T7]` | 0 | `[]` | 2 |
| `c08-toolchain` | `[T8]` | 0 | `[]` | 2 |
| `c09-mtime-owner` | `[T1,T2]` | 0 | `[]` | 2 |
| `c10-toolchain-path` | `[T8,T11]` | 0 | `[]` | 2 |
| `c11-real-change` | `null` | 1 | `table.csv` 1 字节 | 3 |
| `c12-mixed` | `null` | 1 | `table.csv` 1 字节 | 3 |
| `c13-signature` | `[T10]` | 0 | `[]` | 2 |
| `c14-pax-unknown` | `[T7]` | 0 | `[]` | 2 |

完全一致（`minimal_cause_set = []`、退出码 0）不在 14 组里：它是"同一份产物自比"，
由 `TestReportShape` 与 `TestExitCodeMatrix` 用 `--left X --right X` 覆盖。

## 手工复算（不依赖本工具）

### 1) `left.sha256` / `right.sha256`

```bash
sha256sum data/fixtures/c01-mtime/pair/left/synthpkg-0.1.0.tar
# 与 c01-mtime.json 的 left.sha256 相同
sha256sum data/fixtures/c01-mtime/pair/right/synthpkg-0.1.0.tar
```

目录输入的摘要按 §4.8 的拼接式重算（标准库即可）。**注意目录条目也参与**
（`kind=dir` 的 `sha256` 取 §4.9 的空内容常量）：

```bash
# 在目录内执行：relpath ‖ 0x00 ‖ sha256(内容) ‖ 0x0A，再对整体取 sha256
cd data/fixtures/align-golden/rename-add/pair/left
find . -mindepth 1 | sed 's|^\./||' | sort | while read -r f; do
  if [ -d "$f" ]; then
    h=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
  else
    h=$(sha256sum "$f" | cut -d' ' -f1)
  fi
  printf '%s\0%s\n' "$f" "$h"
done | sha256sum
# → d9d2308199f7a5b5a3578d7c12d70c79e1acc6833f2f53167f67538fa85161a9
#   与 `bdiff compare --left <该目录> ...` 输出的 left.sha256 相同
```

### 2) `counts`：看容器头字段

逐类的手工观察方式与 `data/golden/attribute/README.md` §「手工复现」同一张表
（mtime/owner/mode 用 `tar -tvf`、gzip 头用 `od`、ELF 串用 `strings`、PAX 用 `tar --xattrs -tvf`）。

### 3) `residue_bytes` 与 `residual_items`

两者同源：`residue_bytes == sum(residual_items[].bytes_different)`。
手工口径仍是解包后逐字节 `cmp`：

```bash
mkdir -p /tmp/c12-l /tmp/c12-r
tar -xf data/fixtures/c12-mixed/pair/left/synthpkg-0.1.0.tar  -C /tmp/c12-l
tar -xf data/fixtures/c12-mixed/pair/right/synthpkg-0.1.0.tar -C /tmp/c12-r
cmp -l /tmp/c12-l/synthpkg/data/table.csv /tmp/c12-r/synthpkg/data/table.csv | wc -l   # = 1
```

（`entry_stream` 前缀在归一化后两侧相同，因此"内容字节的差异数"就是"不同字节数"；
`residual_items` 的 `path` 用剥离公共根前缀后的 relpath。）

### 4) `minimal_cause_set`

`data/golden/minimize/*.json` 已用 `digest_left == digest_right` 钉住"该集合抹平全部差异"；
`null` 的两组（c11/c12）同时有 `residue_bytes = 1`。复算命令：

```bash
go test ./internal/cli -run 'TestMinimizeFixtureMatrix|TestMinimizeDoDCases|TestExpectedCauseSet' -count=1
```

### 5) `report.md` 的确定性

```bash
go run ./cmd/bdiff compare --left A --right B --format md > m1.md
go run ./cmd/bdiff compare --left A --right B --format md > m2.md
cmp m1.md m2.md && echo "双跑逐字节一致"
grep -nE 'C:\\|/home/|/tmp/|生成于|运行时间|UTC' m1.md || echo "无路径/时间性泄漏"
```

## 边界

- 本目录只覆盖 14 组合成样张；不声称对真实产物成立（`plan/06` R4）。
- 目录输入的 `sha256` 是 M5 新定义（§4.8）；`align-golden` 的目录对可用它复核，
  但**不在**本目录的 14 份 golden 里（它们是归档对）。
- `notes` 当前 14 组都为空；`rule_overlap` 的样张级证据在 `internal/attribute` 的
  闭合性测试里，不靠本目录。
