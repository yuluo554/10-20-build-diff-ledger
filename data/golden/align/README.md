# align golden

> `basic.json` 与 `dup-hash.json` 是 `bdiff align` 的**逐字节**期望输出。
> 输入样张由 `tools/gen_fixtures` 程序化生成（见 `data/fixtures/align-golden/`）。

## 生成命令

```bash
go run ./tools/gen_fixtures --out data/fixtures            # 重建输入（含 align-golden/）
go run ./cmd/bdiff align \
  --left  data/fixtures/align-golden/rename-add/pair/left \
  --right data/fixtures/align-golden/rename-add/pair/right \
  > data/golden/align/basic.json

go run ./cmd/bdiff align \
  --left  data/fixtures/align-golden/dup-hash/pair/left \
  --right data/fixtures/align-golden/dup-hash/pair/right \
  > data/golden/align/dup-hash.json
```

## 手工复现（不依赖本工具）

`basic.json` 的期望集合可以只用人眼 + `sha256sum` 复算：

```bash
cd data/fixtures/align-golden/rename-add/pair
for side in left right; do
  echo "== $side"
  (cd $side && find . -type f | sort | while read f; do echo "$(sha256sum "$f")"; done)
done
```

应当看到：

| relpath | 左侧 | 右侧 | 结论 |
|---|---|---|---|
| `pkg/__init__.py` | 有 | 有 | SAME |
| `pkg/core.py` | 有 | 有 | SAME |
| `pkg/data.csv` | 有 | 有 | SAME |
| `pkg/old_name.py` | 有 | — | 右有 `pkg/new_name.py` 且 sha256 相同 → RENAMED |
| `pkg/legacy.txt` | 有 | — | 右侧无同哈希条目 → REMOVED |
| `pkg/extra.txt` | — | 有 | 左侧无同哈希条目 → ADDED |
| `pkg/`（目录） | 有 | 有 | SAME（目录的 sha256 = `sha256("")`） |

计数：`SAME=4, ADDED=1, REMOVED=1, RENAMED=1`。目录条目也参与计数（plan/03 §二.5）。

`dup-hash.json` 钉住 §4.4 ③ 的**保守性**：

```bash
cd data/fixtures/align-golden/dup-hash/pair
sha256sum left/pkg/copy1.bin left/pkg/copy2.bin right/pkg/moved.bin   # 三个哈希完全相同
```

同一哈希在左侧出现 **2** 次、右侧 1 次 ⇒ **不判改名**，各自记 `REMOVED`/`ADDED`。
计数：`SAME=2, ADDED=1, REMOVED=2, RENAMED=0`。这就是 K10 的机器断言。

## 逐字节校验

```bash
cmp data/golden/align/basic.json <(go run ./cmd/bdiff align \
  --left  data/fixtures/align-golden/rename-add/pair/left \
  --right data/fixtures/align-golden/rename-add/pair/right) && echo "basic ok"
```

`go test ./internal/cli` 里的 `TestAlignGoldenMatchesCLI` 跑的是同一条断言（不经过 shell）。
`go test ./internal/align` 的 `TestAlignDeterministic` 断言同输入双跑逐字节一致。

## 改名与新增为什么能同时出现

`data/README.md` §4.4 的"唯一"取**每哈希唯一**（该 sha256 在两侧各出现 1 次、
两侧各只有一条未匹配条目携带它），不是"全局只剩一条 REMOVED 与一条 ADDED"。
后者会让 `plan/03` §二 的示例（同树 + 1 改名 + 1 新增 → `ADDED:1, RENAMED:1`）
永远不可能成立。两种读法的区别被 `basic.json`（1 改名 + 1 新增 → 都算）与
`dup-hash.json`（同哈希多处 → 都不算）两个 golden 同时钉住。
