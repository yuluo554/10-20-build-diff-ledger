// Package attribute 实现归因引擎（模块③，plan/03 §四）。
//
// 分类法 T1–T12 的触发条件与优先级只在 data/README.md §4.1 定义。
package attribute

import (
	"errors"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
)

// ErrNotImplemented 表示归因尚未落地（M3）。
var ErrNotImplemented = errors.New("attribute 尚未实现（计划于 M3 落地）")

// Counts 是 T1–T12 逐类计数 + total。键序固定（data/README.md §4.8）。
type Counts struct {
	Values map[string]int `json:"-"`
}

// NewCounts 建一张全 0 的计数表，键集合来自 model.RuleIDs（单点定义）。
func NewCounts() Counts {
	m := make(map[string]int, len(model.RuleIDs)+1)
	for _, r := range model.RuleIDs {
		m[r] = 0
	}
	m["total"] = 0
	return Counts{Values: m}
}

// Add 记一次命中并维护 total。
func (c Counts) Add(rule string) {
	c.Values[rule]++
	c.Values["total"]++
}

// Result 是 attribute 的输出。
type Result struct {
	Counts       Counts       `json:"counts"`
	Diffs        []model.Diff `json:"diffs"`
	ResidueBytes int64        `json:"residue_bytes"`
	TreeDigest   TreeDigest   `json:"tree_digest"`
}

// TreeDigest 记录三个摘要（§4.5）。
type TreeDigest struct {
	Left          string `json:"left"`
	Right         string `json:"right"`
	NormalizedAll string `json:"normalized_all"`
}

// Attribute 对两侧 facts 做归因。M3 实现。
//
// 时序纪律：差异点按 (relpath, key, rule_id) 排序；residue_bytes 由实际字节
// 比对得出，不由分类推导。
func Attribute(left, right []model.Fact) (*Result, error) {
	return nil, ErrNotImplemented
}
