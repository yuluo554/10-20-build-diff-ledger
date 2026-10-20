// Package align 实现结构对齐（模块①，plan/03 §二）。
//
// M0 状态：契约已定，算法在 M1 落地。这里的占位实现让 CLI 可运行、
// 让「未实现」的错误显式可见——不做"看起来成功但返回空表"的假绿。
package align

import (
	"errors"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/artifact"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
)

// ErrNotImplemented 表示 align 算法尚未落地（M1）。
var ErrNotImplemented = errors.New("align 尚未实现（计划于 M1 落地）")

// Result 是 align 的输出。
type Result struct {
	LeftKind  string        `json:"left_kind"`
	RightKind string        `json:"right_kind"`
	Entries   []model.Entry `json:"entries"`
	Counts    Counts        `json:"counts"`
}

// Counts 是四类计数，键序固定（plan/03 §二）。
type Counts struct {
	Same    int `json:"SAME"`
	Added   int `json:"ADDED"`
	Removed int `json:"REMOVED"`
	Renamed int `json:"RENAMED"`
}

// Align 对齐两侧条目。M1 实现：
//  1. 规范化 relpath（ToSlash → 去前导 "./" → 折叠重复 "/" → 拒绝 ".." 越界）
//  2. 按 relpath 字节序排序
//  3. 求交并集，按 SAME/ADDED/REMOVED 分类
//  4. 仅当「对侧唯一且 sha256 相等且该哈希两侧各出现 1 次」时判 RENAMED
func Align(left, right []artifact.Raw) (*Result, error) {
	return nil, ErrNotImplemented
}
