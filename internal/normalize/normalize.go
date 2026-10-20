// Package normalize 提供 T1–T11 的纯函数归一化 op 与树摘要。
//
// 口径唯一权威：data/README.md §4.2（op 语义）、§4.5（tree_digest 拼接规则）。
// 纪律：纯函数、幂等、不读时钟/环境/随机；归一化在内存完成，不落盘。
package normalize

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// Entry 是归一化作用的载体：一个条目的路径 + 内容字节 + 事实集合。
type Entry struct {
	Path  string
	Bytes []byte
	Facts map[string]string
}

// Op 是一个归一化操作。必须是纯函数且幂等。
type Op func(Entry) Entry

// Ops 是规则编号 → 归一化 op。T12 没有 op（不可归一化，出现即 residue>0）。
//
// M0 状态：op 表已定型（键集合与 data/README.md §4.1 的 T1–T11 一一对应），
// 具体语义在 M3 落地。未落地的 op 为 nil，Apply 会跳过并在结果里保持可见。
var Ops = map[string]Op{
	"T1": nil, "T2": nil, "T3": nil, "T4": nil,
	"T5": nil, "T6": nil, "T7": nil, "T8": nil,
	"T9": nil, "T10": nil, "T11": nil,
}

// Apply 按 rule id 升序施加集合 S 的归一化（§4.2「可组合」）。
// 顺序固定，不对交换性作假设。
func Apply(set []string, entries []Entry) []Entry {
	ordered := append([]string(nil), set...)
	sort.Strings(ordered)
	out := make([]Entry, len(entries))
	for i, e := range entries {
		out[i] = cloneEntry(e)
	}
	for _, rule := range ordered {
		op, ok := Ops[rule]
		if !ok || op == nil {
			continue // T12 或未落地：显式跳过，不清空差异
		}
		for i := range out {
			out[i] = op(out[i])
		}
	}
	return out
}

func cloneEntry(e Entry) Entry {
	c := Entry{Path: e.Path}
	if e.Bytes != nil {
		c.Bytes = append([]byte(nil), e.Bytes...)
	}
	if e.Facts != nil {
		c.Facts = make(map[string]string, len(e.Facts))
		for k, v := range e.Facts {
			c.Facts[k] = v
		}
	}
	return c
}

// TreeDigest 计算集合 S 下的树摘要（data/README.md §4.5）：
//
//	sha256( concat_{relpath 升序} ( relpath ‖ 0x00 ‖ sha256(entry) ‖ 0x0A ) )
//
// 排序用 relpath 的 UTF-8 字节序；分隔符写死为 0x00 与 0x0A。
// 这是 tree_digest 的**唯一实现**（attribute 与 minimize 都调它）。
func TreeDigest(set []string, entries []Entry) string {
	ordered := Apply(set, entries)
	sorted := append([]Entry(nil), ordered...)
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare([]byte(sorted[i].Path), []byte(sorted[j].Path)) < 0
	})

	h := sha256.New()
	for _, e := range sorted {
		h.Write([]byte(e.Path))
		h.Write([]byte{0x00})
		sum := sha256.Sum256(e.Bytes)
		h.Write([]byte(hex.EncodeToString(sum[:])))
		h.Write([]byte{0x0A})
	}
	return hex.EncodeToString(h.Sum(nil))
}
