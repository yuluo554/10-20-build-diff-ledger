// Package attribute 实现归因引擎（模块③，plan/03 §四）。
//
// 分类法 T1–T12 的触发条件与优先级只在 data/README.md §4.1 定义；
// 归一化 op 的语义在 §4.2，entry_stream / tree_digest 在 §4.5。
//
// 输入：两侧条目的 (path, 归档内序号, 内容字节, facts)。facts 由 internal/facts
// 产出（容器级事实的 path 为空串）。本包**不 import artifact**（K4 依赖方向），
// 装载与 facts 抽取由调用方（internal/cli）完成。
package attribute

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/normalize"
)

// 容器级事实的 path 约定（data/README.md §4.10）。
const containerPath = ""

// EntryInput 是一侧的一个条目。
type EntryInput struct {
	Path  string
	Kind  model.Kind
	Index int // 归档内序号（目录输入 = -1）
	Data  []byte
	Facts map[string]string
}

// SideInput 是一侧的输入：容器形态 + 条目（容器级事实作为 Path="" 的条目）。
type SideInput struct {
	Kind    string
	Entries []EntryInput
}

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

// MarshalJSON 按 model.RuleIDs 的顺序输出 T1…T12 再接 total（键序固定）。
func (c Counts) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i := 0; i < len(model.RuleIDs); i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%q:%d", model.RuleIDs[i], c.Values[model.RuleIDs[i]])
	}
	fmt.Fprintf(&b, ",%q:%d", "total", c.Values["total"])
	b.WriteByte('}')
	return b.Bytes(), nil
}

// UnmarshalJSON 反向解析（golden / 测试用）。
func (c *Counts) UnmarshalJSON(data []byte) error {
	var raw map[string]int
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Values = raw
	return nil
}

// Result 是 attribute 的输出。字段顺序 = JSON 键序。
type Result struct {
	Counts       Counts       `json:"counts"`
	Diffs        []model.Diff `json:"diffs"`
	ResidueBytes int64        `json:"residue_bytes"`
	TreeDigest   TreeDigest   `json:"tree_digest"`
	Notes        []string     `json:"notes"`
}

// TreeDigest 记录三个摘要（§4.5）。
type TreeDigest struct {
	Left          string `json:"left"`
	Right         string `json:"right"`
	NormalizedAll string `json:"normalized_all"`
}

// Entries 把一侧输入转成 normalize.Entry（含公共根前缀与归档内序号）。
// 供树摘要复算与 M4 的最小原因集搜索复用；与 Attribute 内部的预处理同一实现。
func Entries(side SideInput) ([]normalize.Entry, error) {
	s, err := prepare(side)
	if err != nil {
		return nil, err
	}
	return s.entries, nil
}

// Attribute 对两侧做归因。
//
// 时序纪律：T4（路径前缀）先于内容比对；差异点按 (relpath, key, rule_id) 排序；
// residue_bytes 由归一化后的 entry_stream 逐字节比对得出，不由分类推导。
func Attribute(left, right SideInput) (*Result, error) {
	l, err := prepare(left)
	if err != nil {
		return nil, err
	}
	r, err := prepare(right)
	if err != nil {
		return nil, err
	}

	res := &Result{
		Counts: NewCounts(),
		Diffs:  []model.Diff{},
		Notes:  []string{},
	}

	// T4：两侧公共根前缀不同 → 先记一条树级差异（先于逐条目比对）。
	if l.prefix != r.prefix {
		res.Diffs = append(res.Diffs, model.Diff{
			Path: "", Key: "relpath.prefix", RuleID: "T4",
			Left: l.prefix, Right: r.prefix, BytesDelta: 0,
		})
		res.Counts.Add("T4")
	}

	// T5 的前提：条目集合与内容哈希完全相同，只有存储顺序可能不同。
	t5Applicable := sameEntrySet(l, r)

	paths := unionKeys(l.byPath, r.byPath)
	overlap := false
	for _, p := range paths {
		le, lok := l.get(p)
		re, rok := r.get(p)
		switch {
		case !rok:
			res.Diffs = append(res.Diffs, model.Diff{
				Path: p, Key: "content", RuleID: "T12",
				Left: digestHex(le.Bytes), Right: "", BytesDelta: int64(len(le.Bytes)),
			})
			res.Counts.Add("T12")
			continue
		case !lok:
			res.Diffs = append(res.Diffs, model.Diff{
				Path: p, Key: "content", RuleID: "T12",
				Left: "", Right: digestHex(re.Bytes), BytesDelta: int64(len(re.Bytes)),
			})
			res.Counts.Add("T12")
			continue
		}

		// T5：归档内序号（条目集合一致时才判）。
		if t5Applicable && le.Index != re.Index {
			res.Diffs = append(res.Diffs, model.Diff{
				Path: p, Key: "index", RuleID: "T5",
				Left: strconv.Itoa(le.Index), Right: strconv.Itoa(re.Index), BytesDelta: 0,
			})
			res.Counts.Add("T5")
		}

		// 逐 key 比对事实。
		keys := unionFactKeys(le.Facts, re.Facts)
		explained := false
		for _, k := range keys {
			lv, lok2 := le.Facts[k]
			rv, rok2 := re.Facts[k]
			if lok2 == rok2 && lv == rv {
				continue
			}
			rule, multi, ignore := classifyFact(k, lv, rv, lok2, rok2)
			if ignore {
				continue
			}
			if multi {
				overlap = true
			}
			if rule != "T12" && explainsContent(k) {
				explained = true
			}
			res.Diffs = append(res.Diffs, model.Diff{
				Path: p, Key: k, RuleID: rule, Left: lv, Right: rv, BytesDelta: 0,
			})
			res.Counts.Add(rule)
		}

		// 内容字节：只在该条目没有任何 T1–T11 解释时才落 T12。
		if !bytes.Equal(le.Bytes, re.Bytes) && !explained {
			res.Diffs = append(res.Diffs, model.Diff{
				Path: p, Key: "content", RuleID: "T12",
				Left: digestHex(le.Bytes), Right: digestHex(re.Bytes),
				BytesDelta: countDiff(le.Bytes, re.Bytes),
			})
			res.Counts.Add("T12")
		}
	}
	if overlap {
		res.Notes = append(res.Notes, "rule_overlap")
	}

	sortDiffs(res.Diffs)
	res.ResidueBytes = residueBytes(l.entries, r.entries)
	res.TreeDigest = TreeDigest{
		Left:          normalize.TreeDigest(nil, l.entries),
		Right:         normalize.TreeDigest(nil, r.entries),
		NormalizedAll: normalize.TreeDigest(allOps(), l.entries),
	}
	return res, nil
}

// ---- 一侧的预处理 ----

type side struct {
	entries []normalize.Entry
	byPath  map[string]int // key = 剥离公共根前缀后的 relpath → entries 下标
	prefix  string
}

func (s *side) get(key string) (normalize.Entry, bool) {
	i, ok := s.byPath[key]
	if !ok {
		return normalize.Entry{}, false
	}
	return s.entries[i], true
}

func prepare(in SideInput) (*side, error) {
	s := &side{byPath: map[string]int{}}
	paths := make([]string, 0, len(in.Entries))
	for _, e := range in.Entries {
		if e.Path != containerPath {
			paths = append(paths, e.Path)
		}
	}
	sort.Strings(paths)
	s.prefix = commonDirPrefix(paths)

	for _, e := range in.Entries {
		facts := make(map[string]string, len(e.Facts))
		for k, v := range e.Facts {
			facts[k] = v
		}
		ne := normalize.Entry{
			Path:       e.Path,
			Index:      e.Index,
			RootPrefix: s.prefix,
			Bytes:      append([]byte(nil), e.Data...),
			Facts:      facts,
		}
		key := strings.TrimPrefix(e.Path, s.prefix)
		if e.Path == containerPath {
			key = containerPath
		}
		if _, dup := s.byPath[key]; dup {
			return nil, fmt.Errorf("条目路径重复：%q", key)
		}
		s.byPath[key] = len(s.entries)
		s.entries = append(s.entries, ne)
	}
	return s, nil
}

// commonDirPrefix 返回所有路径的公共**目录**前缀（含结尾 `/`）。
// "公共"要求每条路径在该前缀之后还有内容；目录条目自身因此会截断前缀，
// 这正是"剥离到首个稳定锚点"的口径。
func commonDirPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	first := strings.Split(paths[0], "/")
	var prefix []string
	for i := 0; i < len(first)-1; i++ {
		ok := true
		for _, p := range paths {
			segs := strings.Split(p, "/")
			if len(segs) <= i+1 || segs[i] != first[i] {
				ok = false
				break
			}
		}
		if !ok {
			break
		}
		prefix = append(prefix, first[i])
	}
	if len(prefix) == 0 {
		return ""
	}
	return strings.Join(prefix, "/") + "/"
}

func sameEntrySet(l, r *side) bool {
	if len(l.byPath) != len(r.byPath) {
		return false
	}
	for p := range l.byPath {
		le, _ := l.get(p)
		re, ok := r.get(p)
		if !ok || digestHex(le.Bytes) != digestHex(re.Bytes) {
			return false
		}
	}
	return true
}

// ---- 分类 ----

// classifyFact 把一条事实差异映射到 T1–T12。
//   - rule   = 首个命中（T1→T12 优先级）
//   - multi  = 同时命中两个以上规则（§4.1 闭合性 → notes 记 rule_overlap）
//   - ignore = 派生差异（如 DWARF 段 size 随 comp_dir 变化），不单独记一条
func classifyFact(key, lv, rv string, lpresent, rpresent bool) (rule string, multi, ignore bool) {
	// ELF 节 key 的"存在性"承载 T8/T9/T10；offset/size 的值差异多半是派生的。
	if sec, ok := sectionOf(key); ok {
		if lpresent != rpresent {
			switch sec {
			case ".sig":
				return "T10", false, false
			case ".note.gnu.build-id", ".debug_info", ".debug_abbrev", ".debug_str":
				return "T9", false, false
			case ".comment":
				return "T8", false, false
			default:
				return "T12", false, false
			}
		}
		switch sec {
		case ".sig":
			return "T10", false, false
		case ".note.gnu.build-id":
			return "T9", false, false
		default:
			return "", false, true // 派生差异：不单独记账
		}
	}

	switch key {
	case "tar.mtime", "zip.mtime", "gzip.mtime":
		return "T1", false, false
	case "tar.uid", "tar.gid", "tar.uname", "tar.gname":
		return "T2", false, false
	case "tar.mode":
		return "T3", false, false
	case "zip.external_attr":
		lp := zipPerm(lv)
		rp := zipPerm(rv)
		if lp != rp {
			// 权限位与非权限位同时变 → T3 与 T7 都算命中。
			return "T3", zipOther(lv) != zipOther(rv), false
		}
		if zipOther(lv) != zipOther(rv) {
			return "T7", false, false
		}
		return "T12", false, false
	case "gzip.xfl", "gzip.os":
		return "T6", false, false
	case "raw_header_sha256", "zip.create_system":
		return "T7", false, false
	case "elf.comment", "elf.producer":
		return "T8", false, false
	case "elf.build_id":
		return "T9", false, false
	case "elf.comp_dir":
		return "T11", false, false
	case "tar.size":
		// 派生字段：内容长度变了，size 必然跟着变；由 normalize 同步回真实长度，
		// 不单独记一条（否则 T10 删段会被重复记成 T12）。
		return "", false, true
	}
	return "T12", false, false
}

// explainsContent 判断一条事实差异是否会**改变条目内容字节**。
// 只有 ELF 的 T8–T11 会重写内容；tar/zip 的头部事实只描述容器头，
// 不能拿来"解释"条目数据的字节差异（否则 c12 的真改会被 T1/T3 吞掉）。
func explainsContent(key string) bool {
	switch key {
	case "elf.comment", "elf.producer", "elf.build_id", "elf.comp_dir":
		return true
	}
	if sec, ok := sectionOf(key); ok {
		switch sec {
		case ".sig", ".note.gnu.build-id", ".debug_info", ".debug_abbrev", ".debug_str", ".comment":
			return true
		}
	}
	return false
}

func sectionOf(key string) (string, bool) {
	const p = "elf.section."
	if !strings.HasPrefix(key, p) {
		return "", false
	}
	rest := key[len(p):]
	i := strings.LastIndex(rest, ".")
	if i < 0 {
		return "", false
	}
	return rest[:i], true
}

func zipPerm(v string) uint64 {
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return 0
	}
	return (n >> 16) & 0o7777
}

func zipOther(v string) uint64 {
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return 0
	}
	return n &^ (uint64(0o7777) << 16)
}

// ---- 残留与摘要 ----

// allOps 返回 T1–T11（去掉 T12）：§4.5 的 residue 口径。
func allOps() []string {
	out := make([]string, 0, len(model.RuleIDs))
	for _, r := range model.RuleIDs {
		if r == "T12" {
			continue
		}
		out = append(out, r)
	}
	return out
}

// Residuals 返回归一化（S = T1–T11）后**仍不同**的条目，按 path 升序。
//
// 它是 residue_bytes 的逐条目形态（报告 `residual_items` 的数据源），两者共用同一实现：
// `residue_bytes == sum(d.BytesDelta)`。`rule_id` 恒为 T12——残留按定义就是
// 未被 T1–T11 解释的部分（data/README.md §4.1/§4.5）。
func Residuals(left, right []normalize.Entry) []model.Diff {
	ops := allOps()
	l := normalize.Apply(ops, left)
	r := normalize.Apply(ops, right)

	lm := make(map[string]normalize.Entry, len(l))
	for _, e := range l {
		lm[e.Path] = e
	}
	rm := make(map[string]normalize.Entry, len(r))
	for _, e := range r {
		rm[e.Path] = e
	}

	out := []model.Diff{}
	for _, p := range unionKeys(lm, rm) {
		le, lok := lm[p]
		re, rok := rm[p]
		var n int64
		var leftHex, rightHex string
		switch {
		case lok && rok:
			ls, rs := normalize.EntryStream(le), normalize.EntryStream(re)
			n = countDiff(ls, rs)
			leftHex, rightHex = digestHex(ls), digestHex(rs)
		case lok:
			ls := normalize.EntryStream(le)
			n = int64(len(ls))
			leftHex = digestHex(ls)
		default:
			rs := normalize.EntryStream(re)
			n = int64(len(rs))
			rightHex = digestHex(rs)
		}
		if n == 0 {
			continue
		}
		out = append(out, model.Diff{
			Path: p, Key: "content", RuleID: "T12",
			Left: leftHex, Right: rightHex, BytesDelta: n,
		})
	}
	return out
}

// residueBytes = 归一化（S = T1–T11）后两侧 entry_stream 的实际不同字节数之和。
func residueBytes(left, right []normalize.Entry) int64 {
	var total int64
	for _, d := range Residuals(left, right) {
		total += d.BytesDelta
	}
	return total
}

func countDiff(a, b []byte) int64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var d int64
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			d++
		}
	}
	if len(a) > len(b) {
		d += int64(len(a) - len(b))
	} else {
		d += int64(len(b) - len(a))
	}
	return d
}

func digestHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ---- 小工具 ----

func unionKeys[A any, B any](l map[string]A, r map[string]B) []string {
	set := make(map[string]bool, len(l)+len(r))
	for k := range l {
		set[k] = true
	}
	for k := range r {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func unionFactKeys(l, r map[string]string) []string {
	return unionKeys(l, r)
}

func sortDiffs(d []model.Diff) {
	sort.Slice(d, func(i, j int) bool {
		if d[i].Path != d[j].Path {
			return d[i].Path < d[j].Path
		}
		if d[i].Key != d[j].Key {
			return d[i].Key < d[j].Key
		}
		return d[i].RuleID < d[j].RuleID
	})
}
