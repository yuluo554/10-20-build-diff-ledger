# AGENTS.md

本仓库遵循 **`ai-tool-project-sprint`** 方法论（装订日期：2026-10-20）。

## 任何新对话的第一件事

```bash
node tools/check-sprint-layout.mjs
```

布局门必须绿；红了先修布局再干活。门不存在 = 本仓库没装订 → 先补第零步三件（本文件 + 布局门 + `plan/00` 顶部装订行）。

- 续接对话：读 `plan/HANDOFF-M<k>.md`（第 k 棒的收尾交接单），**先跑上面这条门**。
- 接手对话（无 HANDOFF）：先读 `plan/00`、`plan/05`、`plan/06` 与各 HANDOFF，重建"当前状态 + K 口径清单"再动手。

## 强制布局（布局门逐条断言，改动前先看这里）

| 项 | 位置 | 判据 |
| --- | --- | --- |
| 计划套件 | `plan/00`–`plan/06` + `data/README.md` | 存在且各含必需小节（标题关键词） |
| 数据口径 | `data/README.md` §四 | T1–T12 恰好 12 行规则；退出码 0/1/2/3 表在 |
| 交接单 | `plan/HANDOFF-M<k>.md` | 每个 ✅ 里程碑一份（M0 已声明豁免），**七节**齐（含"当前事实·状态不变量"） |
| 里程碑状态副本 | `plan/00` 里程碑表 + `plan/05` 状态表 + `README` 状态行 | **逐处一致**（枚举式断言，真值不取自任何一个副本） |
| 缺口表 | `plan/06` §缺口 | 缺口列不得再提已 ✅ 的里程碑 |
| 发布留档 | `plan/RELEASE-M6.md` | M6 起要求两栏 + 阶段 7 覆盖对账表 |
| 文档点名的工具 | `tools/**` | `plan/*.md` 与 `AGENTS.md` 里出现的 `tools/<name>.<ext>` 必须有实物 |

## 三条项目硬约束

1. **零网络、零运行时依赖、不跑构建**：只读两份已产出的产物；`internal/` 与 `cmd/` 禁止 import `time`、`net`、`math/rand`、`os/exec`（`internal/guard` 有常驻测试）。
2. **数值口径一处定义**：T1–T12 / 归一化 op / 树摘要 / 退出码 / `|S|≤12` / 幂等要求只在 `data/README.md` §四；代码与文档摘录与之同源，禁止第二套表述。
3. **未知即显式**：解析器看不懂的头部进 `raw_header_sha256`；归不进 T1–T11 的进 T12；读不了的输入 exit 1。三条都不许静默。

## 环境事实与禁忌（实测日期 2026-10-20）

- 本机 Windows + Git Bash；`node v24.15.0` 可用；**Go 工具链需按需安装**：本次经阿里云镜像装到
  `C:\Users\ASUS\AppData\Local\Programs\Go`（2026-10-23 复核）。**`C:\Program Files\Go` 不存在**，
  且 Go 的 `bin` **不在 PATH**——不设 PATH 时 `go` 报 `command not found`，Git Bash 下先
  `export PATH="/c/Users/ASUS/AppData/Local/Programs/Go/bin:$PATH"`；`go version` 实测见 `plan/00` §八。
- 国内网络：`go.dev` 直连不稳，`proxy.golang.org` 不通；本项目**零第三方依赖**，不需要 GOPROXY 拉包。
- **禁止**：把"未执行"写成已完成；在 tag 之后改交付面代码；把 benchmark/星数等漂移数字写进代码或断言。
