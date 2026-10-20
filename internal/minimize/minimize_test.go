package minimize

import (
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

// TestSearchNotImplementedIsExplicit 断言未落地时**报错而非返回空集**。
// 返回空集会伪装成"完全一致"，是静默通过的经典漏洞。
func TestSearchNotImplementedIsExplicit(t *testing.T) {
	set, ok, err := Search(func([]string) string { return "a" }, func([]string) string { return "a" })
	if err == nil {
		t.Fatalf("未落地时 Search 必须报错，实得 set=%v ok=%v", set, ok)
	}
	if ok {
		t.Fatal("未落地时不得返回 ok=true")
	}
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
