// Package minimize 实现最小原因集搜索（模块④，plan/03 §五）。
//
// 算法口径（data/README.md §4.5）：按 |S| 升序枚举，同基数按 rule id 字典序；
// 不做剪枝、不做随机化；命中即停（首个命中即最小）。
package minimize

import (
	"errors"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
)

// ErrNotImplemented 表示搜索尚未落地（M4）。
var ErrNotImplemented = errors.New("minimize 尚未实现（计划于 M4 落地）")

// MaxSetSize 是 |S| 的上界（data/README.md §4.5）。
// 由 model.RuleIDs 的长度派生，不写魔数——新增规则时自动跟上。
var MaxSetSize = len(model.RuleIDs)

// Search 返回使两侧树摘要相等的基数最小的规则集合。
//
// 返回：
//   - (set, true, nil)  找到解（set 可能为空切片 = 完全一致）
//   - (nil, false, nil) 无解（调用方必须同时满足 residue_bytes > 0）
func Search(leftDigest func([]string) string, rightDigest func([]string) string) ([]string, bool, error) {
	return nil, false, ErrNotImplemented
}

// Combinations 按既定顺序枚举全集的所有子集：先按基数升序，同基数按字典序。
// 该函数是纯函数且**已实现**（M0）——它只依赖 model.RuleIDs，与搜索引擎解耦，
// 因此可以先钉住"枚举顺序"这条口径（M4 的搜索直接消费它）。
func Combinations() [][]string {
	all := model.RuleIDs
	n := len(all)
	var out [][]string
	for size := 0; size <= MaxSetSize; size++ {
		for _, idx := range choose(n, size) {
			set := make([]string, 0, size)
			for _, i := range idx {
				set = append(set, all[i])
			}
			out = append(out, set)
		}
	}
	return out
}

// choose 按字典序返回从 n 个元素中取 k 个的全部下标组合。
func choose(n, k int) [][]int {
	var out [][]int
	cur := make([]int, 0, k)
	var rec func(start int)
	rec = func(start int) {
		if len(cur) == k {
			out = append(out, append([]int(nil), cur...))
			return
		}
		for i := start; i < n; i++ {
			cur = append(cur, i)
			rec(i + 1)
			cur = cur[:len(cur)-1]
		}
	}
	rec(0)
	return out
}
