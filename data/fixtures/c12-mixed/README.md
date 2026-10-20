# c12-mixed · T1 + T3 + T9 + 一处真改

> 由 `tools/gen_fixtures` 程序化生成，**禁止手改**。
> 重建校验：`go run ./tools/gen_fixtures --out data/fixtures --check`（应输出 `14 cases, 0 diff`）。

## 注入的差异

- 容器：`tar`
- 注入类别：T1、T3、T9、T12
- 两侧参数：见 `expect.json` 的 `inputs`

## 期望

- `counts_positive`：T1、T3、T9、T12
- `minimal_cause_set`：`null`（§4.5：真改不可归一化，任何 S 都无法让树摘要相等）
- `residue_bytes`：1
- 退出码：3

## 手工复现

```bash
cd data/fixtures/c12-mixed
sha256sum pair/left/synthpkg-0.1.0.tar pair/right/synthpkg-0.1.0.tar
tar -tvf pair/left/synthpkg-0.1.0.tar
tar -tvf pair/right/synthpkg-0.1.0.tar
tar -xOf pair/right/synthpkg-0.1.0.tar synthpkg/bin/synthpkg | strings -a | grep -c build || true   # build-id 段存在，但两侧内容不同
```
