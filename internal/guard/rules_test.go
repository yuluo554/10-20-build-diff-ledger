package guard

import (
	"strings"
	"testing"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
)

// TestRuleSetClosed 断言闭合分类法（data/README.md §4.1）：
// 恰好 12 条、编号连续 T1–T12、T12 在末尾、无重复。
// 变异验证：从 model.RuleIDs 删一条 → 本测试红。
func TestRuleSetClosed(t *testing.T) {
	ids := model.RuleIDs
	if len(ids) != 12 {
		t.Fatalf("规则数 = %d，期望 12", len(ids))
	}
	seen := map[string]bool{}
	for i, id := range ids {
		want := "T" + itoa(i+1)
		if id != want {
			t.Errorf("第 %d 条 = %q，期望 %q（编号必须连续）", i, id, want)
		}
		if seen[id] {
			t.Errorf("重复规则 %q", id)
		}
		seen[id] = true
	}
	if ids[len(ids)-1] != "T12" {
		t.Errorf("末条 = %q，期望 T12（未解释必须是兜底）", ids[len(ids)-1])
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b strings.Builder
	var digits []byte
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for i := len(digits) - 1; i >= 0; i-- {
		b.WriteByte(digits[i])
	}
	return b.String()
}
