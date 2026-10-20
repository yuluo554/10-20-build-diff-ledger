// Package artifact 负责把输入路径装载成内存中的条目流。
//
// 支持的形态（plan/03 §二）：目录、tar、tar.gz、zip、whl、jar、OCI layer tar。
// 纪律：解到内存、不落临时目录、不写输入目录、不做大小写折叠。
//
// 形态识别只认**字节魔数**，不认扩展名：`.whl` / `.jar` 就是 zip，
// OCI layer 由 `--layer-kind=oci` 显式声明（不嗅探，避免把普通 tar 猜成镜像层）。
//
// M2 起，装载结果还带两类 facts 输入（plan/03 §三，data/README.md §4.3）：
//   - 条目序号 Index（T5 的判定依据；目录输入没有归档顺序，恒为 -1）；
//   - 原始头部字节 RawHeader（解析器已消费字段之外的部分，由 facts 计算 sha256）；
//   - 容器级头部 Container（gzip.mtime/xfl/os、zip.create_system 不属于任何条目）。
package artifact

import (
	"bufio"
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

// TarInfo 是 tar 条目头里被 facts 抽取的字段（键名映射在 internal/facts）。
// nil = 该条目不是 tar 装载出来的。
type TarInfo struct {
	Mode     int64
	UID      int64
	GID      int64
	Size     int64
	MTime    int64
	Typeflag byte
	Uname    string
	Gname    string
	Linkname string
}

// ZipInfo 是 zip 条目头里被 facts 抽取的字段。
// nil = 该条目不是 zip 装载出来的。
type ZipInfo struct {
	Method         uint16
	ModifiedDate   uint16
	ModifiedTime   uint16
	ExternalAttrs  uint32
	CreatorVersion uint16
	Flags          uint16
	CompressedSize uint32
}

// Raw 是装载后的一个条目。Path 已规范化为 `/` 分隔、无前导 `./`。
type Raw struct {
	Path string
	Kind string // EntryFile | EntryDir | EntrySymlink
	Size int64
	// Data 是 file 的内容字节；symlink 存链接目标（无目标则空，与目录同为 0 字节）；
	// dir 恒为 nil。这样 sha256 对三类条目都有意义，不会把不同软链当成同一条。
	Data []byte

	// Index 是条目在归档内的序号（0 起，按读取顺序，含目录条目）。
	// 目录输入没有"归档顺序"这一概念，恒为 -1（显式未知，不伪造 0）。
	Index int
	// Source 是条目实际来自哪种容器（OCI 层也记录其底层容器：tar / tar.gz），
	// 供 facts 选择头部解释方式。目录输入为 KindDir。
	Source Kind
	// RawHeader 是"解析器已消费字段之外"的原始头部字节（data/README.md §4.3）。
	// tar：保留区 + 本条目之前的 PAX/GNU 扩展头原文；zip：本地头/中央目录的
	// 未抽取字段 + extra + comment；目录输入：文件自身的格式头（由 facts 计算）。
	RawHeader []byte

	Tar *TarInfo
	Zip *ZipInfo
}

// HeaderFact 是容器级头部字段（原样字符串，不做语义解释）。
type HeaderFact struct {
	Key   string
	Value string
}

// Container 是容器级事实与原始头部。路径约定为空串（`""` = 容器自身）。
type Container struct {
	Kind      Kind
	Facts     []HeaderFact
	RawHeader []byte
}

// Document 是一次装载的完整结果。
type Document struct {
	Kind      Kind
	Entries   []Raw
	Container Container
}

// Load 装载一个产物，返回条目表与容器形态。
//
// maxEntryBytes <= 0 时取 DefaultMaxEntryBytes；超限返回错误并**指名条目**
// （安全阀必须可见，不静默丢弃）。layerKind 只接受 "" 与 "oci"。
func Load(name, layerKind string, maxEntryBytes int64) ([]Raw, Kind, error) {
	doc, err := LoadDocument(name, layerKind, maxEntryBytes)
	if err != nil {
		return nil, "", err
	}
	return doc.Entries, doc.Kind, nil
}

// LoadDocument 与 Load 相同，但额外返回容器级事实（M2 新增）。
func LoadDocument(name, layerKind string, maxEntryBytes int64) (Document, error) {
	if layerKind != "" && layerKind != "oci" {
		return Document{}, fmt.Errorf("未知 --layer-kind %q（只支持 oci）", layerKind)
	}
	if maxEntryBytes <= 0 {
		maxEntryBytes = DefaultMaxEntryBytes
	}
	info, err := os.Stat(name)
	if err != nil {
		// 不回显绝对路径（脱敏纪律，plan/02 §六）
		return Document{}, fmt.Errorf("无法读取 %q：路径不存在或不可访问", filepath.Base(name))
	}
	base := filepath.Base(name)
	if info.IsDir() {
		if layerKind == "oci" {
			return Document{}, fmt.Errorf("--layer-kind=oci 只接受归档输入（%q 是目录）", base)
		}
		raws, err := loadDir(name, maxEntryBytes)
		if err != nil {
			return Document{}, err
		}
		return Document{Kind: KindDir, Entries: raws, Container: Container{Kind: KindDir}}, nil
	}
	if !info.Mode().IsRegular() {
		return Document{}, fmt.Errorf("%q 既不是目录也不是普通文件", base)
	}

	f, err := os.Open(name)
	if err != nil {
		return Document{}, fmt.Errorf("无法读取 %q：路径不存在或不可访问", base)
	}
	defer f.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return Document{}, fmt.Errorf("无法读取 %q：%v", base, err)
	}
	head = head[:n]

	switch sniff(head) {
	case KindTarGz:
		// 嗅探已读掉前 512 字节：解析成员头前必须拨回 0。
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return Document{}, fmt.Errorf("无法读取 %q：%v", base, err)
		}
		c, err := readGzipContainer(f)
		if err != nil {
			return Document{}, fmt.Errorf("%q：%v", base, err)
		}
		c.Kind = KindTarGz
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return Document{}, fmt.Errorf("无法读取 %q：%v", base, err)
		}
		zr, err := gzip.NewReader(f)
		if err != nil {
			return Document{}, fmt.Errorf("%q：gzip 头无法解析（%v）", base, err)
		}
		defer zr.Close()
		raws, err := loadTar(bufio.NewReader(zr), maxEntryBytes, base, KindTarGz)
		if err != nil {
			return Document{}, err
		}
		return Document{Kind: ociKind(KindTarGz, layerKind), Entries: raws, Container: c}, nil
	case KindZip:
		if layerKind == "oci" {
			return Document{}, fmt.Errorf("%q 是 zip，不能按 OCI layer tar 装载", base)
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return Document{}, fmt.Errorf("无法读取 %q：%v", base, err)
		}
		raws, c, err := loadZip(f, info.Size(), maxEntryBytes, base)
		if err != nil {
			return Document{}, err
		}
		c.Kind = KindZip
		return Document{Kind: KindZip, Entries: raws, Container: c}, nil
	case KindTar:
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return Document{}, fmt.Errorf("无法读取 %q：%v", base, err)
		}
		raws, err := loadTar(bufio.NewReader(f), maxEntryBytes, base, KindTar)
		if err != nil {
			return Document{}, err
		}
		return Document{Kind: ociKind(KindTar, layerKind), Entries: raws,
			Container: Container{Kind: KindTar}}, nil
	default:
		// 魔数不认识：仍尝试按 tar 解析（v7 tar 没有 ustar 魔数）。
		// 解析失败**或一个条目都没读出来** → 显式报"不支持的格式"，
		// 绝不把"看不懂"当成"空树 / 一致"（静默通过的经典漏洞）。
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return Document{}, fmt.Errorf("无法读取 %q：%v", base, err)
		}
		raws, err := loadTar(bufio.NewReader(f), maxEntryBytes, base, KindTar)
		if err != nil || len(raws) == 0 {
			return Document{}, fmt.Errorf("%q：无法识别容器格式（支持目录 / tar / tar.gz / zip / whl / jar）", base)
		}
		return Document{Kind: ociKind(KindTar, layerKind), Entries: raws,
			Container: Container{Kind: KindTar}}, nil
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
			out = append(out, Raw{Path: relSlash, Kind: EntryDir, Index: -1, Source: KindDir})
		case d.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return fmt.Errorf("条目 %q：无法读取软链目标（%v）", relSlash, err)
			}
			data := []byte(filepath.ToSlash(target))
			out = append(out, Raw{Path: relSlash, Kind: EntrySymlink, Size: int64(len(data)), Data: data,
				Index: -1, Source: KindDir})
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
			out = append(out, Raw{Path: relSlash, Kind: EntryFile, Size: int64(len(data)), Data: data,
				Index: -1, Source: KindDir})
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
