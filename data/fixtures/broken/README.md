# broken · 读不了的输入

`truncated.tar` 是一个声明 100 字节内容、实际只有 10 字节的 tar。
期望：`bdiff align/compare --left broken/truncated.tar ...` 必须 **exit 1**（用法或输入不可解析），
不得当成"一致"（静默通过的经典漏洞，见 data/README.md §4.6）。

手工复现：

```bash
tar -tvf data/fixtures/broken/truncated.tar   # 报 "Unexpected EOF"
```
