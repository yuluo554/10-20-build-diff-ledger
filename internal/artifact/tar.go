// tar 装载器（手写 512 字节块解析）。
//
// 为什么不用 archive/tar：M2 需要**原始头部字节**（raw_header_sha256，data/README.md §4.3）
// 与**归档内序号**（T5）。archive/tar 把头部解析完就丢弃原始块，扩展头（PAX/GNU）
// 也不暴露原文。这里按 POSIX ustar + GNU/PAX 扩展头逐块解析，同时保留：
//   - 条目序号 Index（0 起，扩展头不算条目）；
//   - RawHeader = 本条目之前的扩展头原文（整块 + 数据 + 填充）+ 本条目头的保留区 [345:512]。
//
// 纪律：校验和必须通过（读不了 = exit 1，不静默）；未知类型显式报错。
package artifact

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	tarBlockSize = 512
	// 头部字段偏移（POSIX ustar）。
	offName     = 0
	offMode     = 100
	offUID      = 108
	offGID      = 116
	offSize     = 124
	offMTime    = 136
	offChksum   = 148
	offTypeflag = 156
	offLinkname = 157
	offUname    = 265
	offGname    = 297
	offPrefix   = 345
	// §4.3：name/linkname/magic/version/typeflag/uname/gname/devmajor/devminor/prefix
	// 之后的保留区（含 prefix 字段本身之后的 12 字节填充）。
	offReserved = 345
)

// paxOverrides 是 PAX 扩展头对下一条目头的覆盖值（只取 facts 关心的键）。
type paxOverrides struct {
	values map[string]string
}

// paxValue 取条目级覆盖，回落全局（'g'）覆盖。
func paxValue(entry, global *paxOverrides, k string) (string, bool) {
	if v, ok := entry.get(k); ok {
		return v, true
	}
	return global.get(k)
}

func (p *paxOverrides) get(k string) (string, bool) {
	if p == nil {
		return "", false
	}
	v, ok := p.values[k]
	return v, ok
}

// loadTar 流式解析一个 tar（可来自 gzip 解压流）。
func loadTar(r *bufio.Reader, maxEntryBytes int64, base string, source Kind) ([]Raw, error) {
	var out []Raw
	var pendingRaw [][]byte // 扩展头原文（按出现顺序）
	var pax paxOverrides
	var global paxOverrides // 'g' 扩展头：对所有后续条目生效
	var longName, longLink string
	index := 0

	for {
		blk := make([]byte, tarBlockSize)
		if _, err := io.ReadFull(r, blk); err != nil {
			if err == io.EOF {
				// 条目结束后没有收尾零块：宽容接受（GNU tar 也接受）。
				break
			}
			if err == io.ErrUnexpectedEOF {
				return nil, fmt.Errorf("%q：tar 头部被截断（不足 512 字节）", base)
			}
			return nil, fmt.Errorf("%q：tar 解析失败（%v）", base, err)
		}
		if isZeroBlock(blk) {
			// 归档结束（POSIX 要求两个零块；读到第一个即可停）。
			break
		}
		if err := verifyTarChecksum(blk); err != nil {
			return nil, fmt.Errorf("%q：tar 头部校验和错误（%v）", base, err)
		}

		typeflag := blk[offTypeflag]
		size, err := parseTarNumber(blk[offSize : offSize+12])
		if err != nil {
			return nil, fmt.Errorf("%q：无法解析头部 size 字段（%v）", base, err)
		}

		switch typeflag {
		case 'x', 'g': // PAX 扩展头（x = 下一条目，g = 全局）
			data, err := readTarData(r, size, base, "PAX 扩展头")
			if err != nil {
				return nil, err
			}
			pendingRaw = append(pendingRaw, appendTarBlock(blk, data))
			if typeflag == 'x' {
				if err := parsePAXRecords(data, &pax); err != nil {
					return nil, fmt.Errorf("%q：PAX 扩展头无法解析（%v）", base, err)
				}
			} else {
				// 'g' = 全局扩展头：对后续所有条目生效（不重置）。
				if err := parsePAXRecords(data, &global); err != nil {
					return nil, fmt.Errorf("%q：全局 PAX 扩展头无法解析（%v）", base, err)
				}
			}
			continue
		case 'L': // GNU 长文件名
			data, err := readTarData(r, size, base, "GNU 长文件名")
			if err != nil {
				return nil, err
			}
			pendingRaw = append(pendingRaw, appendTarBlock(blk, data))
			longName = strings.TrimRight(string(data), "\x00")
			continue
		case 'K': // GNU 长链接名
			data, err := readTarData(r, size, base, "GNU 长链接名")
			if err != nil {
				return nil, err
			}
			pendingRaw = append(pendingRaw, appendTarBlock(blk, data))
			longLink = strings.TrimRight(string(data), "\x00")
			continue
		}

		// PAX 的 size 可以覆盖头部（例如 >8GiB 或格式化器约定）。
		if v, ok := paxValue(&pax, &global, "size"); ok {
			n, perr := strconv.ParseInt(v, 10, 64)
			if perr != nil || n < 0 {
				return nil, fmt.Errorf("%q：PAX size 非法（%q）", base, v)
			}
			size = n
		}

		name := cString(blk[offName : offName+100])
		if prefix := cString(blk[offPrefix : offPrefix+155]); prefix != "" {
			name = prefix + "/" + name
		}
		if longName != "" {
			name = longName
		}
		linkname := cString(blk[offLinkname : offLinkname+100])
		if longLink != "" {
			linkname = longLink
		}
		if v, ok := paxValue(&pax, &global, "path"); ok {
			name = v
		}
		if v, ok := paxValue(&pax, &global, "linkpath"); ok {
			linkname = v
		}

		rel, err := NormalizeRel(name)
		if err != nil {
			return nil, fmt.Errorf("%q：%v", base, err)
		}
		if rel == "" {
			// 根条目：仍要吃掉数据（否则后续块错位）
			if _, err := readTarData(r, size, base, "根条目"); err != nil {
				return nil, err
			}
			pendingRaw, pax, longName, longLink = nil, paxOverrides{}, "", ""
			continue
		}

		info := TarInfo{
			Mode:     tarNumOrZero(blk[offMode : offMode+8]),
			UID:      tarNumOrZero(blk[offUID : offUID+8]),
			GID:      tarNumOrZero(blk[offGID : offGID+8]),
			Size:     size,
			MTime:    tarNumOrZero(blk[offMTime : offMTime+12]),
			Typeflag: typeflag,
			Uname:    cString(blk[offUname : offUname+32]),
			Gname:    cString(blk[offGname : offGname+32]),
			Linkname: linkname,
		}
		applyPAX(&info, &pax, &global)

		rawHeader := make([]byte, 0, 167+len(pendingRaw)*tarBlockSize)
		for _, p := range pendingRaw {
			rawHeader = append(rawHeader, p...)
		}
		rawHeader = append(rawHeader, blk[offReserved:tarBlockSize]...)

		var raw Raw
		switch typeflag {
		case '0', 0, '7': // 普通文件（v7 以 NUL 表示）
			if size > maxEntryBytes {
				return nil, fmt.Errorf("%q：条目 %q 大小 %d 超过上限 %d", base, rel, size, maxEntryBytes)
			}
			data, err := readTarData(r, size, base, rel)
			if err != nil {
				return nil, err
			}
			raw = Raw{Path: rel, Kind: EntryFile, Size: int64(len(data)), Data: data}
		case '5':
			if _, err := readTarData(r, size, base, rel); err != nil {
				return nil, err
			}
			raw = Raw{Path: rel, Kind: EntryDir}
		case '2', '1': // 符号链接 / 硬链接
			if _, err := readTarData(r, size, base, rel); err != nil {
				return nil, err
			}
			data := []byte(linkname)
			raw = Raw{Path: rel, Kind: EntrySymlink, Size: int64(len(data)), Data: data}
		default:
			// 设备文件/FIFO/旧格式等：显式拒绝，不静默跳过（未知即显式）
			return nil, fmt.Errorf("%q：条目 %q 的类型 %q 不受支持", base, rel, string(rune(typeflag)))
		}

		raw.Index = index
		raw.Source = source
		raw.RawHeader = rawHeader
		raw.Tar = &info
		out = append(out, raw)
		index++

		pendingRaw, pax, longName, longLink = nil, paxOverrides{}, "", ""
	}
	return out, nil
}

// readTarData 读取 size 字节内容 + 到下一块边界的填充。
func readTarData(r *bufio.Reader, size int64, base, what string) ([]byte, error) {
	if size < 0 {
		return nil, fmt.Errorf("%q：条目 %q 的 size 为负（%d）", base, what, size)
	}
	if size > int64(^uint32(0)) {
		return nil, fmt.Errorf("%q：条目 %q 的 size 过大（%d）", base, what, size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("%q：条目 %q 声明 %d 字节、实际读取失败（%v）", base, what, size, err)
	}
	if pad := padTo(int(size), tarBlockSize); pad > 0 {
		if _, err := io.CopyN(io.Discard, r, int64(pad)); err != nil {
			return nil, fmt.Errorf("%q：条目 %q 的块填充被截断（%v）", base, what, err)
		}
	}
	return buf, nil
}

// appendTarBlock 把扩展头的 512 块 + 数据 + 填充原样串起来（§4.3 的"扩展头原始字节"）。
func appendTarBlock(blk, data []byte) []byte {
	out := make([]byte, 0, tarBlockSize+len(data)+padTo(len(data), tarBlockSize))
	out = append(out, blk...)
	out = append(out, data...)
	if pad := padTo(len(data), tarBlockSize); pad > 0 {
		out = append(out, make([]byte, pad)...)
	}
	return out
}

func isZeroBlock(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

// verifyTarChecksum 同时接受无符号与有符号求和（旧 tar 的两种历史口径）。
func verifyTarChecksum(blk []byte) error {
	stored, err := parseTarNumber(blk[offChksum : offChksum+8])
	if err != nil {
		return fmt.Errorf("chksum 字段不可解析")
	}
	var unsigned, signed int64
	for i, c := range blk {
		if i >= offChksum && i < offChksum+8 {
			c = ' '
		}
		unsigned += int64(c)
		signed += int64(int8(c))
	}
	if stored != unsigned && stored != signed {
		return fmt.Errorf("stored=%d computed=%d", stored, unsigned)
	}
	return nil
}

// parseTarNumber 解析八进制字段（NUL/空格结尾），并支持 GNU base-256。
func parseTarNumber(b []byte) (int64, error) {
	if len(b) == 0 {
		return 0, nil
	}
	if b[0]&0x80 != 0 {
		// base-256：整字段是二进制补码。取低 8 字节即可覆盖 int64 范围。
		n := b
		if len(n) > 8 {
			n = n[len(n)-8:]
		}
		var u uint64
		for _, c := range n {
			u = u<<8 | uint64(c)
		}
		return int64(u), nil
	}
	s := strings.Trim(string(b), " \x00")
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 8, 64)
}

func tarNumOrZero(b []byte) int64 {
	n, err := parseTarNumber(b)
	if err != nil {
		return 0
	}
	return n
}

func cString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// parsePAXRecords 解析 "len key=value\n" 序列；未知 keyword 不报错
// （它们已经进入 raw_header 的扩展头原文，符合 §4.3 的兜底纪律）。
func parsePAXRecords(data []byte, out *paxOverrides) error {
	if out.values == nil {
		out.values = map[string]string{}
	}
	for len(data) > 0 {
		sp := bytes.IndexByte(data, ' ')
		if sp <= 0 {
			return fmt.Errorf("记录缺少长度前缀")
		}
		n, err := strconv.Atoi(string(data[:sp]))
		if err != nil || n <= sp || n > len(data) {
			return fmt.Errorf("记录长度非法（%q）", data[:sp])
		}
		rec := data[sp+1 : n]
		data = data[n:]
		if len(rec) > 0 && rec[len(rec)-1] == '\n' {
			rec = rec[:len(rec)-1]
		}
		eq := bytes.IndexByte(rec, '=')
		if eq <= 0 {
			return fmt.Errorf("记录缺少 key=value")
		}
		out.values[string(rec[:eq])] = string(rec[eq+1:])
	}
	return nil
}

func applyPAX(info *TarInfo, pax, global *paxOverrides) {
	if v, ok := paxValue(pax, global, "uid"); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			info.UID = n
		}
	}
	if v, ok := paxValue(pax, global, "gid"); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			info.GID = n
		}
	}
	if v, ok := paxValue(pax, global, "mtime"); ok {
		if dot := strings.IndexByte(v, '.'); dot >= 0 {
			v = v[:dot]
		}
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			info.MTime = n
		}
	}
	if v, ok := paxValue(pax, global, "uname"); ok {
		info.Uname = v
	}
	if v, ok := paxValue(pax, global, "gname"); ok {
		info.Gname = v
	}
}

func padTo(n, align int) int {
	if r := n % align; r != 0 {
		return align - r
	}
	return 0
}
