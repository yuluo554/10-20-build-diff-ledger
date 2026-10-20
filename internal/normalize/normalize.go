// Package normalize 提供 T1–T11 的纯函数归一化 op 与树摘要。
//
// 口径唯一权威：data/README.md §4.2（op 语义）、§4.5（entry_stream 与 tree_digest）。
// 纪律：纯函数、幂等、不读时钟/环境/随机；归一化在内存完成，不落盘。
package normalize

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
)

// Entry 是归一化作用的载体：一个条目的路径 + 归档内序号 + 根前缀 + 内容字节 + 事实集合。
//
// RootPrefix 由调用方（attribute）按"该侧全部条目 relpath 的公共目录前缀"填入；
// T4 的 op 据此剥离前缀。它不是条目身份的一部分，不进入 entry_stream。
type Entry struct {
	Path       string
	Index      int // 归档内序号（目录输入 = -1）；T5 的判定依据
	RootPrefix string
	Bytes      []byte
	Facts      map[string]string
}

// Op 是一个归一化操作。必须是纯函数且幂等。
type Op func(Entry) Entry

// Ops 是规则编号 → 归一化 op。T12 没有 op（不可归一化，出现即 residue>0）。
// 键集合与 data/README.md §4.1 的 T1–T11 一一对应；新增规则必须同时改 §4.1。
var Ops = map[string]Op{
	"T1": opT1, "T2": opT2, "T3": opT3, "T4": opT4,
	"T5": opT5, "T6": opT6, "T7": opT7, "T8": opT8,
	"T9": opT9, "T10": opT10, "T11": opT11,
}

// Apply 按 rule id **数值**升序施加集合 S 的归一化（§4.2「可组合」）。
// 顺序固定，不对交换性作假设。注意不能按字符串排序：T10 会排到 T2 前面。
func Apply(set []string, entries []Entry) []Entry {
	ordered := append([]string(nil), set...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ruleIndex(ordered[i]) < ruleIndex(ordered[j])
	})
	out := make([]Entry, len(entries))
	for i, e := range entries {
		out[i] = cloneEntry(e)
	}
	for _, rule := range ordered {
		op, ok := Ops[rule]
		if !ok || op == nil {
			continue // T12：显式跳过，不清空差异
		}
		for i := range out {
			out[i] = op(out[i])
		}
	}
	// tar.size 是内容长度的派生字段：任何重写内容字节的 op 之后都同步回真实长度
	// （§4.10）。它不单独参与分类（T10 删段后 size 变 720 是结果，不是新原因）。
	for i := range out {
		if _, ok := fact(out[i], "tar.size"); ok {
			setFact(&out[i], "tar.size", strconv.Itoa(len(out[i].Bytes)))
		}
	}
	return out
}

func ruleIndex(rule string) int {
	for i, r := range model.RuleIDs {
		if r == rule {
			return i
		}
	}
	return len(model.RuleIDs)
}

func cloneEntry(e Entry) Entry {
	c := Entry{Path: e.Path, Index: e.Index, RootPrefix: e.RootPrefix}
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

// EntryStream 返回条目的规范化字节流（§4.5）：
//
//	decimal(index) ‖ 0x00 ‖ concat_{key 升序}( key ‖ 0x00 ‖ value ‖ 0x0A ) ‖ 0x00 ‖ content
//
// 这是 tree_digest 与 residue_bytes 共用的**唯一**条目编码。
func EntryStream(e Entry) []byte {
	var b bytes.Buffer
	b.WriteString(strconv.Itoa(e.Index))
	b.WriteByte(0x00)
	keys := make([]string, 0, len(e.Facts))
	for k := range e.Facts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte(0x00)
		b.WriteString(e.Facts[k])
		b.WriteByte(0x0A)
	}
	b.WriteByte(0x00)
	b.Write(e.Bytes)
	return b.Bytes()
}

// TreeDigest 计算集合 S 下的树摘要（data/README.md §4.5）：
//
//	sha256( concat_{relpath 升序} ( relpath ‖ 0x00 ‖ sha256(EntryStream(entry)) ‖ 0x0A ) )
//
// 排序用 relpath 的 UTF-8 字节序；同 relpath 再用 index、字节流破平（确定性）。
// 分隔符写死为 0x00 与 0x0A。这是 tree_digest 的**唯一实现**（attribute 与 minimize 都调它）。
func TreeDigest(set []string, entries []Entry) string {
	ordered := Apply(set, entries)
	sorted := append([]Entry(nil), ordered...)
	sort.Slice(sorted, func(i, j int) bool {
		if c := bytes.Compare([]byte(sorted[i].Path), []byte(sorted[j].Path)); c != 0 {
			return c < 0
		}
		if sorted[i].Index != sorted[j].Index {
			return sorted[i].Index < sorted[j].Index
		}
		return bytes.Compare(EntryStream(sorted[i]), EntryStream(sorted[j])) < 0
	})

	h := sha256.New()
	for _, e := range sorted {
		h.Write([]byte(e.Path))
		h.Write([]byte{0x00})
		sum := sha256.Sum256(EntryStream(e))
		h.Write([]byte(hex.EncodeToString(sum[:])))
		h.Write([]byte{0x0A})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ---- 事实读写小工具（op 内部用；全部就地作用于 op 自己 clone 出来的 Entry） ----

func fact(e Entry, key string) (string, bool) {
	if e.Facts == nil {
		return "", false
	}
	v, ok := e.Facts[key]
	return v, ok
}

func setFact(e *Entry, key, value string) {
	if e.Facts == nil {
		e.Facts = map[string]string{}
	}
	e.Facts[key] = value
}

func delFact(e *Entry, key string) {
	if e.Facts != nil {
		delete(e.Facts, key)
	}
}

// delSectionFacts 删除某个 ELF 节的全部 key（offset/size）。
func delSectionFacts(e *Entry, section string) {
	if e.Facts == nil {
		return
	}
	prefix := "elf.section." + section + "."
	for k := range e.Facts {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(e.Facts, k)
		}
	}
}

// ---- T1–T11 ----

// T1 时间戳：所有 mtime 置 0（§4.1 / §4.7）。
func opT1(in Entry) Entry {
	e := cloneEntry(in)
	for _, k := range []string{"tar.mtime", "zip.mtime", "gzip.mtime"} {
		if _, ok := fact(e, k); ok {
			setFact(&e, k, "0")
		}
	}
	return e
}

// T2 属主：uid=gid=0，uname=gname=""。
func opT2(in Entry) Entry {
	e := cloneEntry(in)
	for _, k := range []string{"tar.uid", "tar.gid"} {
		if _, ok := fact(e, k); ok {
			setFact(&e, k, "0")
		}
	}
	for _, k := range []string{"tar.uname", "tar.gname"} {
		if _, ok := fact(e, k); ok {
			setFact(&e, k, "")
		}
	}
	return e
}

// T3 权限：文件 0644（十进制 420）、目录 0755（十进制 493）；zip 只动权限位。
func opT3(in Entry) Entry {
	e := cloneEntry(in)
	if _, ok := fact(e, "tar.mode"); ok {
		want := "420"
		if tf, _ := fact(e, "tar.typeflag"); tf == "53" { // '5' = 目录
			want = "493"
		}
		setFact(&e, "tar.mode", want)
	}
	if v, ok := fact(e, "zip.external_attr"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			perm := (n >> 16) & 0o7777
			want := uint64(0o644)
			if perm&0o040000 != 0 {
				want = 0o755
			}
			n = (n &^ (uint64(0o7777) << 16)) | (want << 16)
			setFact(&e, "zip.external_attr", strconv.FormatUint(n, 10))
		}
	}
	return e
}

// T4 路径前缀：剥离该侧的公共根前缀（RootPrefix 由 attribute 预置）。
func opT4(in Entry) Entry {
	e := cloneEntry(in)
	if e.RootPrefix != "" && len(e.Path) >= len(e.RootPrefix) && e.Path[:len(e.RootPrefix)] == e.RootPrefix {
		e.Path = e.Path[len(e.RootPrefix):]
	}
	return e
}

// T5 条目顺序：归档内序号归零。树摘要本身按 relpath 排序，序号进 entry_stream，
// 因此"顺序无关"在归一化后由 index=0 表达。
func opT5(in Entry) Entry {
	e := cloneEntry(in)
	e.Index = 0
	return e
}

// T6 压缩器参数：固定重压口径 gzip -9n（XFL=2 最大压缩，OS=3 Unix，§4.7）。
func opT6(in Entry) Entry {
	e := cloneEntry(in)
	if _, ok := fact(e, "gzip.xfl"); ok {
		setFact(&e, "gzip.xfl", "2")
	}
	if _, ok := fact(e, "gzip.os"); ok {
		setFact(&e, "gzip.os", "3")
	}
	return e
}

// T7 归档头字段：改写为标准最小头——未抽取头部摘要归零、create_system 固定 Unix(3)。
func opT7(in Entry) Entry {
	e := cloneEntry(in)
	if _, ok := fact(e, "raw_header_sha256"); ok {
		setFact(&e, "raw_header_sha256", "")
	}
	if _, ok := fact(e, "zip.create_system"); ok {
		setFact(&e, "zip.create_system", "3")
	}
	return e
}

// T8 工具链串：.comment 与 DWARF producer 置空（内容字节同步重写）。
func opT8(in Entry) Entry {
	e := cloneEntry(in)
	_, hasComment := fact(e, "elf.comment")
	_, hasProducer := fact(e, "elf.producer")
	if !hasComment && !hasProducer {
		return e
	}
	compDir, _ := fact(e, "elf.comp_dir")
	e.Bytes = rewriteELFSections(e.Bytes, func(secs []elfSectionData) []elfSectionData {
		for i := range secs {
			switch secs[i].Name {
			case ".comment":
				secs[i].Data = nil
			case ".debug_info":
				secs[i].Data = rewriteDWARFStrings(secs[i].Data, "", compDir)
			}
		}
		return secs
	})
	if hasComment {
		setFact(&e, "elf.comment", "")
		delSectionFacts(&e, ".comment")
	}
	if hasProducer {
		setFact(&e, "elf.producer", "")
		delSectionFacts(&e, ".debug_info")
	}
	return e
}

// T9 调试符号与 build-id：移除 .note.gnu.build-id 与 DWARF 段（节表按固定布局重建）。
func opT9(in Entry) Entry {
	e := cloneEntry(in)
	_, hasBuildID := fact(e, "elf.build_id")
	hasDWARF := hasSectionFact(e, ".debug_info") || hasSectionFact(e, ".debug_abbrev")
	if !hasBuildID && !hasDWARF {
		return e
	}
	e.Bytes = rewriteELFSections(e.Bytes, func(secs []elfSectionData) []elfSectionData {
		out := secs[:0]
		for _, s := range secs {
			switch s.Name {
			case ".note.gnu.build-id", ".debug_info", ".debug_abbrev", ".debug_str", ".debug_line":
				continue
			}
			out = append(out, s)
		}
		return out
	})
	setFact(&e, "elf.build_id", "")
	delSectionFacts(&e, ".note.gnu.build-id")
	delSectionFacts(&e, ".debug_info")
	delSectionFacts(&e, ".debug_abbrev")
	delSectionFacts(&e, ".debug_str")
	return e
}

// T10 签名 blob：移除 .sig 段。
func opT10(in Entry) Entry {
	e := cloneEntry(in)
	if !hasSectionFact(e, ".sig") {
		return e
	}
	e.Bytes = rewriteELFSections(e.Bytes, func(secs []elfSectionData) []elfSectionData {
		out := secs[:0]
		for _, s := range secs {
			if s.Name == ".sig" {
				continue
			}
			out = append(out, s)
		}
		return out
	})
	delSectionFacts(&e, ".sig")
	return e
}

// T11 构建路径与环境：comp_dir 归一为固定占位 <build-root>（§4.7）。
func opT11(in Entry) Entry {
	e := cloneEntry(in)
	compDir, hasCompDir := fact(e, "elf.comp_dir")
	if !hasCompDir {
		return e
	}
	_ = compDir
	producer, _ := fact(e, "elf.producer")
	e.Bytes = rewriteELFSections(e.Bytes, func(secs []elfSectionData) []elfSectionData {
		for i := range secs {
			if secs[i].Name == ".debug_info" {
				secs[i].Data = rewriteDWARFStrings(secs[i].Data, producer, "<build-root>")
			}
		}
		return secs
	})
	setFact(&e, "elf.comp_dir", "<build-root>")
	delSectionFacts(&e, ".debug_info")
	return e
}

func hasSectionFact(e Entry, section string) bool {
	if e.Facts == nil {
		return false
	}
	prefix := "elf.section." + section + "."
	for k := range e.Facts {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
