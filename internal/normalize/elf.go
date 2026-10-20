// ELF 节表重写（T8–T11 的字节侧归一化）。
//
// 为什么需要重写：删段或改 DWARF 字符串会移动后续字节；只改事实值而留下不同的
// 内容字节，会让 residue_bytes > 0。样张 ELF 由 tools/gen_fixtures 按固定布局手写，
// 这里用**同一布局算法**重建节表（plan/06 R8 的处置）。
//
// 边界：只处理 ELF64 小端（样张形态）。其它形态原样返回——事实仍会被归一化，
// 但内容字节可能残留；该边界写进 HANDOFF-M4（不承诺真实产物，plan/06 R4）。
package normalize

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
)

type elfSectionData struct {
	Name      string
	Type      uint32
	Flags     uint64
	Addr      uint64
	Link      uint32
	Info      uint32
	Addralign uint64
	Entsize   uint64
	Data      []byte
}

// rewriteELFSections 解析 ELF 的节，交给 fn 增删改，再按固定布局重建。
// 非 ELF64 小端或解析失败 → 原样返回。
func rewriteELFSections(data []byte, fn func([]elfSectionData) []elfSectionData) []byte {
	if len(data) < 64 || string(data[:4]) != "\x7fELF" {
		return data
	}
	if data[4] != byte(elf.ELFCLASS64) || data[5] != byte(elf.ELFDATA2LSB) {
		return data
	}
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return data
	}
	defer f.Close()

	secs := make([]elfSectionData, 0, len(f.Sections))
	for i, s := range f.Sections {
		if i == 0 || s.Type == elf.SHT_NULL || s.Name == ".shstrtab" {
			continue
		}
		d, err := s.Data()
		if err != nil {
			d = nil
		}
		secs = append(secs, elfSectionData{
			Name:      s.Name,
			Type:      uint32(s.Type),
			Flags:     uint64(s.Flags),
			Addr:      s.Addr,
			Link:      s.Link,
			Info:      s.Info,
			Addralign: s.Addralign,
			Entsize:   s.Entsize,
			Data:      d,
		})
	}
	secs = fn(secs)
	return buildELF64LE(data[:64], secs)
}

// buildELF64LE 按 tools/gen_fixtures/write_elf.go 的布局重建：
// 头 → 各节（按 Addralign）→ 8 字节对齐 → .shstrtab → 8 字节对齐 → 节表。
func buildELF64LE(hdr []byte, secs []elfSectionData) []byte {
	le := binary.LittleEndian
	var body bytes.Buffer
	body.Write(make([]byte, 64)) // 头部占位，最后覆盖

	offsets := make([]uint64, len(secs))
	sizes := make([]uint64, len(secs))
	for i, s := range secs {
		align := s.Addralign
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

	shstrOff := uint64(body.Len())
	shstrSize := uint64(names.Len())
	body.Write(names.Bytes())
	if pad := padTo(body.Len(), 8); pad > 0 {
		body.Write(make([]byte, pad))
	}
	shoff := uint64(body.Len())

	// NULL 节
	body.Write(make([]byte, 64))
	for i, s := range secs {
		sh := make([]byte, 64)
		le.PutUint32(sh[0:], nameOff[i])
		le.PutUint32(sh[4:], s.Type)
		le.PutUint64(sh[8:], s.Flags)
		le.PutUint64(sh[16:], s.Addr)
		le.PutUint64(sh[24:], offsets[i])
		le.PutUint64(sh[32:], sizes[i])
		le.PutUint32(sh[40:], s.Link)
		le.PutUint32(sh[44:], s.Info)
		le.PutUint64(sh[48:], s.Addralign)
		le.PutUint64(sh[56:], s.Entsize)
		body.Write(sh)
	}
	{
		sh := make([]byte, 64)
		le.PutUint32(sh[0:], shstrNameOff)
		le.PutUint32(sh[4:], uint32(elf.SHT_STRTAB))
		le.PutUint64(sh[24:], shstrOff)
		le.PutUint64(sh[32:], shstrSize)
		le.PutUint64(sh[48:], 1)
		body.Write(sh)
	}

	out := body.Bytes()
	copy(out[0:64], hdr)
	le.PutUint64(out[40:], shoff)                 // e_shoff
	le.PutUint16(out[60:], uint16(1+len(secs)+1)) // e_shnum（NULL + 各节 + .shstrtab）
	le.PutUint16(out[62:], uint16(len(secs)+1))   // e_shstrndx
	return out
}

// rewriteDWARFStrings 重写样张 DWARF v4 单 CU 里的两个字符串：
// 头(11) ‖ 缩写码 0x01 ‖ producer ‖ 0x00 ‖ comp_dir ‖ 0x00。
// 形态不认识 → 原样返回（不猜）。
func rewriteDWARFStrings(data []byte, producer, compDir string) []byte {
	const hdr = 11
	if len(data) < hdr+1 {
		return data
	}
	if binary.LittleEndian.Uint32(data[0:4]) == 0xffffffff { // DWARF64 初始长度
		return data
	}
	if uint32(len(data))-4 != binary.LittleEndian.Uint32(data[0:4]) {
		return data
	}
	body := data[hdr:]
	if len(body) < 2 || body[0] != 0x01 {
		return data
	}
	if _, _, ok := readCString(body[1:]); !ok {
		return data
	}
	var out bytes.Buffer
	out.Write(data[:hdr])
	out.WriteByte(0x01)
	out.WriteString(producer)
	out.WriteByte(0x00)
	out.WriteString(compDir)
	out.WriteByte(0x00)
	b := out.Bytes()
	binary.LittleEndian.PutUint32(b[0:4], uint32(len(b)-4))
	return b
}

func readCString(b []byte) (string, []byte, bool) {
	i := bytes.IndexByte(b, 0x00)
	if i < 0 {
		return "", nil, false
	}
	return string(b[:i]), b[i+1:], true
}

func padTo(n, align int) int {
	if align <= 1 {
		return 0
	}
	if r := n % align; r != 0 {
		return align - r
	}
	return 0
}
