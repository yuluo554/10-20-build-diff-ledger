# c09-mtime-owner · mtime + 属主 同时不同

> 由 `tools/gen_fixtures` 程序化生成，**禁止手改**。
> 重建校验：`go run ./tools/gen_fixtures --out data/fixtures --check`（应输出 `13 cases, 0 diff`）。

## 注入的差异

- 容器：`tar`
- 注入类别：T1、T2
- 两侧参数：见 `expect.json` 的 `inputs`

## 期望

- `counts_positive`：T1、T2
- `minimal_cause_set`：`[T1, T2]`
- `residue_bytes`：0
- 退出码：2

## 手工复现

```bash
cd data/fixtures/c09-mtime-owner
sha256sum pair/left/synthpkg-0.1.0.tar pair/right/synthpkg-0.1.0.tar
tar -tvf pair/left/synthpkg-0.1.0.tar
tar -tvf pair/right/synthpkg-0.1.0.tar
tar -tvf pair/left/synthpkg-0.1.0.tar   # 同时看 mtime 与 owner 两列
```
