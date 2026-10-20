// Package model 定义五个模块之间共享的数据契约（叶子包，不 import 任何内部包）。
//
// 纪律（plan/03 §七）：这些类型不带指针字段，便于深比较与 golden 断言。
package model

// Kind 是条目的形态。
type Kind string

const (
	KindFile Kind = "file"
	KindDir  Kind = "dir"
	KindLink Kind = "symlink"
)

// AlignStatus 是 align 阶段给每个条目的状态。
type AlignStatus string

const (
	StatusSame    AlignStatus = "SAME"
	StatusAdded   AlignStatus = "ADDED"
	StatusRemoved AlignStatus = "REMOVED"
	StatusRenamed AlignStatus = "RENAMED"
)

// Entry 是结构对齐后的一个规范化条目。
type Entry struct {
	RelPath string      `json:"relpath"`
	Status  AlignStatus `json:"status"`
	Kind    Kind        `json:"kind"`
	Size    int64       `json:"size"`
	SHA256  string      `json:"sha256"`
	// RenamedFrom 只在 Status==RENAMED 时有值，记录对侧的原路径。
	RenamedFrom string `json:"renamed_from,omitempty"`
}

// Fact 是一条扁平事实记录。
type Fact struct {
	Path  string `json:"path"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Diff 是一个差异点。
type Diff struct {
	Path       string `json:"path"`
	Key        string `json:"key"`
	RuleID     string `json:"rule_id"`
	Left       string `json:"left"`
	Right      string `json:"right"`
	BytesDelta int64  `json:"bytes_delta"`
}

// RuleIDs 是闭合分类法的全部规则编号，顺序即优先级顺序（data/README.md §4.1）。
// 代码里只此一处列出全集；新增规则必须同时改 data/README.md §4.1 与这里的长度断言。
var RuleIDs = []string{
	"T1", "T2", "T3", "T4", "T5", "T6",
	"T7", "T8", "T9", "T10", "T11", "T12",
}
