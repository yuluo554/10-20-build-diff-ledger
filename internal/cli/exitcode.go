package cli

import (
	"fmt"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/report"
)

// 本文件是退出码判定的**唯一实现**（plan/03 §六）：判定只看 report.Report，
// 不再另读 attribute.Result——否则"报告里写的"与"退出码依据的"会分叉。

// FailOn 是 `--fail-on` 的两形态（data/README.md §4.6）。
type FailOn string

const (
	// FailOnAny：0/2/3 全部按 §4.6 语义返回（默认）。
	FailOnAny FailOn = "any"
	// FailOnResidue：只有残留（3）算失败；"仅元数据差异"降为 0。
	// 它**只能**影响 0/2，不得把 3 降级（与 --budget 同一条纪律）。
	FailOnResidue FailOn = "residue"
)

func parseFailOn(s string) (FailOn, error) {
	switch s {
	case "", string(FailOnAny):
		return FailOnAny, nil
	case string(FailOnResidue):
		return FailOnResidue, nil
	}
	return "", fmt.Errorf("--fail-on 只接受 any / residue（收到 %q）", s)
}

// compareExitCode 按 data/README.md §4.6 判定（顺序即优先级）：
//
//	有残留 或 T12 出现   → 3
//	residue==0 且 S==∅   → 0（完全一致）
//	residue==0 且 S≠∅    → 实际类别 ⊆ budget ? 0 : 2
//	（--fail-on=residue 时 2 → 0；3 永不降级）
//
// S == ∅ 与"无解"必须区分：无解（nil）在 Build 里已与 residue>0 绑定，
// 走到这里的 nil 只可能是口径被破坏，因此按"非 ∅"处理、落到 2/3 分支更安全。
func compareExitCode(r *report.Report, budget []string, failOn FailOn) int {
	if r.ResidueBytes > 0 || r.Counts.Values["T12"] > 0 {
		return ExitResidue
	}
	if r.MinimalCauseSet != nil && len(r.MinimalCauseSet) == 0 {
		return ExitOK
	}
	if budgetCovers(r, budget) {
		return ExitOK
	}
	if failOn == FailOnResidue {
		return ExitOK
	}
	return ExitMetaOnly
}

// budgetCovers 判断**实际出现过的类别**（counts>0 的 Tn）是否都在白名单内。
// 白名单覆盖的是实际类别，不是最小集：最小集只说明"最少改哪几项就一致"。
func budgetCovers(r *report.Report, budget []string) bool {
	if len(budget) == 0 {
		return false
	}
	allowed := make(map[string]bool, len(budget))
	for _, rule := range budget {
		allowed[rule] = true
	}
	for rule, n := range r.Counts.Values {
		if rule == "total" || n == 0 {
			continue
		}
		if !allowed[rule] {
			return false
		}
	}
	return true
}
