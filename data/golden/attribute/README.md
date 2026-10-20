# attribute golden

> `c01-mtime.json` … `c14-pax-unknown.json` 是**归因层**的**逐字节**期望输出，覆盖 14 组样张。
> 口径唯一权威：`data/README.md` §4.1/§4.2/§4.5/§4.6。
> 每个文件钉住四件事：`counts`（逐类计数）、`diffs`（逐差异点）、`residue_bytes`、
> `tree_digest`（left / right / normalized_all）。
>
> **M5 起 `bdiff compare` 的 stdout 是 `report.json`**（报告形态见 `data/golden/report/`）；
> 本目录钉的是它**前半段**的中间结果，由 `TestAttributeGolden` 直调 `buildAttributeResult` 取
> （与 `buildReport` 同一函数，不是测试专用复刻）。

## 生成命令

```bash
go test ./internal/cli -run TestAttributeGolden -update
```

> ⚠️ M5 起 `compare` 已不再输出归因 JSON，**不能用 `go run ./cmd/bdiff compare > golden` 生成**。
> `-update` 只写 golden、不改变断言。

## 逐字节校验

```bash
go test ./internal/cli -run TestAttributeGolden -count=1   # golden + 双跑逐字节
go test ./internal/cli -run 'TestFixtureMatrix|TestExpectedCauseSet|TestEachRuleOp' -count=1
```

`TestFixtureMatrix` 由 `data/fixtures/*/expect.json` 驱动，断言四件事：
`counts_positive` 全部 `>0`、出现的类别都在 `counts_allowed` 内、`residue_bytes` 相等、
退出码相等（并额外断言 `T12 ⇒ residue_bytes > 0`）。
`TestExpectedCauseSetEqualizesDigests` 用 `expect.json` 的 `minimal_cause_set` 复算树摘要
（非 null → 施加后两侧相等；null → 全集 T1–T11 后仍不等）。
`TestEachRuleOpIsIdempotent` 为 T1–T11 各取一组**真实触发它**的样张，断言
`normalize(normalize(x)) == normalize(x)` 且 op 确实改动了输入。

## 手工复现（不依赖本工具）

### 1) 计数：看容器头字段

| 样张 | 注入 | 手工观察 |
|---|---|---|
| `c01` | T1 | `tar -tvf .../left/synthpkg-0.1.0.tar` 与右侧比 mtime 列 |
| `c02` | T2 | 同上，比 owner 列 |
| `c03` | T3 | 同上，比权限列（`-rw-r--r--` vs `-rwxr-xr-x`） |
| `c04` | T4 | `tar -tf` 左右：条目分别以 `build-a/` / `build-b/` 开头 |
| `c05` | T5 | `tar -tf` 左右：同一组条目，顺序相反 |
| `c06` | T6 | `od -An -tx1 -N10 .../c06-gzip-opts/pair/left/synthpkg-0.1.0.tar.gz`：第 9/10 字节 = xfl/os |
| `c07` | T7 | `unzip -l` 两侧条目一致；差异在中央目录 version-made-by 的高字节 |
| `c08` | T8 | `strings -a <tar> | grep GCC`：`13.2.0` vs `13.3.0` |
| `c09` | T1+T2 | 同时看 mtime 与 owner 两列 |
| `c10` | T8+T11 | `strings -a <tar> | grep /build/`：`/build/synthpkg` vs `/build/right` |
| `c11` | T12 | 见下方 `cmp -l` |
| `c12` | T1+T3+T9+T12 | mtime/权限列 + build-id 段内容 + `cmp -l` |
| `c13` | T10 | `tar -xOf ... | grep -c '\.sig'`：左 1、右 0 |
| `c14` | T7 | `tar --xattrs -tvf`：左含 PAX `SCHILY.xattr.user.marker=fixture-unknown` |

### 2) `residue_bytes`：逐字节 `cmp`

只有 c11/c12 有残留，期望值都来自生成器的手工构造（`flipOneByte` 只翻最后一个字节）：

```bash
rm -rf /tmp/c11-l /tmp/c11-r && mkdir -p /tmp/c11-l /tmp/c11-r
tar -xzf data/fixtures/c11-real-change/pair/left/synthpkg-0.1.0.tar.gz  -C /tmp/c11-l
tar -xzf data/fixtures/c11-real-change/pair/right/synthpkg-0.1.0.tar.gz -C /tmp/c11-r
cmp -l /tmp/c11-l/synthpkg/data/table.csv /tmp/c11-r/synthpkg/data/table.csv | wc -l   # = 1

mkdir -p /tmp/c12-l /tmp/c12-r
tar -xf data/fixtures/c12-mixed/pair/left/synthpkg-0.1.0.tar  -C /tmp/c12-l
tar -xf data/fixtures/c12-mixed/pair/right/synthpkg-0.1.0.tar -C /tmp/c12-r
cmp -l /tmp/c12-l/synthpkg/data/table.csv /tmp/c12-r/synthpkg/data/table.csv | wc -l   # = 1
```

其余 12 组 `residue_bytes = 0`：按上表注入的差异都只改变容器头/ELF 元数据，
解包后的**内容**逐字节相同（可用 `sha256sum` 对比两侧解包树）。
`c08/c10/c13` 的 ELF 内容差异由 T8/T9/T10 的节表重写抹平，重写后两侧 ELF 字节相等
（`TestEachRuleOpIsIdempotent` 与 `TestExpectedCauseSetEqualizesDigests` 复算）。

### 3) `tree_digest.left/right`：独立 Python 复算

下面的脚本**不复用本工具的摘要实现**，只读取 `bdiff facts`（M2 的公开输出）与标准库
`tarfile`/`zipfile`，按 `data/README.md` §4.5 的 `entry_stream` 与拼接式重算：

```python
import hashlib, json, subprocess, tarfile, zipfile

def entry_stream(index, facts, content):
    b = str(index).encode() + b"\x00"
    for k in sorted(facts):
        b += k.encode() + b"\x00" + facts[k].encode() + b"\x0a"
    return b + b"\x00" + content

def tree_digest(path):
    out = subprocess.run(["go", "run", "./cmd/bdiff", "facts", "--artifact", path],
                         capture_output=True, check=True).stdout
    fm = {}
    for f in json.loads(out)["facts"]:
        fm.setdefault(f["path"], {})[f["key"]] = f["value"]
    entries = []
    if tarfile.is_tarfile(path):
        with tarfile.open(path) as t:
            for i, m in enumerate(t.getmembers()):
                data = b"" if m.isdir() else (m.linkname.encode() if m.issym()
                                              else t.extractfile(m).read())
                name = m.name.rstrip("/")
                entries.append((name, entry_stream(i, fm.get(name, {}), data)))
    elif zipfile.is_zipfile(path):
        with zipfile.ZipFile(path) as z:
            for i, zi in enumerate(z.infolist()):
                data = b"" if zi.is_dir() else z.read(zi)
                name = zi.filename.rstrip("/")
                entries.append((name, entry_stream(i, fm.get(name, {}), data)))
    if "" in fm:                       # 容器级事实（gzip.* / zip.create_system）
        entries.append(("", entry_stream(-1, fm[""], b"")))
    h = hashlib.sha256()
    for name, s in sorted(entries):
        h.update(name.encode() + b"\x00"
                 + hashlib.sha256(s).hexdigest().encode() + b"\x0a")
    return h.hexdigest()

print(tree_digest("data/fixtures/c01-mtime/pair/left/synthpkg-0.1.0.tar"))
# 与 c01-mtime.json 的 tree_digest.left 相同
```

**边界（如实登记）**：`normalized_all` 的独立复算需要把 T1–T11 的 op（含 T9/T10 删除 ELF
节段后的节表重建）在第二个实现里重写一遍。本 README 不假装一条 shell 命令就能复算它；
它由 golden 逐字节钉住，并由上面两组测试在**同一口径**下复算（`TestExpectedCauseSetEqualizesDigests`
覆盖 c01/c02/c03/c04/c05/c06/c07/c08/c09/c10/c13/c14 的注入集合，c11/c12 断言无解）。
