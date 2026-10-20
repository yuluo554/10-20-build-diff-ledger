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
