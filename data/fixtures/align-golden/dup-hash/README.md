# align golden 输入 · dup-hash

> 由 `tools/gen_fixtures` 程序化生成，**禁止手改**。

两侧是**目录形态**的产物（不是归档），用于 `data/golden/align/` 的逐字节比对。

手工复现条目表：

```bash
find data/fixtures/align-golden/dup-hash/pair/left -type f | sort
find data/fixtures/align-golden/dup-hash/pair/right -type f | sort
cd data/fixtures/align-golden/dup-hash/pair/left && find . -type f -exec sha256sum {} + | sort
```
