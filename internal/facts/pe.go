// PE 事实抽取（抽样：plan/03 §三只要求 timestamp/checksum/Rich 头）。
//
// 不引第三方库：PE 头字段是固定偏移的小端结构，手工读即可，也便于把
// "哪些字节算头部"写清楚（raw_header）。
package facts

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// COFF 头部字段偏移（相对 "PE\0\0" 签名之后）。
const (
	coffTimestamp     = 4  // TimeDateStamp
	coffSizeOfOptHdr  = 16 // SizeOfOptionalHeader
	coffHeaderSize    = 20
	optCheckSumOffset = 64 // CheckSum（PE32 与 PE32+ 同偏移）
)

// analyzePE 抽取 pe.timestamp / pe.checksum / pe.rich.present / pe.rich.digest。
func analyzePE(data []byte) (contentFacts, error) {
	if len(data) < 0x40 {
		return contentFacts{}, fmt.Errorf("PE 的 DOS 头被截断")
	}
	le := binary.LittleEndian
	e := int(le.Uint32(data[0x3C:0x40]))
	if e < 0x40 || e+4+coffHeaderSize > len(data) {
		return contentFacts{}, fmt.Errorf("PE 头的 e_lfanew 越界（%d）", e)
	}
	if string(data[e:e+4]) != "PE\x00\x00" {
		return contentFacts{}, fmt.Errorf("MZ 文件缺少 PE 签名（不是 PE 可执行文件）")
	}
	coff := e + 4
	timestamp := le.Uint32(data[coff+coffTimestamp : coff+coffTimestamp+4])
	sizeOpt := int(le.Uint16(data[coff+coffSizeOfOptHdr : coff+coffSizeOfOptHdr+2]))
	opt := coff + coffHeaderSize
	if opt+sizeOpt > len(data) || sizeOpt < optCheckSumOffset+4 {
		return contentFacts{}, fmt.Errorf("PE 可选头越界（size=%d）", sizeOpt)
	}
	magic := le.Uint16(data[opt : opt+2])
	if magic != 0x10b && magic != 0x20b {
		return contentFacts{}, fmt.Errorf("PE 可选头魔数 0x%04x 不受支持", magic)
	}
	checksum := le.Uint32(data[opt+optCheckSumOffset : opt+optCheckSumOffset+4])

	var cf contentFacts
	cf.kind = "pe"
	cf.values = append(cf.values,
		[2]string{"pe.timestamp", fmt.Sprintf("%d", timestamp)},
		[2]string{"pe.checksum", fmt.Sprintf("%d", checksum)},
	)

	digest, present := richDigest(data[0x40:e])
	if present {
		cf.values = append(cf.values,
			[2]string{"pe.rich.present", "true"},
			[2]string{"pe.rich.digest", digest},
		)
	} else {
		cf.values = append(cf.values, [2]string{"pe.rich.present", "false"})
	}

	// raw_header = DOS 头 + DOS stub（含 Rich）+ PE 签名 + COFF + 可选头。
	cf.header = append([]byte(nil), data[:opt+sizeOpt]...)
	return cf, nil
}

// richDigest 在 DOS stub 里找 DanS … Rich 区，返回 sha256(原始字节) 与"是否存在"。
//
// Rich 头按微软链接器的写法是：DanS + 3 个 XOR key dwords + 若干键值对 + "Rich" + key。
// 这里不解释键值对（v0 只承诺存在性与摘要），摘要取**原始字节**，便于手工复算
// `dd` 切片后 `sha256sum`。
func richDigest(stub []byte) (string, bool) {
	rich := bytes.LastIndex(stub, []byte("Rich"))
	if rich < 0 {
		return "", false
	}
	dans := bytes.LastIndex(stub[:rich], []byte("DanS"))
	if dans < 0 {
		return "", false
	}
	end := rich + 8 // "Rich" + 4 字节 XOR key
	if end > len(stub) {
		end = len(stub)
	}
	sum := sha256.Sum256(stub[dans:end])
	return hex.EncodeToString(sum[:]), true
}
