package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
)

// TestReportKeyOrder 断言 report.json 的**逐字节键序**（data/README.md §4.8）。
// 变异验证：把 Report 的两个字段交换位置 → 本测试红。
func TestReportKeyOrder(t *testing.T) {
	r := New(Side{Basename: "l.tar", SHA256: "aa"}, Side{Basename: "r.tar", SHA256: "bb"})
	got, err := MarshalCanonical(r)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	want := []string{
		`"schema_version"`, `"tool_version"`, `"left"`, `"right"`, `"counts"`,
		`"renamed"`, `"added"`, `"removed"`, `"residue_bytes"`,
		`"minimal_cause_set"`, `"residual_items"`, `"notes"`,
	}
	pos := -1
	for _, key := range want {
		i := bytes.Index(got, []byte(key))
		if i < 0 {
			t.Fatalf("缺键 %s（输出：%s）", key, got)
		}
		if i < pos {
			t.Fatalf("键 %s 顺序错（在前一个键之前）\n输出：%s", key, got)
		}
		pos = i
	}
	if !bytes.HasSuffix(got, []byte("\n")) {
		t.Fatal("报告未以单个 \\n 结尾")
	}
	if bytes.Contains(got, []byte("\n  ")) || bytes.Contains(got, []byte("\n\t")) {
		t.Fatal("报告不应缩进（缩进由 report.md 承担）")
	}
}

// TestCountsKeyOrder 断言 counts 的 T1…T12 + total 顺序。
func TestCountsKeyOrder(t *testing.T) {
	r := New(Side{}, Side{})
	got, err := MarshalCanonical(r)
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Counts json.RawMessage `json:"counts"`
	}
	if err := json.Unmarshal(got, &probe); err != nil {
		t.Fatalf("counts 解析失败: %v", err)
	}
	pos := -1
	for _, rule := range append(append([]string{}, model.RuleIDs...), "total") {
		i := bytes.Index(probe.Counts, []byte(`"`+rule+`":`))
		if i < 0 {
			t.Fatalf("counts 缺 %s：%s", rule, probe.Counts)
		}
		if i < pos {
			t.Fatalf("counts 中 %s 顺序错：%s", rule, probe.Counts)
		}
		pos = i
	}
}

// TestDeterministicDoubleRun 断言同输入双跑逐字节一致。
func TestDeterministicDoubleRun(t *testing.T) {
	mk := func() []byte {
		r := New(Side{Basename: "a", SHA256: "x"}, Side{Basename: "b", SHA256: "y"})
		r.ResidueBytes = 7
		r.Added = []string{"z"}
		out, err := MarshalCanonical(r)
		if err != nil {
			t.Fatal(err)
		}
		return append(out, RenderMarkdown(r)...)
	}
	a, b := mk(), mk()
	if !bytes.Equal(a, b) {
		t.Fatal("双跑不一致（有非确定性来源）")
	}
}

// TestMarkdownHasNoTimestamps 断言人读视图不含时间性文案（§5 确定性设计）。
func TestMarkdownHasNoTimestamps(t *testing.T) {
	r := New(Side{Basename: "a", SHA256: "x"}, Side{Basename: "b", SHA256: "y"})
	md := string(RenderMarkdown(r))
	for _, bad := range []string{"生成于", "运行时间", "20:2", "UTC", "主机"} {
		if strings.Contains(md, bad) {
			t.Fatalf("report.md 含时间性/身份性文案 %q", bad)
		}
	}
	if !strings.Contains(md, "## 分类计数") {
		t.Fatal("report.md 缺分类计数节")
	}
}

// TestParseBudgetThreeForms 是"--budget 单值 / 逗号多值 / all"三种形态的断言
// （plan/06 §二 决策：三形态各一条测试）。
func TestParseBudgetThreeForms(t *testing.T) {
	single, err := ParseBudget("T1")
	if err != nil || len(single) != 1 || single[0] != "T1" {
		t.Fatalf("单值: %v %v", single, err)
	}
	multi, err := ParseBudget("T1,T2")
	if err != nil || len(multi) != 2 {
		t.Fatalf("逗号多值: %v %v", multi, err)
	}
	all, err := ParseBudget("all")
	if err != nil || len(all) != len(model.RuleIDs) {
		t.Fatalf("all: %v %v", all, err)
	}
	if _, err := ParseBudget("T99"); err == nil {
		t.Fatal("未知规则必须被拒绝")
	}
	if _, err := ParseBudget("T1,"); err != nil {
		t.Fatalf("尾随逗号应被容忍: %v", err)
	}
}

// TestVerdictLineReflectsState 断言 md 的结论行与实际状态一致。
func TestVerdictLineReflectsState(t *testing.T) {
	r := New(Side{}, Side{})
	r.ResidueBytes = 1
	if !strings.Contains(string(RenderMarkdown(r)), "有残留") {
		t.Fatal("residue>0 却未判'有残留'")
	}
	r2 := New(Side{}, Side{})
	r2.MinimalCauseSet = []string{"T1"}
	if !strings.Contains(string(RenderMarkdown(r2)), "仅元数据差异") {
		t.Fatal("最小集非空却未判'仅元数据差异'")
	}
	r3 := New(Side{}, Side{})
	if !strings.Contains(string(RenderMarkdown(r3)), "完全一致") {
		t.Fatal("空集+零残留却未判'完全一致'")
	}
}

// TestBuildDistinguishesNullAndEmpty 是 §4.5 的 `null` / `[]` 区分（K31 的报告侧落点）：
// found=false → JSON null；found=true 且集合为空 → JSON []。两者混淆会让
// "无解"被读成"完全一致"。
func TestBuildDistinguishesNullAndEmpty(t *testing.T) {
	// 无解：必须带 residue>0，否则口径冲突（见下一个断言）
	noSolution, err := Build(Input{ResidueBytes: 1, MinimalCauseSetFound: false})
	if err != nil {
		t.Fatalf("无解 + residue>0 应被接受: %v", err)
	}
	buf, err := MarshalCanonical(noSolution)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf, []byte(`"minimal_cause_set":null`)) {
		t.Fatalf("无解应序列化为 null：%s", buf)
	}

	identical, err := Build(Input{ResidueBytes: 0, MinimalCauseSetFound: true, MinimalCauseSet: nil})
	if err != nil {
		t.Fatalf("完全一致应被接受: %v", err)
	}
	buf, err = MarshalCanonical(identical)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf, []byte(`"minimal_cause_set":[]`)) {
		t.Fatalf("完全一致应序列化为 []：%s", buf)
	}
}

// TestBuildRejectsContradiction 断言 §4.5 的"无解 ⇔ residue_bytes > 0"在构造处就被挡住：
// 自相矛盾的报告不得落盘（静默产出会让下游把"无解"当"零残留"）。
func TestBuildRejectsContradiction(t *testing.T) {
	if _, err := Build(Input{ResidueBytes: 0, MinimalCauseSetFound: false}); err == nil {
		t.Fatal("无解但 residue_bytes = 0 必须报错（§4.5 要求两者同真）")
	}
}

// TestBuildFillsZeroCounts 断言 Build 会补齐缺失的 counts 键（cli 只传非零也无妨）。
func TestBuildFillsZeroCounts(t *testing.T) {
	r, err := Build(Input{
		Counts:               map[string]int{"T1": 3, "total": 3},
		ResidueBytes:         0,
		MinimalCauseSetFound: true,
		MinimalCauseSet:      []string{"T1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range model.RuleIDs {
		if _, ok := r.Counts.Values[rule]; !ok {
			t.Fatalf("counts 缺键 %s", rule)
		}
	}
	if r.Counts.Values["T1"] != 3 || r.Counts.Values["T12"] != 0 || r.Counts.Values["total"] != 3 {
		t.Fatalf("counts 取值错：%v", r.Counts.Values)
	}
}

// TestMarkdownTemplateSections 断言 md 的固定模板骨架（plan/03 §六）：
// 六节齐、改名/新增/删除各有清单、残留逐条列出。
func TestMarkdownTemplateSections(t *testing.T) {
	r, err := Build(Input{
		Left:                 Side{Basename: "l.tar", SHA256: strings.Repeat("a", 64)},
		Right:                Side{Basename: "r.tar", SHA256: strings.Repeat("b", 64)},
		Counts:               map[string]int{"T12": 1, "total": 1},
		Renamed:              []RenamedItem{{From: "pkg/old.py", To: "pkg/new.py"}},
		Added:                []string{"pkg/extra.txt"},
		Removed:              []string{"pkg/legacy.txt"},
		ResidueBytes:         7,
		MinimalCauseSetFound: false,
		ResidualItems:        []ResidualItem{{Path: "pkg/core.py", RuleID: "T12", BytesDifferent: 7}},
	})
	if err != nil {
		t.Fatal(err)
	}
	md := string(RenderMarkdown(r))
	for _, want := range []string{
		"# bdiff 差异归因账本",
		"## 输入", "## 分类计数", "## 改名 / 新增 / 删除",
		"### 改名", "### 新增", "### 删除",
		"## 残留", "## 最小原因集", "## 判定",
		"`pkg/old.py`", "`pkg/new.py`", "`pkg/extra.txt`", "`pkg/legacy.txt`",
		"`pkg/core.py`", "`null`", "有残留",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("report.md 缺 %q：\n%s", want, md)
		}
	}
	if strings.Contains(md, "\r") {
		t.Fatal("report.md 含 CR")
	}
	if !strings.HasSuffix(md, "\n") {
		t.Fatal("report.md 未以 \\n 结尾")
	}
}
