// 手写最小 ELF64 生成器（T8 / T9 / T10 / T11 的载体）。
//
// 为什么手写：生成器**不跑构建**（题面红线），而样张需要"两次构建的等效物"。
// 这里直接产出结构合法的 ELF64（可被 debug/elf + debug/dwarf 解析），
// 只包含被测字段：
//
//	.comment            → elf.comment（T8）
//	.note.gnu.build-id  → elf.build_id（T9）
//	.debug_info/.debug_abbrev/.debug_str → elf.producer（T8）、elf.comp_dir（T11）
//	.sig                → 签名段（T10；只有"存在/不存在"两种状态，内容为定长填充）
//
// 所有偏移由本文件的布局计算得出，不依赖任何外部工具或库的默认值。
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// elfSpec 是可注入的 ELF 字段。
type elfSpec struct {
	Comment   string // .comment 全文（T8）
	Producer  string // DWARF DW_AT_producer（T8）
	CompDir   string // DWARF DW_AT_comp_dir（T11）
	BuildID   []byte // .note.gnu.build-id 的 desc（20 字节 = sha1 形态；T9）
	Signature []byte // nil = 无签名段（T10）；非 nil = 写出 .sig
	Text      []byte // .text 内容（无差异，全部样张相同）
}

// ELF 常量（只用到这些）。
const (
	ehdrSize    = 64
	shdrSize    = 64
	etExec      = 2
	emX8664     = 62
	shStrTab    = 3
	shProgBits  = 1
	shNote      = 7
	shfAllocExe = 0x6 // SHF_ALLOC | SHF_EXECINSTR
	shfAlloc    = 0x2
)

// section 是待写出的一个节。
type section struct {
	Name  string
	Type  uint32
	Flags uint64
	Data  []byte
	Align uint64
}

// buildELF 产出一个合法的 ELF64 小端可执行文件（无程序头；节表是唯一入口）。
func buildELF(s elfSpec) ([]byte, error) {
	text := s.Text
	if len(text) == 0 {
		text = []byte{0x55, 0x48, 0x89, 0xe5, 0x31, 0xc0, 0x5d, 0xc3} // push rbp; mov rbp,rsp; xor eax,eax; pop rbp; ret
	}
	if len(s.BuildID) != 20 {
		return nil, fmt.Errorf("build-id 必须是 20 字节（收到 %d）", len(s.BuildID))
	}

	abbrev, info := buildDWARF(s.Producer, s.CompDir)

	secs := []section{
		{Name: ".text", Type: shProgBits, Flags: shfAllocExe, Data: text, Align: 16},
		{Name: ".comment", Type: shProgBits, Flags: 0x30 /* SHF_MERGE|SHF_STRINGS */, Data: append([]byte(s.Comment), 0)},
		{Name: ".note.gnu.build-id", Type: shNote, Flags: shfAlloc, Data: buildIDNote(s.BuildID), Align: 4},
		{Name: ".debug_abbrev", Type: shProgBits, Data: abbrev},
		{Name: ".debug_info", Type: shProgBits, Data: info},
	}
	if s.Signature != nil {
		// 名字带 .note 形态但类型是 PROGBITS：只断言"段是否存在"，
		// 不假装实现真实的签名格式（v0 的边界，见 plan/04 §二 T10 备注）。
		secs = append(secs, section{Name: ".sig", Type: shProgBits, Data: s.Signature, Align: 16})
	}

	// .shstrtab：所有节名（含自身）
	var names bytes.Buffer
	names.WriteByte(0)
	nameOff := make([]uint32, len(secs))
	for i, s := range secs {
		nameOff[i] = uint32(names.Len())
		names.WriteString(s.Name)
		names.WriteByte(0)
	}
	shstrNameOff := uint32(names.Len())
	names.WriteString(".shstrtab")
	names.WriteByte(0)

	// 布局：ELF 头 → 各节数据（按 Align 对齐）→ .shstrtab → 节表
	var body bytes.Buffer
	body.Write(make([]byte, ehdrSize))
	offsets := make([]uint64, len(secs))
	sizes := make([]uint64, len(secs))
	for i, s := range secs {
		align := s.Align
		if align == 0 {
			align = 1
		}
		if pad := padTo(body.Len(), int(align)); pad > 0 {
			body.Write(make([]byte, pad))
		}
		offsets[i] = uint64(body.Len())
		sizes[i] = uint64(len(s.Data))
		body.Write(s.Data)
	}
	if pad := padTo(body.Len(), 8); pad > 0 {
		body.Write(make([]byte, pad))
	}
	shstrOff := uint64(body.Len())
	shstrSize := uint64(names.Len())
	body.Write(names.Bytes())
	if pad := padTo(body.Len(), 8); pad > 0 {
		body.Write(make([]byte, pad))
	}
	shoff := uint64(body.Len())

	shnum := uint16(1 + len(secs) + 1) // NULL + 各节 + .shstrtab
	shstrndx := uint16(len(secs) + 1)

	// 节表：0 号是 SHT_NULL
	body.Write(make([]byte, shdrSize))
	for i, s := range secs {
		sh := make([]byte, shdrSize)
		le := binary.LittleEndian
		le.PutUint32(sh[0:], nameOff[i])
		le.PutUint32(sh[4:], s.Type)
		le.PutUint64(sh[8:], s.Flags)
		le.PutUint64(sh[16:], 0) // sh_addr
		le.PutUint64(sh[24:], offsets[i])
		le.PutUint64(sh[32:], sizes[i])
		le.PutUint32(sh[40:], 0) // sh_link
		le.PutUint32(sh[44:], 0) // sh_info
		align := s.Align
		if align == 0 {
			align = 1
		}
		le.PutUint64(sh[48:], align)
		le.PutUint64(sh[56:], 0) // sh_entsize
		body.Write(sh)
	}
	{
		sh := make([]byte, shdrSize)
		le := binary.LittleEndian
		le.PutUint32(sh[0:], shstrNameOff)
		le.PutUint32(sh[4:], shStrTab)
		le.PutUint64(sh[24:], shstrOff)
		le.PutUint64(sh[32:], shstrSize)
		le.PutUint64(sh[48:], 1)
		body.Write(sh)
	}

	out := body.Bytes()

	// ELF 头（覆盖开头的 64 字节）
	copy(out[0:4], "\x7fELF")
	out[4] = 2 // EI_CLASS = ELFCLASS64
	out[5] = 1 // EI_DATA = ELFDATA2LSB
	out[6] = 1 // EI_VERSION = EV_CURRENT
	out[7] = 0 // EI_OSABI = SYSV
	le := binary.LittleEndian
	le.PutUint16(out[16:], etExec)
	le.PutUint16(out[18:], emX8664)
	le.PutUint32(out[20:], 1)           // e_version
	le.PutUint64(out[24:], 0x400000+64) // e_entry（无实际执行语义）
	le.PutUint64(out[32:], 0)           // e_phoff
	le.PutUint64(out[40:], shoff)
	le.PutUint32(out[48:], 0) // e_flags
	le.PutUint16(out[52:], ehdrSize)
	le.PutUint16(out[54:], 56) // e_phentsize（无程序头，值仍写标准常量）
	le.PutUint16(out[56:], 0)  // e_phnum
	le.PutUint16(out[58:], shdrSize)
	le.PutUint16(out[60:], shnum)
	le.PutUint16(out[62:], shstrndx)
	return out, nil
}

// buildIDNote 产出 .note.gnu.build-id 的节内容：
// namesz=4, descsz=20, type=3(NT_GNU_BUILD_ID), name="GNU\0", desc。
func buildIDNote(desc []byte) []byte {
	var b bytes.Buffer
	var h [12]byte
	binary.LittleEndian.PutUint32(h[0:], 4)
	binary.LittleEndian.PutUint32(h[4:], uint32(len(desc)))
	binary.LittleEndian.PutUint32(h[8:], 3)
	b.Write(h[:])
	b.WriteString("GNU\x00")
	b.Write(desc)
	for padTo(b.Len(), 4) > 0 {
		b.WriteByte(0)
	}
	return b.Bytes()
}

// buildDWARF 产出 (.debug_abbrev, .debug_info) 两节，含一个 compile_unit DIE，
// 属性是 DW_AT_producer 与 DW_AT_comp_dir（DW_FORM_strp → .debug_str 已内联为
// DW_FORM_string，避免多一节；M2 用 debug/dwarf 读 DW_AT_* 即可）。
//
// 结构：DWARF v4 的 unit 头（length/version/abbrev_off/address_size）+ 一个 DIE。
func buildDWARF(producer, compDir string) (abbrev, info []byte) {
	// .debug_abbrev：
	//   1 = DW_TAG_compile_unit, has_children = no
	//       DW_AT_producer (0x25) DW_FORM_string (0x08)
	//       DW_AT_comp_dir (0x1b) DW_FORM_string (0x08)
	//       0, 0 结束属性表
	//   0 结束缩写表
	abbrev = []byte{
		0x01,       // 缩写码 1（ULEB128）
		0x11,       // DW_TAG_compile_unit
		0x00,       // DW_CHILDREN_no
		0x25, 0x08, // DW_AT_producer, DW_FORM_string
		0x1b, 0x08, // DW_AT_comp_dir, DW_FORM_string
		0x00, 0x00, // 属性表结束
		0x00, // 缩写表结束
	}

	// DIE 体：缩写码 1 + 两个 NUL 结尾字符串
	var die bytes.Buffer
	die.WriteByte(0x01)
	die.WriteString(producer)
	die.WriteByte(0)
	die.WriteString(compDir)
	die.WriteByte(0)

	// unit 头是 DWARF v4：length(4) version(2) abbrev_off(4) addr_size(1)
	const hdr = 4 + 2 + 4 + 1
	head := make([]byte, hdr)
	binary.LittleEndian.PutUint16(head[4:6], 4)  // version
	binary.LittleEndian.PutUint32(head[6:10], 0) // debug_abbrev_offset
	head[10] = 8                                 // address_size
	var body bytes.Buffer
	body.Write(head)
	body.Write(die.Bytes())

	out := body.Bytes()
	binary.LittleEndian.PutUint32(out[0:4], uint32(len(out)-4))
	return abbrev, out
}
