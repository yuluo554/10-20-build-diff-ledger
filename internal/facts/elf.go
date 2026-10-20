// ELF 事实抽取（T8–T11 的输入，plan/03 §三）。
//
// 只抽契约里的 key：.comment、.note.gnu.build-id、DWARF producer/comp_dir、
// 固定跟踪节的 offset/size。节表/程序头表的原始字节进入 raw_header（§4.3 第 4 条）。
package facts

import (
	"bytes"
	"debug/dwarf"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

// analyzeELF 用 debug/elf + debug/dwarf 抽取事实。
func analyzeELF(data []byte) (contentFacts, error) {
	ef, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return contentFacts{}, fmt.Errorf("ELF 解析失败（%v）", err)
	}
	defer ef.Close()

	var cf contentFacts
	cf.kind = "elf"

	header, err := elfHeaderBytes(ef, data)
	if err != nil {
		return contentFacts{}, err
	}
	cf.header = header

	if sec := ef.Section(".comment"); sec != nil {
		b, err := sec.Data()
		if err != nil {
			return contentFacts{}, fmt.Errorf(".comment 读取失败（%v）", err)
		}
		cf.values = append(cf.values, [2]string{"elf.comment", encodeString(strings.TrimRight(string(b), "\x00"))})
	}

	if sec := ef.Section(".note.gnu.build-id"); sec != nil {
		b, err := sec.Data()
		if err != nil {
			return contentFacts{}, fmt.Errorf(".note.gnu.build-id 读取失败（%v）", err)
		}
		id, err := buildIDFromNotes(b, ef.ByteOrder)
		if err != nil {
			return contentFacts{}, err
		}
		if len(id) > 0 {
			cf.values = append(cf.values, [2]string{"elf.build_id", hex.EncodeToString(id)})
		}
	}

	dw, err := ef.DWARF()
	if err != nil {
		return contentFacts{}, fmt.Errorf("DWARF 解析失败（%v）", err)
	}
	if dw != nil {
		prod, dir, found, err := dwarfProducerCompDir(dw)
		if err != nil {
			return contentFacts{}, err
		}
		if found {
			cf.values = append(cf.values,
				[2]string{"elf.comp_dir", encodeString(dir)},
				[2]string{"elf.producer", encodeString(prod)},
			)
		}
	}

	for _, name := range TrackedSections {
		sec := ef.Section(name)
		if sec == nil {
			continue
		}
		cf.values = append(cf.values,
			[2]string{SectionKey(name, "offset"), fmt.Sprintf("%d", sec.Offset)},
			[2]string{SectionKey(name, "size"), fmt.Sprintf("%d", sec.Size)},
		)
	}
	return cf, nil
}

// elfHeaderBytes = ELF 头 ‖ 程序头表 ‖ 节表的原始字节（§4.3 第 4 条的"头部扩展区"）。
//
// debug/elf 不导出 e_phoff/e_shoff 等字段，这里按 ELF 规范手工读（类/字节序取自
// elf.FileHeader 的导出字段，避免重复判定）。
func elfHeaderBytes(ef *elf.File, data []byte) ([]byte, error) {
	order := ef.ByteOrder
	ehdr, phoffPos, shoffPos, phentsizePos, phnumPos, shentsizePos, shnumPos := 52, 28, 32, 42, 44, 46, 48
	readWord := func(off int) uint64 { return uint64(order.Uint32(data[off : off+4])) }
	if ef.Class == elf.ELFCLASS64 {
		ehdr, phoffPos, shoffPos, phentsizePos, phnumPos, shentsizePos, shnumPos = 64, 32, 40, 54, 56, 58, 60
		readWord = func(off int) uint64 { return order.Uint64(data[off : off+8]) }
	}
	if len(data) < ehdr {
		return nil, fmt.Errorf("ELF 头被截断（%d < %d）", len(data), ehdr)
	}
	phoff := readWord(phoffPos)
	shoff := readWord(shoffPos)
	phentsize := uint64(order.Uint16(data[phentsizePos : phentsizePos+2]))
	phnum := uint64(order.Uint16(data[phnumPos : phnumPos+2]))
	shentsize := uint64(order.Uint16(data[shentsizePos : shentsizePos+2]))
	shnum := uint64(order.Uint16(data[shnumPos : shnumPos+2]))

	out := append([]byte(nil), data[:ehdr]...)
	if phnum > 0 {
		end := phoff + phnum*phentsize
		if end > uint64(len(data)) || end < phoff {
			return nil, fmt.Errorf("ELF 程序头表越界")
		}
		out = append(out, data[phoff:end]...)
	}
	if shnum > 0 {
		end := shoff + shnum*shentsize
		if end > uint64(len(data)) || end < shoff {
			return nil, fmt.Errorf("ELF 节表越界")
		}
		out = append(out, data[shoff:end]...)
	}
	return out, nil
}

// buildIDFromNotes 解析 SHT_NOTE 内容，取 type=3（NT_GNU_BUILD_ID）的 desc。
func buildIDFromNotes(b []byte, order binary.ByteOrder) ([]byte, error) {
	pos := 0
	for pos+12 <= len(b) {
		namesz := int(order.Uint32(b[pos : pos+4]))
		descsz := int(order.Uint32(b[pos+4 : pos+8]))
		typ := order.Uint32(b[pos+8 : pos+12])
		pos += 12
		if namesz < 0 || descsz < 0 || pos+namesz > len(b) {
			return nil, fmt.Errorf("build-id note 头非法")
		}
		name := b[pos : pos+namesz]
		pos += align4(namesz)
		if pos+descsz > len(b) {
			return nil, fmt.Errorf("build-id note 内容被截断")
		}
		desc := b[pos : pos+descsz]
		pos += align4(descsz)
		if typ == 3 && bytes.HasPrefix(name, []byte("GNU")) {
			return desc, nil
		}
	}
	return nil, nil
}

func align4(n int) int {
	if r := n % 4; r != 0 {
		return n + (4 - r)
	}
	return n
}

// dwarfProducerCompDir 取第一个 compile_unit 的 DW_AT_producer / DW_AT_comp_dir。
func dwarfProducerCompDir(dw *dwarf.Data) (prod, dir string, found bool, err error) {
	r := dw.Reader()
	for {
		e, rerr := r.Next()
		if rerr != nil {
			return "", "", false, fmt.Errorf("DWARF 读取失败（%v）", rerr)
		}
		if e == nil {
			return "", "", false, nil
		}
		if e.Tag != dwarf.TagCompileUnit {
			continue
		}
		p, pok := e.Val(dwarf.AttrProducer).(string)
		d, dok := e.Val(dwarf.AttrCompDir).(string)
		if !pok && !dok {
			return "", "", false, nil
		}
		return p, d, true, nil
	}
}
