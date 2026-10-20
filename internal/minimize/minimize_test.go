package minimize

import (
	"reflect"
	"testing"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
)

// TestCombinationsOrder 断言枚举顺序口径：先按基数升序，同基数按字典序。
// 这是 M4"首个命中即最小"的前提——顺序错了，最小性就失去可复算性。
func TestCombinationsOrder(t *testing.T) {
	combos := Combinations()
	if len(combos) != 4096 {
		t.Fatalf("子集数 = %d，期望 2^12 = 4096", len(combos))
	}
	if len(combos[0]) != 0 {
		t.Fatalf("首项应为空集，实为 %v", combos[0])
	}
	// 逐项检查：基数单调不减；同基数保持字典序
	prevLen := 0
	prev := []string{}
	for i, c := range combos {
		if len(c) < prevLen {
			t.Fatalf("第 %d 项基数 %d < 前一项 %d（未按基数升序）", i, len(c), prevLen)
		}
		if len(c) == prevLen && prevLen > 0 && !lexLess(prev, c) {
			t.Fatalf("同基数未按字典序: %v 之后是 %v", prev, c)
		}
		prevLen = len(c)
		prev = c
	}
	// 抽样定点：第 1 项 = [T1]，第 2 项 = [T2]，末项 = 全集
	if got := combos[1]; len(got) != 1 || got[0] != "T1" {
		t.Errorf("第 1 项 = %v，期望 [T1]", got)
	}
	if got := combos[2]; len(got) != 1 || got[0] != "T2" {
		t.Errorf("第 2 项 = %v，期望 [T2]", got)
	}
	last := combos[len(combos)-1]
	if len(last) != len(model.RuleIDs) {
		t.Errorf("末项基数 = %d，期望 %d（全集）", len(last), len(model.RuleIDs))
	}
}

// TestCombinationsAreUnique 防"重复枚举让搜索退化成非确定性"。
func TestCombinationsAreUnique(t *testing.T) {
	seen := make(map[string]bool, 4096)
	for _, c := range Combinations() {
		key := ""
		for _, r := range c {
			key += r + ","
		}
		if seen[key] {
			t.Fatalf("重复子集: %v", c)
		}
		seen[key] = true
	}
	if len(seen) != 4096 {
		t.Fatalf("唯一子集数 = %d，期望 4096", len(seen))
	}
}

// TestMaxSetSizeFollowsRuleSet 断言上界由规则集派生（无魔数）。
func TestMaxSetSizeFollowsRuleSet(t *testing.T) {
	if MaxSetSize != len(model.RuleIDs) {
		t.Fatalf("MaxSetSize=%d，规则数=%d（上界必须派生）", MaxSetSize, len(model.RuleIDs))
	}
	if MaxSetSize > 12 {
		t.Fatalf("上界 %d 超过 §4.5 的 12；扩表要同步改口径与文档", MaxSetSize)
	}
}

// TestSearchIdenticalReturnsEmptySet 断言"完全一致"返回**非 nil 空集**，而不是无解。
// 二者在报告里分别是 `[]` 与 `null`，混淆会让"一致"变成"有残留"。
func TestSearchIdenticalReturnsEmptySet(t *testing.T) {
	set, ok, err := Search(
		func([]string) string { return "same" },
		func([]string) string { return "same" },
	)
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if !ok {
		t.Fatal("两侧恒相等时应 ok=true（S=∅ 是合法解）")
	}
	if set == nil {
		t.Fatal("完全一致必须返回空切片（非 nil），以便区分 report.json 的 [] 与 null")
	}
	if len(set) != 0 {
		t.Fatalf("期望空集，实得 %v", set)
	}
}

// digestPair 构造一对摘要函数：满足 sat 时两侧都返回 "same"（相等），
// 否则分别返回 "left"/"right"（不等）。
// 注意两侧不能共用同一个闭包：那样未满足时也是 "x"=="x"，搜索会在空集命中。
func digestPair(sat func([]string) bool) (func([]string) string, func([]string) string) {
	left := func(set []string) string {
		if sat(set) {
			return "same"
		}
		return "left"
	}
	right := func(set []string) string {
		if sat(set) {
			return "same"
		}
		return "right"
	}
	return left, right
}

// TestSearchFindsMinimalPairAndNoSuperset 是 M4 的核心最小性断言：
// 只有 S ⊇ {T1,T2} 才能归一化时，首个命中必须是 [T1,T2]（len==2，不是超集）。
func TestSearchFindsMinimalPairAndNoSuperset(t *testing.T) {
	l, r := digestPair(func(set []string) bool {
		return contains(set, "T1") && contains(set, "T2")
	})
	set, ok, err := Search(l, r)
	if err != nil || !ok {
		t.Fatalf("应找到解：set=%v ok=%v err=%v", set, ok, err)
	}
	if want := []string{"T1", "T2"}; !reflect.DeepEqual(set, want) {
		t.Fatalf("最小集 = %v，期望 %v（显式非超集）", set, want)
	}
	if len(set) != 2 {
		t.Fatalf("最小集基数 = %d，期望 2", len(set))
	}
}

// TestSearchPrefersSmallerCardinality 断言"基数优先于字典序"：
// {T2} 与 {T1,T2} 都成立时，必须返回单元素 {T2}，不能因为字典序先遇到而返回大集合。
func TestSearchPrefersSmallerCardinality(t *testing.T) {
	l, r := digestPair(func(set []string) bool { return contains(set, "T2") })
	set, ok, err := Search(l, r)
	if err != nil || !ok {
		t.Fatalf("应找到解：set=%v ok=%v err=%v", set, ok, err)
	}
	if want := []string{"T2"}; !reflect.DeepEqual(set, want) {
		t.Fatalf("最小集 = %v，期望 %v（基数最小优先）", set, want)
	}
}

// TestSearchLexicographicTieBreak 断言同基数取编号序第一个：
// {T1} 与 {T2} 都能成立时返回 {T1}。
func TestSearchLexicographicTieBreak(t *testing.T) {
	l, r := digestPair(func(set []string) bool {
		return contains(set, "T1") || contains(set, "T2")
	})
	set, ok, err := Search(l, r)
	if err != nil || !ok {
		t.Fatalf("应找到解：set=%v ok=%v err=%v", set, ok, err)
	}
	if want := []string{"T1"}; !reflect.DeepEqual(set, want) {
		t.Fatalf("并列时取字典序最小 = %v，期望 %v", set, want)
	}
}

// TestSearchNeverReturnsT12 断言 T12 不会混进最小集：
// T12 没有 op，若 S∪{T12} 成立则 S 也成立且更小，首个命中不可能带 T12。
func TestSearchNeverReturnsT12(t *testing.T) {
	l, r := digestPair(func(set []string) bool { return contains(set, "T2") })
	set, ok, err := Search(l, r)
	if err != nil || !ok {
		t.Fatalf("应找到解：set=%v ok=%v err=%v", set, ok, err)
	}
	for _, rule := range set {
		if rule == "T12" {
			t.Fatalf("最小集 %v 含 T12（T12 无 op，不可能是最小集成员）", set)
		}
	}
}

// TestSearchNoSolutionIsExplicit 断言无解返回 (nil, false, nil)——
// 既不是空集（会伪装成一致），也不是 error（error 属于入参非法）。
func TestSearchNoSolutionIsExplicit(t *testing.T) {
	set, ok, err := Search(
		func([]string) string { return "left" },
		func([]string) string { return "right" },
	)
	if err != nil {
		t.Fatalf("无解不应报错: %v", err)
	}
	if ok {
		t.Fatalf("无解时不得 ok=true（set=%v）", set)
	}
	if set != nil {
		t.Fatalf("无解时 set 必须是 nil，实得 %v", set)
	}
}

// TestSearchIsExhaustiveWithoutPruning 断言"不剪枝"：
// 无解时必须把 2^12 个候选全部算过，且每个候选每侧恰好一次。
// 任何"启发式跳过"都会让计数小于 4096。
func TestSearchIsExhaustiveWithoutPruning(t *testing.T) {
	var leftCalls, rightCalls int
	seen := map[string]int{}
	set, ok, err := Search(
		func(s []string) string { leftCalls++; seen["L"+key(s)]++; return "left" },
		func(s []string) string { rightCalls++; seen["R"+key(s)]++; return "right" },
	)
	if err != nil || ok || set != nil {
		t.Fatalf("期望无解：set=%v ok=%v err=%v", set, ok, err)
	}
	if leftCalls != 4096 || rightCalls != 4096 {
		t.Fatalf("摘要调用次数 = %d/%d，期望各 4096（不剪枝、无重复）", leftCalls, rightCalls)
	}
	for k, n := range seen {
		if n != 1 {
			t.Fatalf("候选 %q 被计算了 %d 次（应恰好 1 次）", k, n)
		}
	}
}

// TestSearchNilDigestIsExplicit 断言 nil 摘要函数报错而非静默返回空集。
func TestSearchNilDigestIsExplicit(t *testing.T) {
	ok := func([]string) string { return "x" }
	if _, _, err := Search(nil, ok); err == nil {
		t.Fatal("leftDigest=nil 必须报错")
	}
	if _, _, err := Search(ok, nil); err == nil {
		t.Fatal("rightDigest=nil 必须报错")
	}
}

// TestSearchIsDeterministic 断言同输入双跑给出同一集合。
func TestSearchIsDeterministic(t *testing.T) {
	l, r := digestPair(func(set []string) bool {
		return contains(set, "T8") && contains(set, "T11")
	})
	a, ok1, _ := Search(l, r)
	b, ok2, _ := Search(l, r)
	if ok1 != ok2 || !reflect.DeepEqual(a, b) {
		t.Fatalf("双跑不一致：%v/%v vs %v/%v", a, ok1, b, ok2)
	}
	if want := []string{"T8", "T11"}; !reflect.DeepEqual(a, want) {
		t.Fatalf("最小集 = %v，期望 %v", a, want)
	}
}

func contains(set []string, rule string) bool {
	for _, r := range set {
		if r == rule {
			return true
		}
	}
	return false
}

func key(set []string) string {
	out := ""
	for _, r := range set {
		out += r + ","
	}
	return out
}

// lexLess 比较两个同长度规则集合的字典序（按 model.RuleIDs 的编号顺序）。
func lexLess(a, b []string) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return ruleIndex(a[i]) < ruleIndex(b[i])
		}
	}
	return len(a) < len(b)
}

func ruleIndex(rule string) int {
	for i, r := range model.RuleIDs {
		if r == rule {
			return i
		}
	}
	return 99
}
