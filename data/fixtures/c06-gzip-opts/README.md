# c06-gzip-opts · tar.gz 的 gzip 成员头 xfl/os 不同

> 由 `tools/gen_fixtures` 程序化生成，**禁止手改**。
> 重建校验：`go run ./tools/gen_fixtures --out data/fixtures --check`（应输出 `13 cases, 0 diff`）。

## 注入的差异

- 容器：`tar.gz`
- 注入类别：T6
- 两侧参数：见 `expect.json` 的 `inputs`

## 期望

- `counts_positive`：T6
- `minimal_cause_set`：`[T6]`
- `residue_bytes`：0
- 退出码：2

## 手工复现

```bash
cd data/fixtures/c06-gzip-opts
sha256sum pair/left/synthpkg-0.1.0.tar.gz pair/right/synthpkg-0.1.0.tar.gz
tar -tzvf pair/left/synthpkg-0.1.0.tar.gz
tar -tzvf pair/right/synthpkg-0.1.0.tar.gz
od -An -tx1 -N10 pair/left/synthpkg-0.1.0.tar.gz   # 第 9/10 字节 = xfl/os
```
