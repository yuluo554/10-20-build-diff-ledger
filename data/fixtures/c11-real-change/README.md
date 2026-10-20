# c11-real-change · 一处内容字节真改（T12，无最小原因集）

> 由 `tools/gen_fixtures` 程序化生成，**禁止手改**。
> 重建校验：`go run ./tools/gen_fixtures --out data/fixtures --check`（应输出 `14 cases, 0 diff`）。

## 注入的差异

- 容器：`tar.gz`
- 注入类别：T12
- 两侧参数：见 `expect.json` 的 `inputs`

## 期望

- `counts_positive`：T12
- `minimal_cause_set`：`null`（§4.5：真改不可归一化，任何 S 都无法让树摘要相等）
- `residue_bytes`：1
- 退出码：3

## 手工复现

```bash
cd data/fixtures/c11-real-change
sha256sum pair/left/synthpkg-0.1.0.tar.gz pair/right/synthpkg-0.1.0.tar.gz
tar -tzvf pair/left/synthpkg-0.1.0.tar.gz
tar -tzvf pair/right/synthpkg-0.1.0.tar.gz
rm -rf /tmp/bdiff-c11-l /tmp/bdiff-c11-r && mkdir -p /tmp/bdiff-c11-l /tmp/bdiff-c11-r && tar -xzf pair/left/synthpkg-0.1.0.tar.gz -C /tmp/bdiff-c11-l && tar -xzf pair/right/synthpkg-0.1.0.tar.gz -C /tmp/bdiff-c11-r && cmp -l /tmp/bdiff-c11-l/synthpkg/data/table.csv /tmp/bdiff-c11-r/synthpkg/data/table.csv | wc -l   # = 1
```
