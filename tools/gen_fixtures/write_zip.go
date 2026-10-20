// 手写 zip 写入器（plan/04 §三.6「zip 跨平台冻结」）。
//
// archive/zip 的 Writer 会把 CreatorVersion 的低字节改写成 zipVersion20、
// 并在 Modified 非零时自动追加 extended-timestamp extra 字段——这两个动作都会
// 覆盖生成器想注入的 T7 字段（create_system / external_attr）。所以这里直接写
// 本地头 + 中央目录 + EOCD，每个字段都来自 zipParams。
//
// 压缩方法固定 Store：deflate 的字节流随 Go 版本可能漂移，而 zip 样张的被测对象
// 是**头部字段**，不是压缩率。这正是 plan/04 §三.6 的退化处置。
package main

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
)

// zipParams 是归档级可注入字段（T7）。
type zipParams struct {
	CreateSystem  uint16 // 0 = FAT/MS-DOS，3 = Unix
	CreateVersion byte   // 作者版本（低字节）；写死以免版本漂移
	ExtractVer    uint16 // 解压所需版本
}

// zipEntry 是一个待写入的条目。
type zipEntry struct {
	Name      string
	Data      []byte
	IsDir     bool
	Mode      uint32 // unix 权限位（ExternalAttrs 的高 16 位）
	DOSTime   uint16
	DOSDate   uint16
	ExtraName []byte // 原样写入 extra field（未知头反例用）
}

// writeZip 产出确定性的 zip 字节流。
func writeZip(w io.Writer, entries []zipEntry, p zipParams) error {
	type central struct {
		name    string
		crc     uint32
		size    uint32
		offset  uint32
		extAttr uint32
		dosTime uint16
		dosDate uint16
		method  uint16
		extra   []byte
	}

	var centrals []central
	var offset uint32
	for _, e := range entries {
		data := e.Data
		if e.IsDir {
			data = nil
		}
		size := uint32(len(data))
		crc := crc32.ChecksumIEEE(data)
		const method = uint16(0) // Store

		nameBytes := []byte(e.Name)
		extra := append([]byte(nil), e.ExtraName...)
		extAttr := e.Mode << 16
		if e.IsDir {
			extAttr |= 0x10 // MS-DOS directory bit
		}
		start := offset

		var lh [30]byte
		binary.LittleEndian.PutUint32(lh[0:4], 0x04034b50)
		binary.LittleEndian.PutUint16(lh[4:6], p.ExtractVer)
		binary.LittleEndian.PutUint16(lh[6:8], 0) // flags：无 UTF-8 位、无 data descriptor
		binary.LittleEndian.PutUint16(lh[8:10], method)
		binary.LittleEndian.PutUint16(lh[10:12], e.DOSTime)
		binary.LittleEndian.PutUint16(lh[12:14], e.DOSDate)
		binary.LittleEndian.PutUint32(lh[14:18], crc)
		binary.LittleEndian.PutUint32(lh[18:22], size)
		binary.LittleEndian.PutUint32(lh[22:26], size)
		binary.LittleEndian.PutUint16(lh[26:28], uint16(len(nameBytes)))
		binary.LittleEndian.PutUint16(lh[28:30], uint16(len(extra)))

		for _, part := range [][]byte{lh[:], nameBytes, extra, data} {
			if _, err := w.Write(part); err != nil {
				return err
			}
			offset += uint32(len(part))
		}
		centrals = append(centrals, central{
			name: e.Name, crc: crc, size: size, offset: start,
			extAttr: extAttr, dosTime: e.DOSTime, dosDate: e.DOSDate, method: method, extra: extra,
		})
	}

	cdStart := offset
	for _, c := range centrals {
		nameBytes := []byte(c.name)
		var ch [46]byte
		binary.LittleEndian.PutUint32(ch[0:4], 0x02014b50)
		binary.LittleEndian.PutUint16(ch[4:6], p.CreateSystem<<8|uint16(p.CreateVersion))
		binary.LittleEndian.PutUint16(ch[6:8], p.ExtractVer)
		binary.LittleEndian.PutUint16(ch[8:10], 0) // flags
		binary.LittleEndian.PutUint16(ch[10:12], c.method)
		binary.LittleEndian.PutUint16(ch[12:14], c.dosTime)
		binary.LittleEndian.PutUint16(ch[14:16], c.dosDate)
		binary.LittleEndian.PutUint32(ch[16:20], c.crc)
		binary.LittleEndian.PutUint32(ch[20:24], c.size)
		binary.LittleEndian.PutUint32(ch[24:28], c.size)
		binary.LittleEndian.PutUint16(ch[28:30], uint16(len(nameBytes)))
		binary.LittleEndian.PutUint16(ch[30:32], uint16(len(c.extra)))
		binary.LittleEndian.PutUint16(ch[32:34], 0) // comment len
		binary.LittleEndian.PutUint16(ch[34:36], 0) // disk start
		binary.LittleEndian.PutUint16(ch[36:38], 0) // internal attrs
		binary.LittleEndian.PutUint32(ch[38:42], c.extAttr)
		binary.LittleEndian.PutUint32(ch[42:46], c.offset)

		for _, part := range [][]byte{ch[:], nameBytes, c.extra} {
			if _, err := w.Write(part); err != nil {
				return err
			}
			offset += uint32(len(part))
		}
	}
	cdSize := offset - cdStart

	var eocd [22]byte
	binary.LittleEndian.PutUint32(eocd[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(eocd[4:6], 0) // disk
	binary.LittleEndian.PutUint16(eocd[6:8], 0) // cd start disk
	binary.LittleEndian.PutUint16(eocd[8:10], uint16(len(centrals)))
	binary.LittleEndian.PutUint16(eocd[10:12], uint16(len(centrals)))
	binary.LittleEndian.PutUint32(eocd[12:16], cdSize)
	binary.LittleEndian.PutUint32(eocd[16:20], cdStart)
	binary.LittleEndian.PutUint16(eocd[20:22], 0) // comment len
	_, err := w.Write(eocd[:])
	return err
}

// dosDateTime 把白名单时间戳转成 MS-DOS 的 (time,date)。
// 只接受 {0, 1700000000, 1700003600}（data/README.md §三）；
// 按 UTC 口径手算，不引 time 包。unix=0 落 MS-DOS 最小合法日期 1980-01-01。
// 白名单外的值返回错误：静默写 0 会让"注入了什么"与"落盘的字节"脱节。
func dosDateTime(unix int64) (uint16, uint16, error) {
	switch unix {
	case 0:
		return 0, 0x0021, nil // 1980-01-01 00:00:00
	case 1700000000, 1700003600:
		t, d := dosTimeOf(unix)
		return t, d, nil
	default:
		return 0, 0, fmt.Errorf("时间戳 %d 不在白名单 {0, 1700000000, 1700003600} 内", unix)
	}
}

func dosTimeOf(unix int64) (uint16, uint16) {
	const day = int64(86400)
	days := unix / day
	secs := unix % day
	y, m, d := civilFromDays(days)
	t := uint16(secs/3600<<11 | (secs%3600)/60<<5 | (secs%60)/2)
	dosD := uint16((y-1980)<<9 | m<<5 | d)
	return t, dosD
}

// civilFromDays 是 1970-01-01 起的天数 → (year, month, day)（Howard Hinnant 算法；
// 不引 time 包）。
func civilFromDays(z int64) (int64, int64, int64) {
	z += 719468
	var era int64
	if z >= 0 {
		era = z / 146097
	} else {
		era = (z - 146096) / 146097
	}
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d := doy - (153*mp+2)/5 + 1
	m := mp + 3
	if mp >= 10 {
		m = mp - 9
	}
	if m <= 2 {
		y++
	}
	return y, m, d
}
