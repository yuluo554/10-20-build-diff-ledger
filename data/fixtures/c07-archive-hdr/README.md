# c07-archive-hdr · zip 的 create_system（归档级字段）不同

> 由 `tools/gen_fixtures` 程序化生成，**禁止手改**。
> 重建校验：`go run ./tools/gen_fixtures --out data/fixtures --check`（应输出 `14 cases, 0 diff`）。

## 注入的差异

- 容器：`zip`
- 注入类别：T7
- 两侧参数：见 `expect.json` 的 `inputs`

## 期望

- `counts_positive`：T7
- `minimal_cause_set`：`[T7]`
- `residue_bytes`：0
- 退出码：2

## 手工复现

```bash
cd data/fixtures/c07-archive-hdr
sha256sum pair/left/synthpkg-0.1.0.zip pair/right/synthpkg-0.1.0.zip
unzip -l pair/left/synthpkg-0.1.0.zip
unzip -l pair/right/synthpkg-0.1.0.zip
unzip -l pair/left/synthpkg-0.1.0.zip   # 条目一致，差异在中央目录的 version-made-by
```
