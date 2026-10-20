// Package align 实现结构对齐（模块①，plan/03 §二）。
//
// 口径唯一权威：data/README.md §4.4（改名判定）。纪律：
//   - 条目按 relpath 的 UTF-8 字节序排序（与归档内存储顺序解耦）；
//   - **仅同 sha256 判改名**，不做相似度、不做模糊对齐、不做大小写折叠；
//   - 对齐层只判**结构**（路径/形态/内容归属），内容差异的归因是 attribute 的职责。
//
// 四种状态（本包的精确语义，golden 逐字节钉住）：
//
//	SAME     两侧存在同 relpath（结构对齐成功）。行取**右侧**的 kind/size/sha256；
//	         字节是否相同由 attribute 判定（SAME ≠ 内容一致，见 plan/03 §二）。
//	RENAMED  路径只在单侧出现，且该 sha256 在**两侧各恰好出现 1 次**（§4.4 三条）。
//	         行的 relpath = 右侧（新）路径，renamed_from = 左侧（旧）路径。
//	REMOVED  只在左侧出现（含同名异形态：左侧行记 REMOVED）。
//	ADDED    只在右侧出现（含同名异形态：右侧行记 ADDED）。
//
// 改名为什么按"每哈希"而不是"全局唯一"判：plan/03 §二 的示例输出是
// "同树 + 1 改名 + 1 新增 → ADDED:1, RENAMED:1"。若要求全局只剩一条 REMOVED
// 与一条 ADDED，这个示例永远不可能成立（1 改名 + 1 新增 = 1 REMOVED + 2 ADDED）。
// 因此 §4.4 的"唯一"取**每哈希唯一**：该哈希在两侧各出现 1 次、且两侧各只有
// 一条未匹配条目携带它。同一哈希多处出现 → 不判改名（K10）。
package align

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/artifact"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
)

// SchemaVersion 是 align JSON 的结构版本；字段或语义变更时 +1。
const SchemaVersion = 1

// EmptySHA256 是空内容（目录条目）的 sha256 常量（data/README.md §4.9）。
const EmptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// Result 是 align 的输出。字段顺序 = align JSON 的键序（golden 逐字节断言）。
type Result struct {
	SchemaVersion int           `json:"schema_version"`
	Left          Side          `json:"left"`
	Right         Side          `json:"right"`
	Entries       []model.Entry `json:"entries"`
	Counts        Counts        `json:"counts"`
}

// Side 是一侧的输入摘要：kind / 条目数 / 内容字节数（不写路径）。
type Side struct {
	Kind       string `json:"kind"`
	Entries    int    `json:"entries"`
	TotalBytes int64  `json:"total_bytes"`
}

// Counts 是四类计数，键序固定（plan/03 §二）。
type Counts struct {
	Same    int `json:"SAME"`
	Added   int `json:"ADDED"`
	Removed int `json:"REMOVED"`
	Renamed int `json:"RENAMED"`
}

// Digest 返回条目的 sha256（小写 hex）。
// 目录条目无内容 → EmptySHA256；软链 → 链接目标的 sha256。
func Digest(r artifact.Raw) string {
	sum := sha256.Sum256(r.Data)
	return hex.EncodeToString(sum[:])
}

// Align 对齐两侧条目（算法见包注释与 plan/03 §二）。
func Align(left, right []artifact.Raw) (*Result, error) {
	l := index(left)
	r := index(right)

	res := &Result{
		SchemaVersion: SchemaVersion,
		Left:          Side{Entries: len(l), TotalBytes: contentBytes(left)},
		Right:         Side{Entries: len(r), TotalBytes: contentBytes(right)},
		Entries:       []model.Entry{},
	}

	var removedOnly, addedOnly []model.Entry // 路径只在单侧出现 = 改名候选
	paths := unionPaths(l, r)
	for _, p := range paths {
		le, lok := l[p]
		re, rok := r[p]
		switch {
		case lok && rok && le.Kind == re.Kind:
			// 同 relpath、同形态 → 结构对齐成功（内容差异交给 attribute）
			e := entryOf(re, p)
			e.Status = model.StatusSame
			res.Entries = append(res.Entries, e)
			res.Counts.Same++
		case lok && rok:
			// 同 relpath 但形态不同（file ↔ dir ↔ symlink）：两侧各自记一条。
			// 这不是改名（路径没变），也不能算 SAME（形态是结构差异）。
			lo := entryOf(le, p)
			lo.Status = model.StatusRemoved
			res.Entries = append(res.Entries, lo)
			res.Counts.Removed++
			ro := entryOf(re, p)
			ro.Status = model.StatusAdded
			res.Entries = append(res.Entries, ro)
			res.Counts.Added++
		case lok:
			removedOnly = append(removedOnly, entryOf(le, p))
		default:
			addedOnly = append(addedOnly, entryOf(re, p))
		}
	}

	// 保守改名（§4.4）：
	//   ① 该 sha256 在左侧**全部条目**中出现 1 次、右侧也出现 1 次（K10）；
	//   ② 左侧那条是本次未匹配的（removedOnly），右侧那条也是（addedOnly）；
	//   ③ 两条 sha256 相等（由 ① 的哈希键承担）。
	// 明确不做：相似度、模糊对齐、字节重排匹配。
	leftHash := hashCountsAll(l)
	rightHash := hashCountsAll(r)
	byHash := make(map[string]model.Entry, len(removedOnly))
	for _, e := range removedOnly {
		if leftHash[e.SHA256] == 1 {
			byHash[e.SHA256] = e
		}
	}
	consumed := make(map[string]bool, len(removedOnly)) // 左侧被改名吸收的路径
	absorbed := make(map[string]bool, len(addedOnly))   // 右侧已作为改名行输出的路径
	for _, re := range addedOnly {
		if rightHash[re.SHA256] != 1 || leftHash[re.SHA256] != 1 {
			continue // 同哈希多处出现 → 不判改名（K10）
		}
		le, ok := byHash[re.SHA256]
		if !ok {
			continue
		}
		consumed[le.RelPath] = true
		absorbed[re.RelPath] = true
		re.Status = model.StatusRenamed
		re.RenamedFrom = le.RelPath
		res.Entries = append(res.Entries, re)
		res.Counts.Renamed++
		delete(byHash, re.SHA256) // 一个哈希只吸收一次
	}

	for _, le := range removedOnly {
		if consumed[le.RelPath] {
			continue // 已作为改名的左侧被吸收，不重复计数
		}
		le.Status = model.StatusRemoved
		res.Entries = append(res.Entries, le)
		res.Counts.Removed++
	}
	for _, re := range addedOnly {
		if absorbed[re.RelPath] {
			continue
		}
		re.Status = model.StatusAdded
		res.Entries = append(res.Entries, re)
		res.Counts.Added++
	}

	sortEntries(res.Entries)
	return res, nil
}

func entryOf(r artifact.Raw, path string) model.Entry {
	return model.Entry{
		RelPath: path,
		Kind:    model.Kind(r.Kind),
		Size:    r.Size,
		SHA256:  Digest(r),
	}
}

func index(raws []artifact.Raw) map[string]artifact.Raw {
	m := make(map[string]artifact.Raw, len(raws))
	for _, r := range raws {
		// 同一路径出现两次：后者覆盖。确定性来自调用方稳定的装载顺序
		// （artifact 的 tar/zip 都按归档内顺序读取，dir 按 WalkDir 字典序）。
		m[r.Path] = r
	}
	return m
}

func unionPaths(l, r map[string]artifact.Raw) []string {
	set := make(map[string]bool, len(l)+len(r))
	for p := range l {
		set[p] = true
	}
	for p := range r {
		set[p] = true
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sortPaths(paths)
	return paths
}

// hashCountsAll 统计一侧**全部条目**的 sha256 出现次数（§4.4 ③ 的口径）。
func hashCountsAll(side map[string]artifact.Raw) map[string]int {
	m := make(map[string]int, len(side))
	for _, r := range side {
		m[Digest(r)]++
	}
	return m
}

// contentBytes 是内容字节总数（文件内容 + 软链目标；目录为 0）。
func contentBytes(raws []artifact.Raw) int64 {
	var n int64
	for _, r := range raws {
		n += int64(len(r.Data))
	}
	return n
}

// sortEntries 按 relpath 的 UTF-8 字节序排序。
// 同 relpath 的"同名异形态"两行用 status 作确定性次序（否则不稳定排序会让双跑漂移）。
func sortEntries(entries []model.Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if c := bytes.Compare([]byte(entries[i].RelPath), []byte(entries[j].RelPath)); c != 0 {
			return c < 0
		}
		if entries[i].Status != entries[j].Status {
			return entries[i].Status < entries[j].Status // "ADDED" < "REMOVED"
		}
		return entries[i].SHA256 < entries[j].SHA256
	})
}

func sortPaths(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		return bytes.Compare([]byte(paths[i]), []byte(paths[j])) < 0
	})
}

// RenderMarkdown 渲染人读视图（确定性：无时间性文案、无绝对路径）。
func RenderMarkdown(r *Result) []byte {
	var b strings.Builder
	b.WriteString("# bdiff 结构对齐\n\n")
	b.WriteString("## 输入\n\n| 侧 | kind | 条目数 | 总字节 |\n|---|---|---|---|\n")
	fmt.Fprintf(&b, "| left | `%s` | %d | %d |\n", r.Left.Kind, r.Left.Entries, r.Left.TotalBytes)
	fmt.Fprintf(&b, "| right | `%s` | %d | %d |\n", r.Right.Kind, r.Right.Entries, r.Right.TotalBytes)
	b.WriteString("\n## 计数\n\n| SAME | ADDED | REMOVED | RENAMED |\n|---|---|---|---|\n")
	fmt.Fprintf(&b, "| %d | %d | %d | %d |\n", r.Counts.Same, r.Counts.Added, r.Counts.Removed, r.Counts.Renamed)
	b.WriteString("\n## 条目\n\n| relpath | status | kind | size | sha256 |\n|---|---|---|---|---|\n")
	for _, e := range r.Entries {
		path := e.RelPath
		if e.RenamedFrom != "" {
			path = e.RenamedFrom + " → " + e.RelPath
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %d | `%s` |\n", path, e.Status, e.Kind, e.Size, e.SHA256)
	}
	return []byte(b.String())
}
