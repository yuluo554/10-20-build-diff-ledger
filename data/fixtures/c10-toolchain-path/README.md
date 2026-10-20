# c10-toolchain-path · ELF 的 .comment（T8）与 DWARF comp_dir（T11）不同

> 由 `tools/gen_fixtures` 程序化生成，**禁止手改**。
> 重建校验：`go run ./tools/gen_fixtures --out data/fixtures --check`（应输出 `14 cases, 0 diff`）。

## 注入的差异

- 容器：`tar`
- 注入类别：T8、T11
- 两侧参数：见 `expect.json` 的 `inputs`

## 期望

- `counts_positive`：T8、T11
- `minimal_cause_set`：`[T8, T11]`
- `residue_bytes`：0
- 退出码：2

## 手工复现

```bash
cd data/fixtures/c10-toolchain-path
sha256sum pair/left/synthpkg-0.1.0.tar pair/right/synthpkg-0.1.0.tar
tar -tvf pair/left/synthpkg-0.1.0.tar
tar -tvf pair/right/synthpkg-0.1.0.tar
strings -a pair/right/synthpkg-0.1.0.tar | grep /build/
```
