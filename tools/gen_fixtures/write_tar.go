// 手写 tar / tar.gz 写入器。
//
// 为什么不用 archive/tar.Writer：生成器必须**逐字段显式赋值**（uid/gid/uname/
// gname/mode/mtime 是样张的被测对象），而 tar.Writer 会做 format 推断与字段补全；
// 手写 512 字节块让"注入了什么"与"写进文件的字节"一一对应，便于 README 手工复现。
//
// gzip 同理：XFL/OS 是 T6 的被测字段，compress/gzip 的 Writer 取不到（它从
// level 推 XFL、OS 写死 255），所以这里直接写成员头 + compress/flate 流。
package main

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
)

// tarHeader 是生成器关心的 tar 头部字段（其余字段一律写零）。
type tarHeader struct {
	Name     string
	Mode     int64
	UID      int64
	GID      int64
	Size     int64
	MTime    int64
	Typeflag byte
	Linkname string
	Uname    string
	Gname    string
}

const blockSize = 512

// paxRecord 编码一条 PAX 记录（"len key=value\n"，len 含自身）。
func paxRecord(kv string) []byte {
	body := kv + "\n"
	n := len(body)
	for {
		s := fmt.Sprintf("%d %s", n, body)
		if len(s) == n {
			return []byte(s)
		}
		n = len(s)
	}
}

// writeTar 按给定顺序写 tar。零字节用两个 512 块收尾（POSIX 的最小合法尾部）。
//
// 条目带 PaxRecords 时，先写一个 typeflag='x' 的扩展头（c14 的未知 keyword 载体）。
func writeTar(w io.Writer, entries []tarEntry) error {
	for _, e := range entries {
		if len(e.PaxRecords) > 0 {
			var payload []byte
			for _, kv := range e.PaxRecords {
				payload = append(payload, paxRecord(kv)...)
			}
			base := e.Name
			if i := lastSlash(base); i >= 0 {
				base = base[i+1:]
			}
			ph, err := encodeTarHeader(tarHeader{
				Name:     "PaxHeaders/" + base,
				Mode:     0o644,
				Size:     int64(len(payload)),
				MTime:    0,
				Typeflag: 'x',
			})
			if err != nil {
				return err
			}
			if _, err := w.Write(ph); err != nil {
				return err
			}
			if _, err := w.Write(payload); err != nil {
				return err
			}
			if pad := padTo(len(payload), blockSize); pad > 0 {
				if _, err := w.Write(make([]byte, pad)); err != nil {
					return err
				}
			}
		}
		h := &tarHeader{
			Name:     e.Name,
			Mode:     e.Mode,
			UID:      e.UID,
			GID:      e.GID,
			MTime:    e.MTime,
			Uname:    e.Uname,
			Gname:    e.Gname,
			Typeflag: '0',
		}
		var data []byte
		switch e.Kind {
		case tarKindDir:
			h.Typeflag = '5'
			h.Size = 0
		case tarKindSymlink:
			h.Typeflag = '2'
			h.Size = 0
			h.Linkname = e.Linkname
		default:
			data = e.Data
			h.Size = int64(len(data))
		}
		blk, err := encodeTarHeader(*h)
		if err != nil {
			return err
		}
		if _, err := w.Write(blk); err != nil {
			return err
		}
		if len(data) > 0 {
			if _, err := w.Write(data); err != nil {
				return err
			}
			if pad := padTo(len(data), blockSize); pad > 0 {
				if _, err := w.Write(make([]byte, pad)); err != nil {
					return err
				}
			}
		}
	}
	// 两个全零块 = 归档结束
	_, err := w.Write(make([]byte, 2*blockSize))
	return err
}

// encodeTarHeader 产出一个 512 字节 USTAR 头。
func encodeTarHeader(h tarHeader) ([]byte, error) {
	if len(h.Name) == 0 || len(h.Name) > 100 {
		return nil, fmt.Errorf("tar 条目名长度非法（%d 字节）：%q", len(h.Name), h.Name)
	}
	if len(h.Uname) > 32 || len(h.Gname) > 32 {
		return nil, fmt.Errorf("tar uname/gname 超过 32 字节：%q / %q", h.Uname, h.Gname)
	}
	if h.Size < 0 || h.MTime < 0 {
		return nil, fmt.Errorf("tar size/mtime 不得为负：%q", h.Name)
	}

	blk := make([]byte, blockSize)
	copy(blk[0:100], h.Name)
	putOctal(blk[100:108], h.Mode)
	putOctal(blk[108:116], h.UID)
	putOctal(blk[116:124], h.GID)
	putOctal(blk[124:136], h.Size)
	putOctal(blk[136:148], h.MTime)
	// chksum（148:156）先填空格再求和
	for i := 148; i < 156; i++ {
		blk[i] = ' '
	}
	blk[156] = h.Typeflag
	copy(blk[157:257], h.Linkname)
	copy(blk[257:263], "ustar\x00")
	copy(blk[263:265], "00")
	copy(blk[265:297], h.Uname)
	copy(blk[297:329], h.Gname)
	putOctal(blk[329:337], 0) // devmajor
	putOctal(blk[337:345], 0) // devminor

	var sum int64
	for _, b := range blk {
		sum += int64(b)
	}
	// 6 位八进制 + NUL + 空格（POSIX 的经典形态）：占 148..155，156 是 typeflag
	copy(blk[148:154], []byte(fmt.Sprintf("%06o", sum)))
	blk[154] = 0
	blk[155] = ' '
	return blk, nil
}

// putOctal 写 NUL 结尾的定宽八进制字段。
func putOctal(dst []byte, v int64) {
	s := fmt.Sprintf("%0*o", len(dst)-1, v)
	copy(dst, s)
	dst[len(dst)-1] = 0
}

func padTo(n, align int) int {
	if r := n % align; r != 0 {
		return align - r
	}
	return 0
}

// lastSlash 返回最后一个 '/' 的下标（无则 -1）。
func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

// gzipParams 是 gzip 成员头的可注入字段（T6）。
type gzipParams struct {
	XFL    byte  // 2 = 最大压缩，4 = 最快
	OS     byte  // 3 = Unix，255 = unknown
	MTime  int64 // 0 = 未设置
	Level  int   // flate 压缩级别（与 XFL 解耦：样张只改头字段）
	NoName bool  // 始终 true：不写 FNAME，避免路径泄漏
}

// writeTarGz 写一个 tar.gz：成员头逐字节由 gzipParams 决定。
func writeTarGz(w io.Writer, entries []tarEntry, p gzipParams) error {
	var tarBuf bytes.Buffer
	if err := writeTar(&tarBuf, entries); err != nil {
		return err
	}
	payload := tarBuf.Bytes()

	// 成员头：ID1 ID2 CM FLG MTIME(4) XFL OS
	var head [10]byte
	head[0], head[1] = 0x1f, 0x8b
	head[2] = 8 // deflate
	head[3] = 0 // FLG：无 FEXTRA/FNAME/FCOMMENT/FHCRC
	binary.LittleEndian.PutUint32(head[4:8], uint32(p.MTime))
	head[8] = p.XFL
	head[9] = p.OS
	if _, err := w.Write(head[:]); err != nil {
		return err
	}

	level := p.Level
	if level == 0 {
		level = flate.BestCompression
	}
	fw, err := flate.NewWriter(w, level)
	if err != nil {
		return err
	}
	if _, err := fw.Write(payload); err != nil {
		return err
	}
	if err := fw.Close(); err != nil {
		return err
	}

	var tail [8]byte
	binary.LittleEndian.PutUint32(tail[0:4], crc32.ChecksumIEEE(payload))
	binary.LittleEndian.PutUint32(tail[4:8], uint32(len(payload)))
	_, err = w.Write(tail[:])
	return err
}
