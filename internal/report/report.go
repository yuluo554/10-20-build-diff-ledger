// Package report 实现报告渲染与门禁（模块⑤，plan/03 §六）。
//
// 键序唯一权威：data/README.md §4.8。所有 JSON 输出经本包 MarshalCanonical，
// 子命令不得自行 json.Marshal 后写盘（plan/02 §二 规则 3）。
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/version"
)

// Side 是报告里的一侧输入：只写 basename 与 sha256（不写绝对路径）。
type Side struct {
	Basename string `json:"basename"`
	SHA256   string `json:"sha256"`
}

// Report 的字段顺序 = report.json 的键序（data/README.md §4.8）。
// 新增字段必须追加在末尾并在 §4.8 同步登记，否则 golden 会红。
type Report struct {
	SchemaVersion   int            `json:"schema_version"`
	ToolVersion     string         `json:"tool_version"`
	Left            Side           `json:"left"`
	Right           Side           `json:"right"`
	Counts          Counts         `json:"counts"`
	Renamed         []RenamedItem  `json:"renamed"`
	Added           []string       `json:"added"`
	Removed         []string       `json:"removed"`
	ResidueBytes    int64          `json:"residue_bytes"`
	MinimalCauseSet []string       `json:"minimal_cause_set"`
	ResidualItems   []ResidualItem `json:"residual_items"`
	Notes           []string       `json:"notes"`
}

// Counts 是 T1–T12 + total，**序列化键序固定**（手写 MarshalJSON）。
type Counts struct {
	Values map[string]int
}

// RenamedItem 是一条改名记录。
type RenamedItem struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ResidualItem 是一条残留条目。
type ResidualItem struct {
	Path           string `json:"path"`
	RuleID         string `json:"rule_id"`
	BytesDifferent int64  `json:"bytes_different"`
}

// MarshalJSON 按 model.RuleIDs 的顺序输出 T1…T12 再接 total。
func (c Counts) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, rule := range model.RuleIDs {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%q:%d", rule, c.Values[rule])
	}
	fmt.Fprintf(&b, ",%q:%d", "total", c.Values["total"])
	b.WriteByte('}')
	return b.Bytes(), nil
}

// UnmarshalJSON 反向解析（golden 比对用）。
func (c *Counts) UnmarshalJSON(data []byte) error {
	var raw map[string]int
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Values = raw
	return nil
}

// New 建一份空报告（M5 由 attribute/minimize 结果填充）。
func New(left, right Side) *Report {
	return &Report{
		SchemaVersion:   version.SchemaVersion,
		ToolVersion:     version.Version,
		Left:            left,
		Right:           right,
		Counts:          Counts{Values: zeroCounts()},
		Renamed:         []RenamedItem{},
		Added:           []string{},
		Removed:         []string{},
		MinimalCauseSet: []string{},
		ResidualItems:   []ResidualItem{},
		Notes:           []string{},
	}
}

// Input 是 Build 的入参：只用 model 与原生类型，**不 import attribute/align**（K4 依赖方向）。
// 由 cli 把 attribute.Result / minimize.Search / align.Result 折算成这里。
type Input struct {
	Left, Right Side
	// Counts 的键集合应为 model.RuleIDs + "total"；缺键按 0 处理。
	Counts map[string]int
	// Renamed 按 to 升序；Added/Removed 按 relpath 升序。
	Renamed []RenamedItem
	Added   []string
	Removed []string

	ResidueBytes int64
	// MinimalCauseSetFound=false → 无解，序列化为 null（§4.5）。
	// Found=true 且集合为空 → 完全一致，序列化为 []。
	MinimalCauseSet      []string
	MinimalCauseSetFound bool

	ResidualItems []ResidualItem
	Notes         []string
}

// Build 是 report.Report 的**唯一**构造入口（cli 不得逐字段赋值）：
// 它负责三件容易写错的事——键集合补零、`null` 与 `[]` 的区分、切片永不为 nil。
//
// 返回 error 的情形只有一种：违反 §4.5 的"无解 ⇔ residue_bytes > 0"。
// 那是构造方的口径错误（不是用户输入问题），宁可显式报错也不产出自相矛盾的报告。
func Build(in Input) (*Report, error) {
	if !in.MinimalCauseSetFound && in.ResidueBytes <= 0 {
		return nil, fmt.Errorf("报告口径冲突：minimal_cause_set 无解但 residue_bytes = %d（§4.5 要求 > 0）", in.ResidueBytes)
	}
	r := New(in.Left, in.Right)
	for _, rule := range model.RuleIDs {
		r.Counts.Values[rule] = in.Counts[rule]
	}
	r.Counts.Values["total"] = in.Counts["total"]
	r.Renamed = append(r.Renamed, in.Renamed...)
	r.Added = append(r.Added, in.Added...)
	r.Removed = append(r.Removed, in.Removed...)
	r.ResidueBytes = in.ResidueBytes
	if in.MinimalCauseSetFound {
		r.MinimalCauseSet = append([]string{}, in.MinimalCauseSet...)
	} else {
		r.MinimalCauseSet = nil // JSON null：无解
	}
	r.ResidualItems = append(r.ResidualItems, in.ResidualItems...)
	r.Notes = append(r.Notes, in.Notes...)
	return r, nil
}

func zeroCounts() map[string]int {
	m := make(map[string]int, len(model.RuleIDs)+1)
	for _, r := range model.RuleIDs {
		m[r] = 0
	}
	m["total"] = 0
	return m
}

// MarshalCanonical 是 report.json 的唯一序列化入口：
// 不缩进、不转义 HTML、`\n` 结尾、字段顺序由 struct 定义。
func MarshalCanonical(r *Report) ([]byte, error) {
	return MarshalValue(r)
}

// MarshalValue 是**一切 JSON 输出**的唯一编码器（plan/02 §二 规则 3）：
// bdiff align 的条目表也走这里，禁止子命令自行 json.Marshal。
// 口径：不缩进、不转义 HTML、单个 `\n` 结尾、键序 = struct 字段序。
func MarshalValue(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil { // Encode 自带结尾换行
		return nil, err
	}
	return b.Bytes(), nil
}

// RenderMarkdown 渲染人读视图（plan/03 §六 的固定模板）：
// 标题 / 输入摘要 / 逐类计数表 / 改名·新增·删除清单 / 残留条目 / 最小原因集 / 结论行。
//
// 纪律：不含运行时刻、不含绝对路径、不含主机名；双跑逐字节一致；换行只用 `\n`。
func RenderMarkdown(r *Report) []byte {
	var b strings.Builder
	b.WriteString("# bdiff 差异归因账本\n\n")
	b.WriteString("## 输入\n\n")
	b.WriteString("| 侧 | basename | sha256 |\n|---|---|---|\n")
	fmt.Fprintf(&b, "| left | `%s` | `%s` |\n", r.Left.Basename, r.Left.SHA256)
	fmt.Fprintf(&b, "| right | `%s` | `%s` |\n", r.Right.Basename, r.Right.SHA256)

	b.WriteString("\n## 分类计数\n\n| rule | 名称 | 计数 |\n|---|---|---|\n")
	for _, rule := range model.RuleIDs {
		fmt.Fprintf(&b, "| %s | %s | %d |\n", rule, RuleLabel(rule), r.Counts.Values[rule])
	}
	fmt.Fprintf(&b, "| **total** | — | **%d** |\n", r.Counts.Values["total"])

	b.WriteString("\n## 改名 / 新增 / 删除\n\n")
	b.WriteString("### 改名\n\n")
	if len(r.Renamed) == 0 {
		b.WriteString("（无）\n")
	} else {
		b.WriteString("| from | to |\n|---|---|\n")
		for _, it := range r.Renamed {
			fmt.Fprintf(&b, "| `%s` | `%s` |\n", it.From, it.To)
		}
	}
	b.WriteString("\n### 新增\n\n")
	writePathList(&b, r.Added)
	b.WriteString("\n### 删除\n\n")
	writePathList(&b, r.Removed)

	b.WriteString("\n## 残留\n\n")
	fmt.Fprintf(&b, "- `residue_bytes`: %d\n", r.ResidueBytes)
	if len(r.ResidualItems) == 0 {
		b.WriteString("- 残留条目：（无）\n")
	} else {
		b.WriteString("\n| path | rule | 不同字节 |\n|---|---|---|\n")
		for _, it := range r.ResidualItems {
			fmt.Fprintf(&b, "| `%s` | %s | %d |\n", it.Path, it.RuleID, it.BytesDifferent)
		}
	}

	b.WriteString("\n## 最小原因集\n\n")
	if r.MinimalCauseSet == nil {
		b.WriteString("- `minimal_cause_set`: `null`（无解：T1–T11 全施加后两侧仍不同）\n")
	} else if len(r.MinimalCauseSet) == 0 {
		b.WriteString("- `minimal_cause_set`: `[]`（两侧完全一致，无需归一化）\n")
	} else {
		parts := make([]string, 0, len(r.MinimalCauseSet))
		for _, rule := range r.MinimalCauseSet {
			parts = append(parts, fmt.Sprintf("`%s`（%s）", rule, RuleLabel(rule)))
		}
		b.WriteString("- `minimal_cause_set`: " + strings.Join(parts, "、") + "\n")
	}

	if len(r.Notes) > 0 {
		b.WriteString("\n## 口径提示\n\n")
		for _, n := range r.Notes {
			fmt.Fprintf(&b, "- %s\n", n)
		}
	}

	b.WriteString("\n## 判定\n\n")
	b.WriteString(verdictLine(r))
	return []byte(b.String())
}

func writePathList(b *strings.Builder, paths []string) {
	if len(paths) == 0 {
		b.WriteString("（无）\n")
		return
	}
	for _, p := range paths {
		fmt.Fprintf(b, "- `%s`\n", p)
	}
}

func verdictLine(r *Report) string {
	switch {
	case r.ResidueBytes > 0:
		return "- 结论：**有残留**（存在归一化后仍不同的字节）。\n"
	case len(r.MinimalCauseSet) > 0:
		return "- 结论：**仅元数据差异**（全部差异已被分类规则解释）。\n"
	default:
		return "- 结论：**完全一致**。\n"
	}
}

// RuleLabel 把 rule id 转成人读名（帮助行文，不参与判定）。
func RuleLabel(rule string) string {
	labels := map[string]string{
		"T1": "时间戳", "T2": "属主", "T3": "权限", "T4": "路径前缀",
		"T5": "条目顺序", "T6": "压缩器参数", "T7": "归档头字段", "T8": "工具链串",
		"T9": "调试符号与 build-id", "T10": "签名 blob", "T11": "构建路径与环境", "T12": "未解释",
	}
	if l, ok := labels[rule]; ok {
		return l
	}
	return rule
}

// ParseBudget 解析 --budget：支持单值、"a,b" 逗号多值、all、空串（= 无白名单）。
// 非法值返回 error（CLI 映射为 exit 1）。三形态各有测试（plan/06 §二）。
func ParseBudget(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if s == "all" {
		return append([]string(nil), model.RuleIDs...), nil
	}
	var out []string
	seen := make(map[string]bool)
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if !isRuleID(p) {
			return nil, fmt.Errorf("--budget 含未知规则 %q（合法值：T1…T12 或 all）", p)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--budget 为空（可省略该参数，或写 T1,T2 / all）")
	}
	return out, nil
}

func isRuleID(s string) bool {
	for _, r := range model.RuleIDs {
		if r == s {
			return true
		}
	}
	return false
}

// FormatInt 是报告里整数的唯一格式化入口（十进制，无本地化）。
func FormatInt(n int64) string { return strconv.FormatInt(n, 10) }
