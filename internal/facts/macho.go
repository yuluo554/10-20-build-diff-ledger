// Mach-O 事实抽取（抽样：uuid / minos / sdk / tools）。
//
// 不引第三方库：Mach-O 头 + load commands 是固定布局，手工读即可。
// 支持 thin（32/64、两种字节序）与 fat（取第一个架构）；其余形态显式报错。
package facts

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

// load command 常量（只列用到的）。
const (
	lcLoadDylib         = 0xc
	lcUUID              = 0x1b
	lcVersionMinMacOSX  = 0x24
	lcVersionMinIPhone  = 0x25
	lcVersionMinTVOS    = 0x2f
	lcVersionMinWatchOS = 0x30
	lcBuildVersion      = 0x32
)

func analyzeMachO(data []byte) (contentFacts, error) {
	be := binary.BigEndian
	if len(data) < 4 {
		return contentFacts{}, fmt.Errorf("Mach-O 头被截断")
	}
	switch be.Uint32(data[:4]) {
	case 0xcafebabe, 0xcafebabf:
		// fat：big-endian 头，20 字节/架构。
		if len(data) < 8 {
			return contentFacts{}, fmt.Errorf("fat Mach-O 头被截断")
		}
		n := int(be.Uint32(data[4:8]))
		if n <= 0 || 8+20*n > len(data) {
			return contentFacts{}, fmt.Errorf("fat Mach-O 架构表越界（n=%d）", n)
		}
		off := int(be.Uint32(data[16:20]))
		size := int(be.Uint32(data[20:24]))
		if off < 0 || size <= 0 || off+size > len(data) {
			return contentFacts{}, fmt.Errorf("fat Mach-O 第一架构越界")
		}
		return analyzeThinMachO(data[off:off+size], be, be.Uint32(data[off:off+4]) == 0xfeedfacf)
	case 0xbebafeca:
		return contentFacts{}, fmt.Errorf("little-endian fat Mach-O 暂不支持（未知即显式）")
	case 0xfeedface, 0xfeedfacf:
		return analyzeThinMachO(data, be, be.Uint32(data[:4]) == 0xfeedfacf)
	case 0xcefaedfe, 0xcffaedfe:
		le := binary.LittleEndian
		return analyzeThinMachO(data, le, be.Uint32(data[:4]) == 0xcffaedfe)
	}
	return contentFacts{}, fmt.Errorf("不是 Mach-O 魔数")
}

func analyzeThinMachO(data []byte, order binary.ByteOrder, is64 bool) (contentFacts, error) {
	hdr := 28
	if is64 {
		hdr = 32
	}
	if len(data) < hdr {
		return contentFacts{}, fmt.Errorf("Mach-O 头被截断（%d < %d）", len(data), hdr)
	}
	ncmds := int(order.Uint32(data[16:20]))
	sizeofcmds := int(order.Uint32(data[20:24]))
	end := hdr + sizeofcmds
	if ncmds < 0 || sizeofcmds < 0 || end > len(data) || end < hdr {
		return contentFacts{}, fmt.Errorf("Mach-O load commands 越界（ncmds=%d size=%d）", ncmds, sizeofcmds)
	}

	var cf contentFacts
	cf.kind = "macho"
	cf.header = append([]byte(nil), data[:end]...)

	pos := hdr
	var buildMin, buildSDK uint32
	var haveBuild bool
	var minVer, minSDK uint32
	var haveMin bool
	var tools []string

	for i := 0; i < ncmds; i++ {
		if pos+8 > end {
			return contentFacts{}, fmt.Errorf("第 %d 条 load command 被截断", i)
		}
		cmd := order.Uint32(data[pos : pos+4])
		size := int(order.Uint32(data[pos+4 : pos+8]))
		if size < 8 || pos+size > end {
			return contentFacts{}, fmt.Errorf("第 %d 条 load command 长度非法（%d）", i, size)
		}
		switch cmd {
		case lcUUID:
			if size < 24 {
				return contentFacts{}, fmt.Errorf("LC_UUID 长度非法（%d）", size)
			}
			cf.values = append(cf.values, [2]string{"macho.uuid", hex.EncodeToString(data[pos+8 : pos+24])})
		case lcBuildVersion:
			if size < 24 {
				return contentFacts{}, fmt.Errorf("LC_BUILD_VERSION 长度非法（%d）", size)
			}
			buildMin = order.Uint32(data[pos+12 : pos+16])
			buildSDK = order.Uint32(data[pos+16 : pos+20])
			ntools := int(order.Uint32(data[pos+20 : pos+24]))
			if 24+ntools*8 > size {
				return contentFacts{}, fmt.Errorf("LC_BUILD_VERSION 工具表越界（ntools=%d）", ntools)
			}
			haveBuild = true
			for t := 0; t < ntools; t++ {
				base := pos + 24 + t*8
				id := order.Uint32(data[base : base+4])
				ver := order.Uint32(data[base+4 : base+8])
				tools = append(tools, toolName(id)+":"+formatMachVersion(ver))
			}
		case lcVersionMinMacOSX, lcVersionMinIPhone, lcVersionMinTVOS, lcVersionMinWatchOS:
			if size < 16 {
				return contentFacts{}, fmt.Errorf("LC_VERSION_MIN 长度非法（%d）", size)
			}
			minVer = order.Uint32(data[pos+8 : pos+12])
			minSDK = order.Uint32(data[pos+12 : pos+16])
			haveMin = true
		case lcLoadDylib:
			if size < 12 {
				return contentFacts{}, fmt.Errorf("LC_LOAD_DYLIB 长度非法（%d）", size)
			}
			// 只做边界校验：v0 不把依赖库当作 macho.tools（tools 只认 BUILD_VERSION）。
			off := int(order.Uint32(data[pos+8 : pos+12]))
			if off < 0 || off >= size {
				return contentFacts{}, fmt.Errorf("LC_LOAD_DYLIB 名称偏移越界（%d）", off)
			}
		}
		pos += size
	}

	switch {
	case haveBuild:
		cf.values = append(cf.values,
			[2]string{"macho.minos", formatMachVersion(buildMin)},
			[2]string{"macho.sdk", formatMachVersion(buildSDK)},
		)
	case haveMin:
		cf.values = append(cf.values,
			[2]string{"macho.minos", formatMachVersion(minVer)},
			[2]string{"macho.sdk", formatMachVersion(minSDK)},
		)
	}
	if len(tools) > 0 {
		cf.values = append(cf.values, [2]string{"macho.tools", strings.Join(tools, ",")})
	}
	return cf, nil
}

// formatMachVersion 把 nibble 版本号（xxxx.yy.zz）转成 "x.y.z"。
func formatMachVersion(v uint32) string {
	return fmt.Sprintf("%d.%d.%d", v>>16, (v>>8)&0xff, v&0xff)
}

// toolName 是 LC_BUILD_VERSION 的 tool id 名称（未知 id 用十进制）。
func toolName(id uint32) string {
	switch id {
	case 1:
		return "clang"
	case 2:
		return "swift"
	case 3:
		return "ld"
	default:
		return fmt.Sprintf("tool%d", id)
	}
}
