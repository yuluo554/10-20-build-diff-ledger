package attribute

import (
	"encoding/json"
	"testing"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/normalize"
)

func sideOf(entries ...EntryInput) SideInput { return SideInput{Kind: "tar", Entries: entries} }

func e(path string, index int, data string, facts map[string]string) EntryInput {
	return EntryInput{Path: path, Index: index, Data: []byte(data), Facts: facts}
}

// TestIdenticalSides 断言同输入 → total=0、residue=0、三摘要一致。
func TestIdenticalSides(t *testing.T) {
	in := sideOf(e("pkg/a", 0, "aaa", map[string]string{"tar.mtime": "1"}))
	res, err := Attribute(in, in)
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts.Values["total"] != 0 {
		t.Fatalf("total = %d，期望 0：%+v", res.Counts.Values["total"], res.Diffs)
	}
	if res.ResidueBytes != 0 {
		t.Fatalf("residue = %d，期望 0", res.ResidueBytes)
	}
	if res.TreeDigest.Left != res.TreeDigest.Right {
		t.Fatalf("同输入摘要不一致：%+v", res.TreeDigest)
	}
}

// TestT4StripsPrefixBeforeContent 钉住"T4 先于内容比对"（HANDOFF-M3 的已知难点）：
// 两侧全部 relpath 带不同前缀时，只应记一条 T4，不应炸成大量增删/T12。
func TestT4StripsPrefixBeforeContent(t *testing.T) {
	l := sideOf(
		e("build-a/pkg", 0, "", nil),
		e("build-a/pkg/a.txt", 1, "same", map[string]string{"tar.mtime": "1"}),
	)
	r := sideOf(
		e("build-b/pkg", 0, "", nil),
		e("build-b/pkg/a.txt", 1, "same", map[string]string{"tar.mtime": "1"}),
	)
	res, err := Attribute(l, r)
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts.Values["T4"] != 1 || res.Counts.Values["total"] != 1 {
		t.Fatalf("期望只命中 1 条 T4，实得 %+v：%+v", res.Counts.Values, res.Diffs)
	}
	if res.ResidueBytes != 0 {
		t.Fatalf("residue = %d，期望 0", res.ResidueBytes)
	}
	// T4 必须让树摘要相等（minimal_cause_set=[T4] 的前提）。
	le, _ := Entries(l)
	re, _ := Entries(r)
	if d1, d2 := digestOf([]string{"T4"}, le), digestOf([]string{"T4"}, re); d1 != d2 {
		t.Fatalf("施加 T4 后摘要仍不等：%s vs %s", d1, d2)
	}
}

// TestRuleOverlapNote 钉住 §4.1 闭合性：zip.external_attr 的权限位与非权限位
// 同时变化 → 取首个命中的 T3，并在 notes 记 rule_overlap。
func TestRuleOverlapNote(t *testing.T) {
	// 左 0644；右 0755 + DOS archive 位 0x20。
	l := sideOf(e("a", 0, "x", map[string]string{"zip.external_attr": "27525120"}))
	r := sideOf(e("a", 0, "x", map[string]string{"zip.external_attr": "32309280"}))
	res, err := Attribute(l, r)
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts.Values["T3"] != 1 {
		t.Fatalf("期望命中 T3，实得 %+v", res.Counts.Values)
	}
	if len(res.Notes) != 1 || res.Notes[0] != "rule_overlap" {
		t.Fatalf("notes = %v，期望 [rule_overlap]", res.Notes)
	}
}

// TestOnlyOtherExternalAttrBitsIsT7：只动非权限位 → T7（不记 rule_overlap）。
func TestOnlyOtherExternalAttrBitsIsT7(t *testing.T) {
	l := sideOf(e("a", 0, "x", map[string]string{"zip.external_attr": "27525120"}))
	r := sideOf(e("a", 0, "x", map[string]string{"zip.external_attr": "27525152"})) // +0x20
	res, err := Attribute(l, r)
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts.Values["T7"] != 1 || len(res.Notes) != 0 {
		t.Fatalf("期望 T7 且无 notes，实得 %+v notes=%v", res.Counts.Values, res.Notes)
	}
}

// TestResidueIsByteCountNotLengthDelta：同长度 1 字节差异 → residue=1；长度差也计入。
func TestResidueIsByteCountNotLengthDelta(t *testing.T) {
	l := sideOf(e("a", 0, "abc", nil))
	r := sideOf(e("a", 0, "abd", nil))
	res, _ := Attribute(l, r)
	if res.ResidueBytes != 1 {
		t.Fatalf("residue = %d，期望 1", res.ResidueBytes)
	}
	r2 := sideOf(e("a", 0, "abcde", nil))
	res2, _ := Attribute(l, r2)
	// len("abc")=3、len("abcde")=5，共同前缀全同 → 差异 = 长度差 2。
	if res2.ResidueBytes != 2 {
		t.Fatalf("长度差 residue = %d，期望 2", res2.ResidueBytes)
	}
	if res2.Counts.Values["T12"] == 0 {
		t.Fatal("内容差异必须落 T12")
	}
}

// TestT12ImpliesResidue 是 §4.1 的硬断言：T12 出现 ⇒ residue_bytes > 0。
func TestT12ImpliesResidue(t *testing.T) {
	l := sideOf(e("a", 0, "x", map[string]string{"macho.uuid": "aa"}))
	r := sideOf(e("a", 0, "x", map[string]string{"macho.uuid": "bb"}))
	res, err := Attribute(l, r)
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts.Values["T12"] == 0 {
		t.Fatalf("未知 key 差异必须落 T12：%+v", res.Counts.Values)
	}
	if res.ResidueBytes <= 0 {
		t.Fatalf("T12 出现但 residue = %d（§4.1 要求 > 0）", res.ResidueBytes)
	}
}

// TestCountsKeyOrder 钉住 counts 的序列化键序：T1…T12 后接 total。
func TestCountsKeyOrder(t *testing.T) {
	c := NewCounts()
	c.Add("T12")
	c.Add("T1")
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"T1":1,"T2":0,"T3":0,"T4":0,"T5":0,"T6":0,"T7":0,"T8":0,"T9":0,"T10":0,"T11":0,"T12":1,"total":2}`
	if string(b) != want {
		t.Fatalf("counts 键序不符\n got=%s\nwant=%s", b, want)
	}
	var back Counts
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Values["total"] != 2 {
		t.Fatalf("Unmarshal 后 total = %d", back.Values["total"])
	}
}

func digestOf(set []string, entries []normalize.Entry) string {
	return normalize.TreeDigest(set, entries)
}
