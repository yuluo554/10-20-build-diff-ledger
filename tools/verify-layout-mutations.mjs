#!/usr/bin/env node
// 布局门的变异验证脚本（plan/05 §四）。
//
// 每组变异：改一处 → 跑门 → **必须报出指定判据为 FAIL** → 还原 → 断言字节已还原。
// 三连断言（替换前 marker 存在 / 替换后文件已变 / 还原后字节一致）写在每个 mutate() 里，
// 防"变异没生效却以为门是假门"（crash-loop-rescue 的变异三连条）。
//
// 用法：node tools/verify-layout-mutations.mjs
// 退出码：0 全部按预期红且还原干净 / 1 有变异没红或还原失败。

import { readFileSync, writeFileSync, existsSync, unlinkSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const GATE = join(ROOT, "tools", "check-sprint-layout.mjs");

function gate() {
  const r = spawnSync(process.execPath, [GATE], { encoding: "utf8" });
  return { code: r.status, out: (r.stdout ?? "") + (r.stderr ?? "") };
}

function mutate(file, from, to, label) {
  const path = join(ROOT, file);
  const before = readFileSync(path, "utf8");
  if (!before.includes(from)) throw new Error(`${label}: marker 不存在于 ${file}`);
  const after = before.replace(from, to);
  if (after === before) throw new Error(`${label}: 替换未产生字节变化`);
  writeFileSync(path, after);
  return () => {
    writeFileSync(path, before);
    const restored = readFileSync(path, "utf8");
    if (restored !== before) throw new Error(`${label}: 还原后字节不一致`);
  };
}

function removeFile(file, label) {
  const path = join(ROOT, file);
  if (!existsSync(path)) throw new Error(`${label}: 待删文件不存在`);
  const before = readFileSync(path);
  unlinkSync(path);
  return () => {
    writeFileSync(path, before);
    if (!existsSync(path)) throw new Error(`${label}: 还原失败`);
  };
}

const MUTATIONS = [
  {
    id: "V1",
    label: "删 AGENTS.md",
    expect: "LAYOUT.FILES",
    apply: () => removeFile("AGENTS.md", "V1"),
  },
  {
    id: "V2",
    label: "删 plan/02",
    expect: "LAYOUT.FILES",
    apply: () => removeFile("plan/02-架构与技术选型.md", "V2"),
  },
  {
    id: "V3",
    label: "删 plan/00 装订行",
    expect: "LAYOUT.BINDING_HEAD",
    apply: () =>
      mutate(
        "plan/00-README总览.md",
        "> 方法论：`ai-tool-project-sprint`（装订 2026-10-20）｜布局门：`node tools/check-sprint-layout.mjs`｜续接对话第一件事 = 跑它",
        "> （装订行已移除）",
        "V3",
      ),
  },
  {
    id: "V4",
    label: "plan/05 把 M6 翻成 ✅（其它副本仍 ⬜）",
    expect: "LAYOUT.STATUS",
    // M6 已是最后一个里程碑：M6 收尾后无处可打 → 改打"哨兵里程碑 M7"，
    // 变异 = plan/05 单方面把 M7 翻 ✅（其它副本无此行）。
    apply: () =>
      mutate(
        "plan/05-里程碑与验收.md",
        "| M7 | 哨兵里程碑（变异靶子，实际不存在） | ⬜ 未开始 |",
        "| M7 | 哨兵里程碑（变异靶子，实际不存在） | ✅ 完成 |",
        "V4",
      ),
  },
  {
    id: "V5",
    label: "取消 M0 豁免并要求 HANDOFF（删豁免 + M0 无交接单）",
    expect: "LAYOUT.HANDOFFS",
    apply: () =>
      mutate(
        "tools/check-sprint-layout.mjs",
        "const EXEMPT_MILESTONES = [0];",
        "const EXEMPT_MILESTONES = [];",
        "V5",
      ),
  },
  {
    id: "V6",
    label: "删 data/README.md §4.1 的 T12 行",
    expect: "LAYOUT.CONTRACT",
    apply: () =>
      mutate(
        "data/README.md",
        "| T12 | 未解释 | 归不进 T1–T11 的任何差异 | **无 op**（T12 不可归一化；出现即 `residue_bytes > 0`） |",
        "",
        "V6",
      ),
  },
  {
    id: "V7",
    label: "internal/artifact 里 import net/http",
    expect: "LAYOUT.NO_NET",
    apply: () =>
      mutate(
        "internal/artifact/artifact.go",
        'import (\n\t"bufio"',
        'import (\n\t_ "net/http"\n\t"bufio"',
        "V7",
      ),
  },
  {
    id: "V8",
    label: "删 ci.yml 的布局门步骤",
    expect: "LAYOUT.CI",
    apply: () =>
      mutate(
        ".github/workflows/ci.yml",
        "      - name: \"Sprint layout gate\"\n        run: node tools/check-sprint-layout.mjs",
        "      - name: \"Sprint layout gate\"\n        run: echo skipped",
        "V8",
      ),
  },
  {
    id: "V9",
    label: "plan/06 缺口列写'待 M0'",
    expect: "LAYOUT.GAPS",
    // 变异点必须落在**缺口列**（第 2 列），不是"关闭条件"列（第 3 列）。
    // 首版写在第 3 列 → 门按表头定位取第 2 列、正确地没报红；
    // 这是"变异靶子瞄错"而非假门（方法论假门族诊断三问的第②问）。
    apply: () =>
      mutate(
        "plan/06-交付对标与决策.md",
        "| G5 | ~~远端 Release 页未建~~ 已关闭：Release `v0.1.0` 已建并附产物，附件 digest 与本地留副本一致（见发布留档 §二） | — |",
        "| G5 | 待 M0 收尾 | — |",
        "V9",
      ),
  },
  {
    id: "V10",
    label: "plan/05 引用一个不存在的 tools 脚本路径",
    expect: "LAYOUT.DOC_TOOLS",
    // marker 随里程碑推进而更新：M6 收尾后 M6 DoD 全部打勾 →
    // 改打发布留档 `plan/RELEASE-M6.md` §二的待回填行。
    apply: () =>
      mutate(
        "plan/RELEASE-M6.md",
        "- 公开仓库：https://github.com/yuluo554/10-20-build-diff-ledger （2026-10-20 建，首推 main + tag v0.1.0）。",
        "- 先跑 `tools/definitely-missing.go`\n- 公开仓库：https://github.com/yuluo554/10-20-build-diff-ledger （2026-10-20 建，首推 main + tag v0.1.0）。",
        "V10",
      ),
  },
  {
    id: "V11",
    label: "plan/HANDOFF-M1 删掉七节中的一节（状态不变量）",
    expect: "LAYOUT.HANDOFFS",
    // 覆盖"已存在的交接单一律校七节"这条判据（与 V5 的"缺件"是两条独立判据）。
    apply: () =>
      mutate(
        "plan/HANDOFF-M1.md",
        "## 二、当前事实·状态不变量（下一棒动手前必须知道）",
        "## 二、补充说明",
        "V11",
      ),
  },
];

function main() {
  const baseline = gate();
  if (baseline.code !== 0) {
    console.error("基线不是绿的——先修布局再跑变异验证");
    console.error(baseline.out);
    process.exit(1);
  }
  console.log(`基线：全绿（${baseline.out.trim().split("\n").length} 行输出）\n`);

  let failed = 0;
  for (const m of MUTATIONS) {
    let restore = null;
    try {
      restore = m.apply();
      const r = gate();
      const red = r.code === 1 && r.out.includes(`FAIL  ${m.expect}`);
      if (red) {
        console.log(`PASS  ${m.id}  ${m.label} → ${m.expect} 如期变红`);
      } else {
        failed += 1;
        console.log(`FAIL  ${m.id}  ${m.label} → 期望 ${m.expect} 变红，实际 exit=${r.code}`);
        console.log(r.out.split("\n").filter((l) => l.startsWith("FAIL")).join("\n"));
      }
    } catch (err) {
      failed += 1;
      console.log(`FAIL  ${m.id}  ${m.label} → 变异本身出错：${err.message}`);
    } finally {
      if (restore) restore();
    }
  }

  const final = gate();
  if (final.code !== 0) {
    failed += 1;
    console.log("\n还原后基线不是绿的：");
    console.log(final.out);
  } else {
    console.log("\n还原后基线：全绿");
  }
  console.log(`\n结论：${MUTATIONS.length - failed}/${MUTATIONS.length} 组变异如期变红`);
  process.exitCode = failed === 0 ? 0 : 1;
}

main();
