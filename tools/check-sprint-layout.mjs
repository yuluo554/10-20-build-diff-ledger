#!/usr/bin/env node
// 布局门（方法论装订第②件）：把"项目还长在 ai-tool-project-sprint 方法论上"做成可跑红的断言。
// 判据一律落**仓库内文件**，不读 git 元数据（浅克隆下 tag/历史深度会翻转）。
// 退出码：0 全绿 / 1 有 FAIL / 2 用法错误。
//
// 假门族对照（references/methodology-binding.md §二）：
//   ① 真值不取自任何一个副本 —— 逐副本采集 Map<k,Set<state>>，出现两种状态即 FAIL
//   ② 按表头定位列号再取单元格 —— 不写"行里含『缺口』/『状态』"
//   ③ 判据用同现（前瞻），不用"或"
//   ④ 顺序无关（装订行三锚点用 (?=) 前瞻）
//   ⑤ 状态判据只认状态表的数据行，不扫自由文本
//   ⑥⑦⑧ 匹配前剥离行内代码与文件名形态，避免 plan/RELEASE-M6.md 这类标识符误判
//   非空跑自证：每次运行都打印"这轮扫到了几个"

import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");

const checks = [];
const add = (id, ok, detail) => checks.push({ id, ok, detail });
const read = (rel) => (existsSync(join(ROOT, rel)) ? readFileSync(join(ROOT, rel), "utf8") : null);
const scanned = { docs: 0, handoffs: 0, copies: 0, rows: 0, docTools: 0 };

const REQUIRED_FILES = [
  "AGENTS.md",
  "README.md",
  "data/README.md",
  "ci/example.yml",
  ".github/workflows/ci.yml",
  "go.mod",
  "cmd/bdiff/main.go",
  "tools/gen_fixtures/main.go",
  "tools/check-sprint-layout.mjs",
  "plan/00-README总览.md",
  "plan/01-题目文档.md",
  "plan/02-架构与技术选型.md",
  "plan/03-模块详设.md",
  "plan/04-数据计划.md",
  "plan/05-里程碑与验收.md",
  "plan/06-交付对标与决策.md",
];

/** 三锚点同现于一行：方法论名 + 布局门命令 + 装订日期（顺序无关）。 */
const BINDING_LINE = /^>(?=.*ai-tool-project-sprint)(?=.*check-sprint-layout)(?=.*20\d{2}-\d{2}-\d{2}).*$/m;

const REQUIRED_SECTIONS = {
  "plan/00-README总览.md": ["文档索引", "决策记录", "承诺-证据台账", "里程碑"],
  "plan/01-题目文档.md": ["任务要求", "评分标准", "边界与非目标"],
  "plan/02-架构与技术选型.md": ["分层", "构建"],
  "plan/03-模块详设.md": ["align", "facts", "attribute", "minimize", "report"],
  "plan/04-数据计划.md": ["样张矩阵", "真值纪律", "数值口径"],
  "plan/05-里程碑与验收.md": ["DoD", "已知未验证项"],
  "plan/06-交付对标与决策.md": ["评分对标", "风险登记", "缺口"],
  "data/README.md": ["依据查证记录", "数值口径"],
};

/** 七节（超集合法：只断言"都在"，不写死节数）。 */
const HANDOFF_SECTIONS = ["当前进度", "状态不变量", "待办", "既定口径", "环境坑", "DoD", "命令速查"];

/** 项目已声明的豁免：M0 是计划阶段，收尾记录就是计划套件本身。 */
const EXEMPT_MILESTONES = [0];

/**
 * 状态副本：三处各一张「状态」表。
 * 真值不取自任何一个副本（见假门族①）。
 */
const STATUS_COPIES = [
  "plan/00-README总览.md",
  "plan/05-里程碑与验收.md",
  "README.md",
];

/** 剥离行内代码、文件名与路径形态——防 ⑥⑦⑧ 的标识符误判。 */
const stripIdentifiers = (s) => s.replace(/`[^`]*`/g, "").replace(/[\w./-]+\.(?:md|go|mjs|json|yml|yaml)/g, "");

/**
 * 按 markdown 表格行切单元格。
 * **必须先处理 `\|`**：单元格里的字面竖线（如 `\|S\|≤12`）在 markdown 里是合法转义，
 * 天真的 `split("|")` 会把它当成列分隔符 → 取到的"状态"格是内容列的一段，
 * 表现为"状态格里既无 ✅ 也无 ⬜"（本门自己实测到的第一个缺陷）。
 */
const PIPE_PLACEHOLDER = "\u0001";
function splitRow(line) {
  return line.split("|").map((cell) => cell.split(PIPE_PLACEHOLDER).join("|"));
}
function normalizeEscapedPipes(line) {
  return line.replace(/\\\|/g, PIPE_PLACEHOLDER);
}

/**
 * 按表头定位列号再取单元格（假门族②）。
 * 一份文档里可能有**多张**带「状态」列的表（如 plan/00 的文档索引表），
 * 所以逐个候选表头尝试，取**第一张真有 M 行**的表——"表头里有那个词就以为判到了"是假门族②。
 * 返回 { headerLine, statusCol, rows:[{id,state,line}] } 或 null。
 */
function parseStatusTable(text) {
  if (text === null) return null;
  const lines = text.split("\n").map(normalizeEscapedPipes);
  for (let headerIdx = 0; headerIdx < lines.length; headerIdx += 1) {
    const line = lines[headerIdx];
    if (!line.startsWith("|") || !line.includes("状态")) continue;
    const cells = splitRow(line);
    const statusCol = cells.findIndex((c) => c.includes("状态"));
    if (statusCol < 1) continue;
    const rows = [];
    for (const row of lines.slice(headerIdx + 2)) {
      if (!row.startsWith("|")) break; // 表结束
      const parts = splitRow(row);
      const first = parts[1] ?? "";
      const m = /^\s*\*{0,2}M(\d+)\b/.exec(first);
      if (!m) continue;
      const cell = (parts[statusCol] ?? "").replace(/\*/g, "").trim();
      let state = null;
      if (/✅/.test(cell)) state = "done";
      else if (/⬜|🟡|未开始|待开始|未执行|待执行/.test(cell)) state = "pending";
      rows.push({ id: Number(m[1]), state, cell, line: row.trim().slice(0, 80) });
    }
    if (rows.length > 0) return { headerLine: line.trim(), statusCol, rows };
  }
  return null;
}

function checkRequiredFiles() {
  const missing = REQUIRED_FILES.filter((rel) => !existsSync(join(ROOT, rel)));
  scanned.docs = REQUIRED_FILES.length - missing.length;
  add("LAYOUT.FILES", missing.length === 0, missing.length ? `缺: ${missing.join(",")}` : `${scanned.docs}/${REQUIRED_FILES.length} 份必备件齐备`);
}

function checkBindingHead() {
  const text = read("plan/00-README总览.md") ?? "";
  const ok = BINDING_LINE.test(text);
  add("LAYOUT.BINDING_HEAD", ok, ok ? "plan/00 顶部有装订行（方法论名+门命令+日期 同现）" : "plan/00 顶部缺装订行（需同现：方法论名 + 布局门命令 + 装订日期）");
}

function checkSections() {
  const bad = [];
  for (const [rel, needles] of Object.entries(REQUIRED_SECTIONS)) {
    const text = read(rel);
    if (text === null) continue;
    const missing = needles.filter((n) => !text.includes(n));
    if (missing.length) bad.push(`${rel}: ${missing.join("/")}`);
  }
  add("LAYOUT.SECTIONS", bad.length === 0, bad.length ? bad.join("；") : `${Object.keys(REQUIRED_SECTIONS).length} 份文档小节齐备`);
}

function collectStates() {
  const states = new Map();
  const note = (k, state, where) => {
    if (!states.has(k)) states.set(k, []);
    states.get(k).push({ state, where });
  };
  for (const file of STATUS_COPIES) {
    const table = parseStatusTable(read(file));
    if (!table) continue;
    scanned.copies += 1;
    for (const row of table.rows) {
      scanned.rows += 1;
      note(row.id, row.state ?? "unknown", file);
    }
  }
  return states;
}

function checkStatusAgreement(states) {
  const bad = [];
  for (const [k, entries] of [...states.entries()].sort((a, b) => a[0] - b[0])) {
    const distinct = new Set(entries.map((e) => e.state));
    if (distinct.has("unknown")) bad.push(`M${k} 状态格里既无 ✅ 也无 ⬜（${entries.find((e) => e.state === "unknown").where}）`);
    if (distinct.size > 1) {
      bad.push(`M${k} 副本间不一致: ${entries.map((e) => `${e.where}=${e.state}`).join(" vs ")}`);
    }
  }
  if (scanned.copies < 3) bad.push(`只扫到 ${scanned.copies} 处状态副本（下界 3）`);
  add("LAYOUT.STATUS", bad.length === 0, bad.length ? bad.join("；") : `${scanned.copies} 处副本 × ${states.size} 个里程碑 × ${scanned.rows} 行 全一致`);
}

function checkHandoffs(done) {
  const bad = [];
  for (const k of done) {
    if (EXEMPT_MILESTONES.includes(k)) continue;
    const rel = `plan/HANDOFF-M${k}.md`;
    if (read(rel) === null) bad.push(`${rel}(缺)`);
  }
  // 已存在的交接单**一律**校七节（不只看"已 ✅ 里程碑"的那份）：
  // 提前落盘、为下一棒写的那份也必须形态合格，否则形态缺陷会拖到它被计入时才暴露。
  const present = existsSync(join(ROOT, "plan"))
    ? readdirSync(join(ROOT, "plan")).filter((f) => /^HANDOFF-M\d+\.md$/.test(f))
    : [];
  for (const f of present) {
    const text = read(`plan/${f}`);
    scanned.handoffs += 1;
    const lack = HANDOFF_SECTIONS.filter((n) => !text.includes(n));
    if (lack.length) bad.push(`plan/${f}(缺节: ${lack.join("/")})`);
  }
  add("LAYOUT.HANDOFFS", bad.length === 0, bad.length ? bad.join("；") : `${scanned.handoffs} 份交接单七节齐（M0 已声明豁免）`);
}

/** plan/06 §缺口：按表头定位「缺口」列，再取单元格；先剥标识符。 */
function checkStaleGaps(done) {
  const text = read("plan/06-交付对标与决策.md");
  if (text === null) {
    add("LAYOUT.GAPS", false, "plan/06 不存在");
    return;
  }
  const lines = text.split("\n");
  const headerIdx = lines.findIndex((l) => l.startsWith("|") && l.includes("缺口"));
  if (headerIdx < 0) {
    add("LAYOUT.GAPS", false, "plan/06 找不到含『缺口』表头的表（判据表搬走了？）");
    return;
  }
  const statusCol = lines[headerIdx].split("|").findIndex((c) => c.includes("缺口"));
  const stale = [];
  let rows = 0;
  for (const line of lines.slice(headerIdx + 2)) {
    if (!line.startsWith("|")) break;
    rows += 1;
    const cell = stripIdentifiers(line.split("|")[statusCol] ?? "");
    for (const k of done) {
      // 不对任何已 ✅ 的里程碑开豁免：M0 也照判。
      // 首版这里写了 `if (k === 0) continue;`，变异验证 V9 当场证伪——
      // 于是"已完成的 M0 留在缺口列"永远抓不到（自我豁免 = 判据里的洞）。
      if (new RegExp(`(^|[^0-9A-Za-z])M${k}([^0-9]|$)`).test(cell)) stale.push(`plan/06 缺口列仍提已 ✅ 的 M${k}: ${cell.trim()}`);
    }
  }
  if (rows === 0) {
    add("LAYOUT.GAPS", false, "plan/06 缺口表 0 行——判据是空的");
    return;
  }
  add("LAYOUT.GAPS", stale.length === 0, stale.length ? stale.join("；") : `缺口表 ${rows} 行，无已 ✅ 里程碑`);
}

/** M6 起要求发布留档两栏 + 阶段 7 覆盖对账表。 */
function checkReleaseDoc(done) {
  const last = done.length ? Math.max(...done) : 0;
  if (last < 6) {
    add("LAYOUT.RELEASE", true, `当前最高 ✅ 里程碑 M${last}（M6 起才要求发布留档）`);
    return;
  }
  const release = read(`plan/RELEASE-M${last}.md`);
  if (release === null) {
    add("LAYOUT.RELEASE", false, `缺 plan/RELEASE-M${last}.md`);
    return;
  }
  const cols = /^##\s*一、tag 前事实/m.test(release) && /^##\s*二、tag 后追加/m.test(release);
  const heading = /^#{2,3}.*覆盖对账/m.test(release);
  const rows = release.split("\n").filter((l) => /^\|/.test(l) && /(实现|不适用|待首推)/.test(l)).length;
  add("LAYOUT.RELEASE", cols && heading && rows >= 3, `两栏:${cols} 覆盖对账:${heading} 三态行:${rows}`);
}

/** 文档点名的 tools/ 脚本必须有实物（判据只认路径形态，不读语义）。 */
function checkDocumentedToolsExist() {
  const EXEMPT = new Set([]);
  const docs = ["AGENTS.md", ...(existsSync(join(ROOT, "plan")) ? readdirSync(join(ROOT, "plan")).filter((f) => f.endsWith(".md")).map((f) => `plan/${f}`) : [])];
  const referenced = new Set();
  for (const rel of docs) {
    const text = read(rel);
    if (text === null) continue;
    // 允许子目录（`tools/gen_fixtures/main.go`）——首版正则只认 `tools/<name>.<ext>`，
    // 于是文档里真点名的子目录脚本一个都没进判据（本门自己实测到的第二个缺陷）。
    for (const m of text.matchAll(/tools\/(?:[\w.-]+\/)*[\w.-]+\.(?:mjs|js|cjs|py|sh|ps1|go)/g)) referenced.add(m[0]);
  }
  if (referenced.size === 0) {
    add("LAYOUT.DOC_TOOLS", false, `扫了 ${docs.length} 份文档，0 处 tools/ 引用——判据是空的（阳性对照失败）`);
    return;
  }
  scanned.docTools = referenced.size;
  const missing = [...referenced].filter((rel) => !EXEMPT.has(rel) && !existsSync(join(ROOT, rel)));
  add("LAYOUT.DOC_TOOLS", missing.length === 0, missing.length ? `文档点名但不存在: ${missing.join(",")}` : `${referenced.size} 个文档点名的 tools/ 脚本全部在位`);
}

/** 数值口径单点：data/README.md §四 必须有 T1–T12 恰好 12 行 + 退出码 0/1/2/3。 */
function checkNumericContract() {
  const text = read("data/README.md");
  if (text === null) {
    add("LAYOUT.CONTRACT", false, "data/README.md 不存在");
    return;
  }
  const section = text.split(/^###\s*4\.1/m)[1]?.split(/^###\s*4\.2/m)[0] ?? "";
  const rules = new Set([...section.matchAll(/^\|\s*T(\d+)\s*\|/gm)].map((m) => Number(m[1])));
  const want = Array.from({ length: 12 }, (_, i) => i + 1);
  const missing = want.filter((n) => !rules.has(n));
  const exitOk = ["0", "1", "2", "3"].every((c) => new RegExp(`^\\|\\s*${c}\\s*\\|`, "m").test(text));
  add("LAYOUT.CONTRACT", missing.length === 0 && exitOk, `T1–T12: ${rules.size}/12${missing.length ? `(缺 ${missing.join(",")})` : ""} 退出码表:${exitOk}`);
}

/**
 * 零网络/零时钟的静态守门：源码禁止 import 受限包。
 * 判据只认 **import 块内的路径**（不扫注释/字符串字面量）——否则 `[]string{"T1","T2"}`
 * 这类字面量会制造假红（假门族③"断言了全世界"的镜像形态）。
 */
function checkForbiddenImports() {
  const forbidden = ["net", "net/http", "time", "math/rand", "os/exec", "os/user"];
  const roots = ["cmd", "internal", "tools"];
  const files = [];
  const walk = (dir) => {
    if (!existsSync(dir)) return;
    for (const ent of readdirSync(dir, { withFileTypes: true })) {
      const p = join(dir, ent.name);
      if (ent.isDirectory()) walk(p);
      else if (ent.name.endsWith(".go")) files.push(p);
    }
  };
  for (const r of roots) walk(join(ROOT, r));

  const hits = [];
  let importBlocks = 0;
  for (const f of files) {
    const src = readFileSync(f, "utf8");
    // 单行 import "x" 与 import ( ... ) 块；块内的行形如 `alias "path"` 或 `"path"`。
    const specs = [];
    for (const m of src.matchAll(/^import\s+\(([\s\S]*?)^\)/gm)) {
      importBlocks += 1;
      for (const line of m[1].split("\n")) {
        const s = /^\s*(?:[\w.]+\s+)?"([^"]+)"/.exec(line);
        if (s) specs.push(s[1]);
      }
    }
    for (const m of src.matchAll(/^import\s+(?:[\w.]+\s+)?"([^"]+)"/gm)) specs.push(m[1]);
    for (const p of specs) {
      if (forbidden.includes(p)) hits.push(`${f.slice(ROOT.length + 1)}: import ${p}`);
    }
  }
  if (files.length === 0 || importBlocks === 0) {
    add("LAYOUT.NO_NET", false, `${files.length} 个 .go 文件 / ${importBlocks} 个 import 块——判据是空的（阳性对照失败）`);
    return;
  }
  add("LAYOUT.NO_NET", hits.length === 0, hits.length ? hits.join("；") : `${files.length} 个 Go 文件 / ${importBlocks} 个 import 块无受限 import（${forbidden.join("/")}）`);
}

/** CI 里必须有一步跑本门（否则它只在开发机存在）。 */
function checkCiStep() {
  const text = read(".github/workflows/ci.yml");
  const ok = text !== null && text.includes("check-sprint-layout");
  add("LAYOUT.CI", ok, ok ? "ci.yml 含布局门步骤" : ".github/workflows/ci.yml 缺少 check-sprint-layout 步骤");
}

function main() {
  for (const run of [checkRequiredFiles, checkBindingHead, checkSections, checkNumericContract, checkForbiddenImports, checkDocumentedToolsExist, checkCiStep]) {
    try {
      run();
    } catch (error) {
      add(run.name, false, `崩了: ${error.message}`);
    }
  }
  const states = collectStates();
  const done = [...states.entries()].filter(([, es]) => es.every((e) => e.state === "done")).map(([k]) => k);
  for (const run of [
    () => checkStatusAgreement(states),
    () => checkHandoffs(done),
    () => checkStaleGaps(done),
    () => checkReleaseDoc(done),
  ]) {
    try {
      run();
    } catch (error) {
      add(run.name, false, `崩了: ${error.message}`);
    }
  }

  const failed = checks.filter((c) => !c.ok);
  process.stdout.write("布局门（方法论装订） · 判据只落仓库内文件\n");
  process.stdout.write("=".repeat(60) + "\n");
  for (const c of checks) process.stdout.write(`${c.ok ? "PASS" : "FAIL"}  ${c.id.padEnd(22)} ${c.detail}\n`);
  process.stdout.write("=".repeat(60) + "\n");
  process.stdout.write(`这轮扫到：必备件 ${scanned.docs} / 状态副本 ${scanned.copies} / 状态行 ${scanned.rows} / 交接单 ${scanned.handoffs} / 文档点名工具 ${scanned.docTools}\n`);
  process.stdout.write(`结论: ${failed.length === 0 ? "全绿" : `${failed.length} 项 FAIL`}\n`);
  process.exitCode = failed.length === 0 ? 0 : 1;
}

main();
