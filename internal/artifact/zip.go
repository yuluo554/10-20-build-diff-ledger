// zip 装载器：数据用 archive/zip 解压，头部原文自己读。
//
// 为什么头部要自己读：§4.3 要求 raw_header_sha256 覆盖"中央目录记录与本地文件头中
// 未被 facts 抽取的字段（含 extra field 原文）"。archive/zip 只给出 FileHeader 的
// 结构体字段，不暴露本地头字节与中央目录记录原文；这里按 EOCD → 中央目录 → 本地头
// 逐字段读，并把未抽取字段按固定顺序拼成确定性的字节序列，每个分量带
// le32 长度前缀（构造口径写进 data/README.md §4.3）：
//
//	分量顺序 = 本地头[4:6], [6:8], [14:26], localExtra,
//	          中央目录[8:10], [16:28], [32:38], [42:46], centralExtra, comment
//	raw      = concat_{i} ( le32(len(part_i)) ‖ part_i )
//
// 被抽取（因而**不**进入 raw）的字段：name、method、date/time、create_system
// （version-made-by 高字节）、external_attr。
package artifact

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// zipCentral 是一条中央目录记录里本包关心的字段 + 原文。
type zipCentral struct {
	name           string
	creatorVersion uint16
	flags          uint16
	method         uint16
	modifiedTime   uint16
	modifiedDate   uint16
	crc            uint32
	compressedSize uint32
	uncompSize     uint32
	diskStart      uint16
	internalAttrs  uint16
	externalAttrs  uint32
	localOffset    uint32
	extra          []byte
	comment        []byte
	raw            []byte
}

// readZipCentral 读 EOCD 与中央目录。zip64 显式拒绝（未知即显式）。
func readZipCentral(f *os.File, size int64) ([]zipCentral, error) {
	if size < 22 {
		return nil, fmt.Errorf("zip 过短（%d 字节）", size)
	}
	tailLen := int64(22 + 0xffff)
	if tailLen > size {
		tailLen = size
	}
	tail := make([]byte, tailLen)
	if _, err := f.ReadAt(tail, size-tailLen); err != nil {
		return nil, fmt.Errorf("读取 zip 尾部失败（%v）", err)
	}
	idx := bytes.LastIndex(tail, []byte("PK\x05\x06"))
	if idx < 0 || idx+22 > len(tail) {
		return nil, fmt.Errorf("找不到 EOCD（不是 zip）")
	}
	eocd := tail[idx:]
	total := binary.LittleEndian.Uint16(eocd[10:12])
	cdSize := binary.LittleEndian.Uint32(eocd[12:16])
	cdOff := binary.LittleEndian.Uint32(eocd[16:20])
	if total == 0xffff || cdSize == 0xffffffff || cdOff == 0xffffffff {
		return nil, fmt.Errorf("zip64 暂不支持")
	}
	if int64(cdOff)+int64(cdSize) > size {
		return nil, fmt.Errorf("中央目录越界（offset=%d size=%d file=%d）", cdOff, cdSize, size)
	}
	cd := make([]byte, cdSize)
	if _, err := f.ReadAt(cd, int64(cdOff)); err != nil {
		return nil, fmt.Errorf("读取中央目录失败（%v）", err)
	}

	out := make([]zipCentral, 0, total)
	pos := 0
	for i := 0; i < int(total); i++ {
		if pos+46 > len(cd) {
			return nil, fmt.Errorf("中央目录记录被截断")
		}
		if binary.LittleEndian.Uint32(cd[pos:]) != 0x02014b50 {
			return nil, fmt.Errorf("中央目录记录签名错误")
		}
		nameLen := int(binary.LittleEndian.Uint16(cd[pos+28 : pos+30]))
		extraLen := int(binary.LittleEndian.Uint16(cd[pos+30 : pos+32]))
		commentLen := int(binary.LittleEndian.Uint16(cd[pos+32 : pos+34]))
		end := pos + 46 + nameLen + extraLen + commentLen
		if end > len(cd) {
			return nil, fmt.Errorf("中央目录记录越界")
		}
		rec := cd[pos:end]
		recLen := end - pos
		out = append(out, zipCentral{
			name:           string(rec[46 : 46+nameLen]),
			creatorVersion: binary.LittleEndian.Uint16(rec[4:6]),
			flags:          binary.LittleEndian.Uint16(rec[8:10]),
			method:         binary.LittleEndian.Uint16(rec[10:12]),
			modifiedTime:   binary.LittleEndian.Uint16(rec[12:14]),
			modifiedDate:   binary.LittleEndian.Uint16(rec[14:16]),
			crc:            binary.LittleEndian.Uint32(rec[16:20]),
			compressedSize: binary.LittleEndian.Uint32(rec[20:24]),
			uncompSize:     binary.LittleEndian.Uint32(rec[24:28]),
			diskStart:      binary.LittleEndian.Uint16(rec[34:36]),
			internalAttrs:  binary.LittleEndian.Uint16(rec[36:38]),
			externalAttrs:  binary.LittleEndian.Uint32(rec[38:42]),
			localOffset:    binary.LittleEndian.Uint32(rec[42:46]),
			extra:          append([]byte(nil), rec[46+nameLen:46+nameLen+extraLen]...),
			comment:        append([]byte(nil), rec[46+nameLen+extraLen:recLen]...),
			raw:            append([]byte(nil), rec...),
		})
		pos = end
	}
	return out, nil
}

// localRaw 读本地文件头，返回未抽取字段的原文与校验后的名称。
func localRaw(f *os.File, c zipCentral) (raw []byte, err error) {
	lh := make([]byte, 30)
	if _, err := f.ReadAt(lh, int64(c.localOffset)); err != nil {
		return nil, fmt.Errorf("读取本地头失败（%v）", err)
	}
	if binary.LittleEndian.Uint32(lh) != 0x04034b50 {
		return nil, fmt.Errorf("本地头签名错误（offset=%d）", c.localOffset)
	}
	nameLen := int(binary.LittleEndian.Uint16(lh[26:28]))
	extraLen := int(binary.LittleEndian.Uint16(lh[28:30]))
	if name, err := readAt(f, int64(c.localOffset)+30, nameLen); err != nil {
		return nil, err
	} else if string(name) != c.name {
		return nil, fmt.Errorf("本地头文件名与中央目录不一致（%q vs %q）", name, c.name)
	}
	extra, err := readAt(f, int64(c.localOffset)+30+int64(nameLen), extraLen)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	writeU32Prefixed(&b, lh[4:6])
	writeU32Prefixed(&b, lh[6:8])
	writeU32Prefixed(&b, lh[14:26])
	writeU32Prefixed(&b, extra)
	return b.Bytes(), nil
}

func readAt(f *os.File, off int64, n int) ([]byte, error) {
	if n == 0 {
		return nil, nil
	}
	b := make([]byte, n)
	if _, err := f.ReadAt(b, off); err != nil {
		return nil, fmt.Errorf("读取 zip 头部失败（%v）", err)
	}
	return b, nil
}

// writeU32Prefixed 写入 le32(len) ‖ 原文，避免拼接歧义。
func writeU32Prefixed(b *bytes.Buffer, p []byte) {
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(p)))
	b.Write(n[:])
	b.Write(p)
}

// loadZip 装载 zip（含 whl / jar）。
func loadZip(f *os.File, size, maxEntryBytes int64, base string) ([]Raw, Container, error) {
	centrals, err := readZipCentral(f, size)
	if err != nil {
		return nil, Container{}, fmt.Errorf("%q：zip 解析失败（%v）", base, err)
	}
	zr, err := zip.NewReader(f, size)
	if err != nil {
		return nil, Container{}, fmt.Errorf("%q：zip 解析失败（%v）", base, err)
	}
	if len(zr.File) != len(centrals) {
		return nil, Container{}, fmt.Errorf("%q：中央目录条目数 %d 与解析器 %d 不一致", base, len(centrals), len(zr.File))
	}

	c := Container{Kind: KindZip}
	if len(centrals) > 0 {
		c.Facts = []HeaderFact{{
			Key:   "zip.create_system",
			Value: fmt.Sprintf("%d", centrals[0].creatorVersion>>8),
		}}
	}

	var out []Raw
	for i, zf := range zr.File {
		zc := centrals[i]
		if zf.Name != zc.name {
			return nil, Container{}, fmt.Errorf("%q：第 %d 个条目名不一致（%q vs %q）", base, i, zf.Name, zc.name)
		}
		rel, err := NormalizeRel(zc.name)
		if err != nil {
			return nil, Container{}, fmt.Errorf("%q：%v", base, err)
		}
		if rel == "" {
			continue
		}
		local, err := localRaw(f, zc)
		if err != nil {
			return nil, Container{}, fmt.Errorf("%q：条目 %q %v", base, rel, err)
		}
		var ch bytes.Buffer
		writeU32Prefixed(&ch, zc.raw[8:10])
		writeU32Prefixed(&ch, zc.raw[16:28])
		writeU32Prefixed(&ch, zc.raw[32:38])
		writeU32Prefixed(&ch, zc.raw[42:46])
		writeU32Prefixed(&ch, zc.extra)
		writeU32Prefixed(&ch, zc.comment)
		header := append(append([]byte(nil), local...), ch.Bytes()...)

		info := &ZipInfo{
			Method:         zc.method,
			ModifiedDate:   zc.modifiedDate,
			ModifiedTime:   zc.modifiedTime,
			ExternalAttrs:  zc.externalAttrs,
			CreatorVersion: zc.creatorVersion,
			Flags:          zc.flags,
			CompressedSize: zc.compressedSize,
		}

		raw := Raw{
			Path:      rel,
			Index:     i,
			Source:    KindZip,
			RawHeader: header,
			Zip:       info,
		}
		mode := zf.Mode()
		switch {
		case zf.FileInfo().IsDir() || strings.HasSuffix(zc.name, "/"):
			raw.Kind = EntryDir
		case mode&os.ModeSymlink != 0:
			data, err := readZipEntry(zf, maxEntryBytes)
			if err != nil {
				return nil, Container{}, fmt.Errorf("%q：条目 %q %v", base, rel, err)
			}
			raw.Kind = EntrySymlink
			raw.Size = int64(len(data))
			raw.Data = data
		default:
			if zf.UncompressedSize64 > uint64(maxEntryBytes) {
				return nil, Container{}, fmt.Errorf("%q：条目 %q 大小 %d 超过上限 %d",
					base, rel, zf.UncompressedSize64, maxEntryBytes)
			}
			data, err := readZipEntry(zf, maxEntryBytes)
			if err != nil {
				return nil, Container{}, fmt.Errorf("%q：条目 %q %v", base, rel, err)
			}
			raw.Kind = EntryFile
			raw.Size = int64(len(data))
			raw.Data = data
		}
		out = append(out, raw)
	}
	return out, c, nil
}

func readZipEntry(zf *zip.File, maxEntryBytes int64) ([]byte, error) {
	rc, err := zf.Open()
	if err != nil {
		return nil, fmt.Errorf("打开失败（%v）", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, maxEntryBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取失败（%v）", err)
	}
	if int64(len(data)) > maxEntryBytes {
		return nil, fmt.Errorf("解压后超过上限 %d 字节", maxEntryBytes)
	}
	return data, nil
}
