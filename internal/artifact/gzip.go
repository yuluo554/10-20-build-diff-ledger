// gzip 成员头解析（容器级事实 + raw_header）。
//
// 为什么手写：compress/gzip 的 Reader 不暴露 XFL/OS，也不暴露 FEXTRA/FNAME/
// FCOMMENT/FHCRC 原文，而 §4.3 要求这些未抽取字段进入 raw_header_sha256。
// 这里只读**成员头**（10 字节 + 可选字段），不碰 deflate 流；解压仍由
// compress/gzip 完成，避免自写 inflate。
//
// 容器级 facts 的键：gzip.mtime / gzip.xfl / gzip.os（data/README.md §4.3）。
package artifact

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
)

// gzipHeader 是成员头解析结果。
type gzipHeader struct {
	MTime uint32
	XFL   byte
	OS    byte
	// Raw 是除 mtime/xfl/os 之外的原始头部字节：
	// FEXTRA（含 2 字节 XLEN）‖ FNAME（含 NUL）‖ FCOMMENT（含 NUL）‖ FHCRC（2 字节）。
	Raw []byte
}

// readGzipHeader 从流当前位置读一个 gzip 成员头。
func readGzipHeader(r io.Reader) (gzipHeader, error) {
	var h gzipHeader
	head := make([]byte, 10)
	if _, err := io.ReadFull(r, head); err != nil {
		return h, fmt.Errorf("gzip 头被截断（%v）", err)
	}
	if head[0] != 0x1f || head[1] != 0x8b {
		return h, fmt.Errorf("不是 gzip 魔数（%02x%02x）", head[0], head[1])
	}
	if head[2] != 8 {
		return h, fmt.Errorf("gzip 压缩方法 %d 不受支持（只支持 deflate=8）", head[2])
	}
	flg := head[3]
	h.MTime = binary.LittleEndian.Uint32(head[4:8])
	h.XFL = head[8]
	h.OS = head[9]

	var raw bytes.Buffer
	if flg&0x04 != 0 { // FEXTRA
		var xlen [2]byte
		if _, err := io.ReadFull(r, xlen[:]); err != nil {
			return h, fmt.Errorf("FEXTRA 长度被截断（%v）", err)
		}
		n := int(binary.LittleEndian.Uint16(xlen[:]))
		extra := make([]byte, n)
		if _, err := io.ReadFull(r, extra); err != nil {
			return h, fmt.Errorf("FEXTRA 内容被截断（%v）", err)
		}
		raw.Write(xlen[:])
		raw.Write(extra)
	}
	if flg&0x08 != 0 { // FNAME
		s, err := readCString(r)
		if err != nil {
			return h, fmt.Errorf("FNAME 被截断（%v）", err)
		}
		raw.WriteString(s)
	}
	if flg&0x10 != 0 { // FCOMMENT
		s, err := readCString(r)
		if err != nil {
			return h, fmt.Errorf("FCOMMENT 被截断（%v）", err)
		}
		raw.WriteString(s)
	}
	if flg&0x02 != 0 { // FHCRC
		var crc [2]byte
		if _, err := io.ReadFull(r, crc[:]); err != nil {
			return h, fmt.Errorf("FHCRC 被截断（%v）", err)
		}
		raw.Write(crc[:])
	}
	if flg&0xe0 != 0 {
		// 保留位非零：按 RFC 1952 是非法输入，不许静默。
		return h, fmt.Errorf("gzip FLG 保留位非零（0x%02x）", flg)
	}
	h.Raw = raw.Bytes()
	return h, nil
}

// readCString 读到 NUL（含 NUL 写回结果）。
func readCString(r io.Reader) (string, error) {
	var b []byte
	var one [1]byte
	for {
		if _, err := io.ReadFull(r, one[:]); err != nil {
			return "", err
		}
		b = append(b, one[0])
		if one[0] == 0 {
			return string(b), nil
		}
		if len(b) > 1<<20 {
			return "", fmt.Errorf("NUL 结尾字符串超过 1 MiB")
		}
	}
}

// readGzipContainer 读取容器级事实（调用方随后 Seek(0) 再解压）。
func readGzipContainer(r io.Reader) (Container, error) {
	h, err := readGzipHeader(r)
	if err != nil {
		return Container{}, err
	}
	c := Container{
		Kind: KindTarGz,
		Facts: []HeaderFact{
			{Key: "gzip.mtime", Value: strconv.FormatUint(uint64(h.MTime), 10)},
			{Key: "gzip.xfl", Value: strconv.Itoa(int(h.XFL))},
			{Key: "gzip.os", Value: strconv.Itoa(int(h.OS))},
		},
		RawHeader: h.Raw,
	}
	return c, nil
}
