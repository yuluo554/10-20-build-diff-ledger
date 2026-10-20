# data/golden/facts · facts 输出的逐字节 golden

> 冻结件：由 `bdiff facts` 产出，**禁止手改**。改抽取逻辑 → 重新生成并同提交更新本目录。
> 输入是 `data/fixtures/` 下由 `tools/gen_fixtures` 程序化合成的样张（不含任何真实发行版产物）。

## 文件与覆盖

| 文件 | 输入 | 覆盖的 key 家族 |
|---|---|---|
| `c01-tar-left.json` | `c01-mtime/pair/left/synthpkg-0.1.0.tar` | `tar.*`、`raw_header_sha256`（ustar 保留区） |
| `c06-gzip-left.json` | `c06-gzip-opts/pair/left/synthpkg-0.1.0.tar.gz` | `gzip.*`（容器级，path=`""`）、`tar.*` |
| `c07-zip-left.json` | `c07-archive-hdr/pair/left/synthpkg-0.1.0.zip` | `zip.*`（含容器级 `zip.create_system`）、`raw_header_sha256`（本地头/中央目录未抽取字段） |
| `c08-elf-left.json` | `c08-toolchain/pair/left/synthpkg-0.1.0.tar` | `elf.*`（`.comment` / build-id / DWARF / 固定节的 offset+size） |
| `c14-pax-left.json` | `c14-pax-unknown/pair/left/synthpkg-0.1.0.tar` | `raw_header_sha256` 的**未知 PAX keyword** 反例（T7） |

## 生成命令（改抽取逻辑后整目录重生成）

```bash
go run ./cmd/bdiff facts --artifact data/fixtures/c01-mtime/pair/left/synthpkg-0.1.0.tar       > data/golden/facts/c01-tar-left.json
go run ./cmd/bdiff facts --artifact data/fixtures/c06-gzip-opts/pair/left/synthpkg-0.1.0.tar.gz > data/golden/facts/c06-gzip-left.json
go run ./cmd/bdiff facts --artifact data/fixtures/c07-archive-hdr/pair/left/synthpkg-0.1.0.zip  > data/golden/facts/c07-zip-left.json
go run ./cmd/bdiff facts --artifact data/fixtures/c08-toolchain/pair/left/synthpkg-0.1.0.tar     > data/golden/facts/c08-elf-left.json
go run ./cmd/bdiff facts --artifact data/fixtures/c14-pax-unknown/pair/left/synthpkg-0.1.0.tar   > data/golden/facts/c14-pax-left.json
```

逐字节断言：`go test ./internal/cli -run TestFactsGoldenMatchesCLI`。

## 手工复现（不经过 bdiff 的独立路径）

### 1. tar 条目头（`tar.*`）

```bash
cd data/fixtures/c01-mtime/pair/left
tar -tvf synthpkg-0.1.0.tar                       # mtime / owner / mode / size
od -An -c -j 100 -N 64 synthpkg-0.1.0.tar         # 头部字段：mode uid gid size mtime
```

### 2. tar 的 `raw_header_sha256`（保留区 + PAX 扩展头原文）

```bash
py -3.12 - <<'PY'
import hashlib
b = open('data/fixtures/c14-pax-unknown/pair/left/synthpkg-0.1.0.tar','rb').read()
pos = 0
while b[pos+156] != ord('x'):
    pos += 512
size = int(b[pos+124:pos+136].split(b'\0')[0], 8)
ext_end = pos + 512 + size + (-size % 512)        # 扩展头：块 + 数据 + 填充
entry = ext_end
raw = b[pos:ext_end] + b[entry+345:entry+512]     # 扩展头原文 + 条目头保留区
print(hashlib.sha256(raw).hexdigest())
PY
```

输出必须等于 `c14-pax-left.json` 里 `synthpkg/METADATA` 的 `raw_header_sha256`。
（`internal/facts` 的 `TestRawHeaderUnknownPAXIsHandRecomputed` 用 Go 独立重算同一段字节。）

### 3. gzip 容器头（`gzip.*`）

```bash
od -An -tu4 -j4 -N4 data/fixtures/c06-gzip-opts/pair/left/synthpkg-0.1.0.tar.gz   # mtime
od -An -tu1 -j8 -N2 data/fixtures/c06-gzip-opts/pair/left/synthpkg-0.1.0.tar.gz   # xfl os
```

### 4. zip 头（`zip.*`）

```bash
unzip -v data/fixtures/c07-archive-hdr/pair/left/synthpkg-0.1.0.zip   # method / 时间
zipinfo -v data/fixtures/c07-archive-hdr/pair/left/synthpkg-0.1.0.zip | head -40
```
`zip.mtime` 的取值格式是 `YYYYMMDDHHMMSS`（UTC，无时区换算），见 `data/README.md` §4.10。

### 5. ELF（`elf.*`）

```bash
tar -xOf data/fixtures/c08-toolchain/pair/left/synthpkg-0.1.0.tar synthpkg/bin/synthpkg > /tmp/synthpkg.elf
readelf -p .comment /tmp/synthpkg.elf                  # elf.comment
readelf -n /tmp/synthpkg.elf | grep -A1 'Build ID'     # elf.build_id
readelf --debug-dump=info /tmp/synthpkg.elf | grep -E 'DW_AT_(producer|comp_dir)'
readelf -S /tmp/synthpkg.elf                           # 各节 offset/size
```

`elf.section..<name>.offset` 的键名带节名里的前导点（`elf.section..text.offset`），见 `data/README.md` §4.10。
