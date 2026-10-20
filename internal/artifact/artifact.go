// Package artifact 负责把输入路径装载成内存中的条目流。
//
// 支持的形态（plan/03 §二）：目录、tar、tar.gz、zip、whl、jar、OCI layer tar。
// 纪律：解到内存、不落临时目录、不写输入目录、不做大小写折叠。
//
// 形态识别只认**字节魔数**，不认扩展名：`.whl` / `.jar` 就是 zip，
// OCI layer 由 `--layer-kind=oci` 显式声明（不嗅探，避免把普通 tar 猜成镜像层）。
package artifact

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DefaultMaxEntryBytes 是单条目上限的默认值（data/README.md §4.9）。
const DefaultMaxEntryBytes int64 = 256 << 20

// Kind 是容器的形态。
type Kind string

const (
	KindDir   Kind = "dir"
	KindTar   Kind = "tar"
	KindTarGz Kind = "tar.gz"
	KindZip   Kind = "zip"
	KindOCI   Kind = "oci-layer-tar"
)

// 条目形态（Raw.Kind 的取值）。
const (
	EntryFile    = "file"
	EntryDir     = "dir"
	EntrySymlink = "symlink"
)

// Raw 是装载后的一个条目。Path 已规范化为 `/` 分隔、无前导 `./`。
type Raw struct {
	Path string
	Kind string // EntryFile | EntryDir | EntrySymlink
	Size int64
	// Data 是 file 的内容字节；symlink 存链接目标（无目标则空，与目录同为 0 字节）；
	// dir 恒为 nil。这样 sha256 对三类条目都有意义，不会把不同软链当成同一条。
	Data []byte
}

// Load 装载一个产物，返回条目表与容器形态。
//
// maxEntryBytes <= 0 时取 DefaultMaxEntryBytes；超限返回错误并**指名条目**
// （安全阀必须可见，不静默丢弃）。layerKind 只接受 "" 与 "oci"。
func Load(name, layerKind string, maxEntryBytes int64) ([]Raw, Kind, error) {
	if layerKind != "" && layerKind != "oci" {
		return nil, "", fmt.Errorf("未知 --layer-kind %q（只支持 oci）", layerKind)
	}
	if maxEntryBytes <= 0 {
		maxEntryBytes = DefaultMaxEntryBytes
	}
	info, err := os.Stat(name)
	if err != nil {
		// 不回显绝对路径（脱敏纪律，plan/02 §六）
		return nil, "", fmt.Errorf("无法读取 %q：路径不存在或不可访问", filepath.Base(name))
	}
	base := filepath.Base(name)
	if info.IsDir() {
		if layerKind == "oci" {
			return nil, "", fmt.Errorf("--layer-kind=oci 只接受归档输入（%q 是目录）", base)
		}
		raws, err := loadDir(name, maxEntryBytes)
		return raws, KindDir, err
	}
	if !info.Mode().IsRegular() {
		return nil, "", fmt.Errorf("%q 既不是目录也不是普通文件", base)
	}

	f, err := os.Open(name)
	if err != nil {
		return nil, "", fmt.Errorf("无法读取 %q：路径不存在或不可访问", base)
	}
	defer f.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, "", fmt.Errorf("无法读取 %q：%v", base, err)
	}
	head = head[:n]

	switch sniff(head) {
	case KindTarGz:
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, "", fmt.Errorf("无法读取 %q：%v", base, err)
		}
		raws, err := loadTar(f, true, maxEntryBytes, base)
		return raws, ociKind(KindTarGz, layerKind), err
	case KindZip:
		if layerKind == "oci" {
			return nil, "", fmt.Errorf("%q 是 zip，不能按 OCI layer tar 装载", base)
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, "", fmt.Errorf("无法读取 %q：%v", base, err)
		}
		raws, err := loadZip(f, info.Size(), maxEntryBytes, base)
		return raws, KindZip, err
	case KindTar:
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, "", fmt.Errorf("无法读取 %q：%v", base, err)
		}
		raws, err := loadTar(f, false, maxEntryBytes, base)
		return raws, ociKind(KindTar, layerKind), err
	default:
		// 魔数不认识：仍尝试按 tar 解析（v7 tar 没有 ustar 魔数）。
		// 解析失败**或一个条目都没读出来** → 显式报"不支持的格式"，
		// 绝不把"看不懂"当成"空树 / 一致"（静默通过的经典漏洞）。
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, "", fmt.Errorf("无法读取 %q：%v", base, err)
		}
		raws, err := loadTar(f, false, maxEntryBytes, base)
		if err != nil || len(raws) == 0 {
			return nil, "", fmt.Errorf("%q：无法识别容器格式（支持目录 / tar / tar.gz / zip / whl / jar）", base)
		}
		return raws, ociKind(KindTar, layerKind), nil
	}
}

func ociKind(k Kind, layerKind string) Kind {
	if layerKind == "oci" {
		return KindOCI
	}
	return k
}

// sniff 只看头部字节判定容器形态；返回 "" 表示未识别。
func sniff(head []byte) Kind {
	if len(head) >= 2 && head[0] == 0x1f && head[1] == 0x8b {
		return KindTarGz
	}
	if len(head) >= 4 && head[0] == 'P' && head[1] == 'K' &&
		(head[2] == 3 || head[2] == 5 || head[2] == 7) {
		return KindZip
	}
	if len(head) >= 262 && string(head[257:262]) == "ustar" {
		return KindTar
	}
	return ""
}

// NormalizeRel 规范化条目路径（plan/03 §二.2）：
// `/` 分隔 → 去空段与前导 `./` → 折叠重复 `/` → 拒绝 `..` 与绝对路径。
// 返回 "" 表示"根条目"（调用方应跳过）。不做大小写折叠。
func NormalizeRel(p string) (string, error) {
	s := filepath.ToSlash(p)
	if strings.HasPrefix(s, "/") || hasDriveLetter(s) {
		return "", fmt.Errorf("条目路径必须是相对路径：%q", p)
	}
	parts := strings.Split(s, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			// 不改写成别的路径：越界就是坏输入（plan/03 §二.2）
			return "", fmt.Errorf("条目路径越界（含 `..`）：%q", p)
		default:
			out = append(out, part)
		}
	}
	return strings.Join(out, "/"), nil
}

// hasDriveLetter 识别 Windows 盘符形态（C:/... 或 C:\...，ToSlash 后统一为 C:/）。
func hasDriveLetter(s string) bool {
	return len(s) >= 2 && s[1] == ':' &&
		((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z'))
}

func loadDir(root string, maxEntryBytes int64) ([]Raw, error) {
	var out []Raw
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("无法读取 %q：%v", filepath.Base(root), err)
		}
		if p == root {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		relSlash, err := NormalizeRel(rel)
		if err != nil {
			return err
		}
		if relSlash == "" {
			return nil
		}
		switch {
		case d.IsDir():
			out = append(out, Raw{Path: relSlash, Kind: EntryDir})
		case d.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return fmt.Errorf("条目 %q：无法读取软链目标（%v）", relSlash, err)
			}
			data := []byte(filepath.ToSlash(target))
			out = append(out, Raw{Path: relSlash, Kind: EntrySymlink, Size: int64(len(data)), Data: data})
		case d.Type().IsRegular():
			info, err := d.Info()
			if err != nil {
				return fmt.Errorf("条目 %q：无法读取大小（%v）", relSlash, err)
			}
			if info.Size() > maxEntryBytes {
				return fmt.Errorf("条目 %q 大小 %d 超过上限 %d", relSlash, info.Size(), maxEntryBytes)
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("条目 %q：无法读取内容（%v）", relSlash, err)
			}
			out = append(out, Raw{Path: relSlash, Kind: EntryFile, Size: int64(len(data)), Data: data})
		default:
			return fmt.Errorf("条目 %q：不支持的目录项类型 %v", relSlash, d.Type())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func loadTar(r io.Reader, gzipped bool, maxEntryBytes int64, base string) ([]Raw, error) {
	if gzipped {
		zr, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("%q：gzip 头无法解析（%v）", base, err)
		}
		defer zr.Close()
		r = zr
	}
	tr := tar.NewReader(r)
	var out []Raw
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%q：tar 解析失败（%v）", base, err)
		}
		rel, err := NormalizeRel(h.Name)
		if err != nil {
			return nil, fmt.Errorf("%q：%v", base, err)
		}
		if rel == "" {
			continue
		}
		switch h.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			if h.Size > maxEntryBytes {
				return nil, fmt.Errorf("%q：条目 %q 大小 %d 超过上限 %d", base, rel, h.Size, maxEntryBytes)
			}
			data, err := io.ReadAll(io.LimitReader(tr, h.Size))
			if err != nil {
				return nil, fmt.Errorf("%q：条目 %q 读取失败（%v）", base, rel, err)
			}
			if int64(len(data)) != h.Size {
				return nil, fmt.Errorf("%q：条目 %q 声明 %d 字节、实际 %d 字节（归档被截断？）", base, rel, h.Size, len(data))
			}
			out = append(out, Raw{Path: rel, Kind: EntryFile, Size: h.Size, Data: data})
		case tar.TypeDir:
			out = append(out, Raw{Path: rel, Kind: EntryDir})
		case tar.TypeSymlink, tar.TypeLink:
			data := []byte(filepath.ToSlash(h.Linkname))
			out = append(out, Raw{Path: rel, Kind: EntrySymlink, Size: int64(len(data)), Data: data})
		default:
			// 设备文件/FIFO 等：显式拒绝，不静默跳过（未知即显式）
			return nil, fmt.Errorf("%q：条目 %q 的类型 %q 不受支持", base, rel, string(rune(h.Typeflag)))
		}
	}
	return out, nil
}

func loadZip(f *os.File, size, maxEntryBytes int64, base string) ([]Raw, error) {
	zr, err := zip.NewReader(f, size)
	if err != nil {
		return nil, fmt.Errorf("%q：zip 解析失败（%v）", base, err)
	}
	var out []Raw
	for _, zf := range zr.File {
		rel, err := NormalizeRel(zf.Name)
		if err != nil {
			return nil, fmt.Errorf("%q：%v", base, err)
		}
		if rel == "" {
			continue
		}
		mode := zf.Mode()
		switch {
		case zf.FileInfo().IsDir() || strings.HasSuffix(zf.Name, "/"):
			out = append(out, Raw{Path: rel, Kind: EntryDir})
			continue
		case mode&os.ModeSymlink != 0:
			// 软链目标存放在 zip 的内容里
			data, err := readZipEntry(zf, maxEntryBytes)
			if err != nil {
				return nil, fmt.Errorf("%q：条目 %q %v", base, rel, err)
			}
			out = append(out, Raw{Path: rel, Kind: EntrySymlink, Size: int64(len(data)), Data: data})
			continue
		default:
			if zf.UncompressedSize64 > uint64(maxEntryBytes) {
				return nil, fmt.Errorf("%q：条目 %q 大小 %d 超过上限 %d", base, rel, zf.UncompressedSize64, maxEntryBytes)
			}
			data, err := readZipEntry(zf, maxEntryBytes)
			if err != nil {
				return nil, fmt.Errorf("%q：条目 %q %v", base, rel, err)
			}
			out = append(out, Raw{Path: rel, Kind: EntryFile, Size: int64(len(data)), Data: data})
		}
	}
	return out, nil
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
