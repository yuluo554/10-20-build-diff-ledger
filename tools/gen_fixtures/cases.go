// 样张矩阵与容器渲染（plan/04 §二）。
//
// 每个 case 由「逻辑树 + 两侧注入参数」描述，容器字节由本文件渲染。
// 期望值写进 expect.json 的 expect 段；它必须**可由 inputs 独立复算**
// （plan/04 §四），因此只写注入参数与期望，不写任何 observed_* 字段。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// fileKind 是逻辑条目形态。
type fileKind int

const (
	kindFile fileKind = iota
	kindDir
	kindSymlink
	kindELF // 内容由 sideSpec.ELF 现场合成
)

// fileSpec 是逻辑树里的一个条目。
type fileSpec struct {
	Path     string
	Kind     fileKind
	Data     []byte
	Linkname string
}

// tarKind 是渲染到 tar 时的形态。
type tarKind int

const (
	tarKindFile tarKind = iota
	tarKindDir
	tarKindSymlink
)

// tarEntry 是渲染到容器的一个条目。
type tarEntry struct {
	Name     string
	Kind     tarKind
	Data     []byte
	Linkname string
	Mode     int64
	UID      int64
	GID      int64
	MTime    int64
	Uname    string
	Gname    string
	// PaxRecords 非空时，在该条目之前写一个 PAX 扩展头（'x'）承载这些记录。
	// c14 用它注入"未知 keyword"（raw_header_sha256 的兜底反例）。
	PaxRecords []string
}

// sideSpec 是一侧的注入参数。所有可差异化的字段都在这里，
// 写盘时必须逐字段显式赋值（不依赖任何库的默认值）。
type sideSpec struct {
	Prefix    string // 条目根前缀（c04 注入 T4）
	MTime     int64
	UID       int64
	GID       int64
	Uname     string
	Gname     string
	FileMode  int64
	DirMode   int64
	Reverse   bool              // 存储顺序反转（c05 注入 T5）
	Overrides map[string][]byte // 按逻辑路径覆盖内容（c11/c12 注入真改）
	// PAX 是按逻辑路径注入的 PAX 扩展头记录（c14 注入未知 keyword）。
	PAX map[string][]string
	ELF elfSpec

	GzipXFL   byte
	GzipOS    byte
	GzipMTime int64

	Zip zipParams
}

func baseSide() sideSpec {
	return sideSpec{
		MTime:    TimeTable[1], // 1700000000
		UID:      1000,
		GID:      1000,
		Uname:    "builder",
		Gname:    "builder",
		FileMode: 0o644,
		DirMode:  0o755,
		ELF: elfSpec{
			Comment:  "GCC: (GNU) 13.2.0",
			Producer: "GNU C17 13.2.0 -O2 -fno-ident",
			CompDir:  "/build/synthpkg",
			BuildID:  filler("build-id", 20),
		},
		GzipXFL:   2, // 最大压缩
		GzipOS:    3, // Unix
		GzipMTime: TimeTable[0],
		Zip: zipParams{
			CreateSystem:  3, // Unix
			CreateVersion: 20,
			ExtractVer:    20,
		},
	}
}

// expectBody 是 expect.json 的期望段。
// 只有"可由 inputs 复算"的量：分类正例集合、允许出现的类别、
// 最小原因集（null = 无解）、残留字节数、退出码。
type expectBody struct {
	CountsPositive  []string `json:"counts_positive"`
	CountsAllowed   []string `json:"counts_allowed"`
	MinimalCauseSet []string `json:"minimal_cause_set"` // nil → JSON null
	ResidueBytes    int64    `json:"residue_bytes"`
	ExitCode        int      `json:"exit_code"`
}

// caseDef 是一个样张的完整定义。
type caseDef struct {
	Name     string
	Title    string
	Format   string // "tar" | "tar.gz" | "zip"
	Files    []fileSpec
	Left     sideSpec
	Right    sideSpec
	Injected []string
	Manual   string // 该 case 特有的手工复现命令（追加在通用命令之后）
	Expect   expectBody
}

// artifactName 是两侧归档的文件名（两侧同名，便于报告只比 basename）。
func (c caseDef) artifactName() string {
	switch c.Format {
	case "tar":
		return synthName + "-" + synthVersion + ".tar"
	case "tar.gz":
		return synthName + "-" + synthVersion + ".tar.gz"
	case "zip":
		return synthName + "-" + synthVersion + ".zip"
	}
	return ""
}

// allCounts 是分类法的全部编号（与 model.RuleIDs 同源；生成器不 import internal，
// 因此在这里按 T1..T12 枚举，并有 test 断言与 model.RuleIDs 一致）。
func allCounts() []string {
	return []string{"T1", "T2", "T3", "T4", "T5", "T6", "T7", "T8", "T9", "T10", "T11", "T12"}
}

// buildEntries 把逻辑树 + 注入参数渲染成容器条目。
//
// 顺序：目录（按路径字典序）在前、文件（按 Files 列表顺序）在后；
// Reverse 时整体反转（T5 的注入点）。
func buildEntries(files []fileSpec, s sideSpec) ([]tarEntry, error) {
	dirs := map[string]bool{}
	for _, f := range files {
		dir := ""
		for _, seg := range splitPath(f.Path) {
			if dir == "" {
				dir = seg
			} else {
				dir += "/" + seg
			}
			if dir != f.Path {
				dirs[dir] = true
			}
		}
	}
	dirList := make([]string, 0, len(dirs))
	for d := range dirs {
		dirList = append(dirList, d)
	}
	sort.Strings(dirList)

	var out []tarEntry
	for _, d := range dirList {
		out = append(out, tarEntry{
			Name:  s.Prefix + d + "/",
			Kind:  tarKindDir,
			Mode:  s.DirMode,
			UID:   s.UID,
			GID:   s.GID,
			MTime: s.MTime,
			Uname: s.Uname,
			Gname: s.Gname,
		})
	}
	for _, f := range files {
		data := f.Data
		if f.Kind == kindELF {
			elf, err := buildELF(s.ELF)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", f.Path, err)
			}
			data = elf
		}
		if o, ok := s.Overrides[f.Path]; ok {
			data = o
		}
		te := tarEntry{
			Name:  s.Prefix + f.Path,
			Mode:  s.FileMode,
			UID:   s.UID,
			GID:   s.GID,
			MTime: s.MTime,
			Uname: s.Uname,
			Gname: s.Gname,
		}
		if recs, ok := s.PAX[f.Path]; ok {
			te.PaxRecords = recs
		}
		switch f.Kind {
		case kindDir:
			te.Kind = tarKindDir
			te.Name += "/"
		case kindSymlink:
			te.Kind = tarKindSymlink
			te.Linkname = f.Linkname
		default:
			te.Kind = tarKindFile
			te.Data = data
		}
		out = append(out, te)
	}
	if s.Reverse {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out, nil
}

// render 把条目渲染成容器字节。
func (c caseDef) render(s sideSpec) ([]byte, error) {
	entries, err := buildEntries(c.Files, s)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	switch c.Format {
	case "tar":
		if err := writeTar(&buf, entries); err != nil {
			return nil, err
		}
	case "tar.gz":
		p := gzipParams{XFL: s.GzipXFL, OS: s.GzipOS, MTime: s.GzipMTime, NoName: true}
		if err := writeTarGz(&buf, entries, p); err != nil {
			return nil, err
		}
	case "zip":
		zEntries := make([]zipEntry, 0, len(entries))
		for _, e := range entries {
			t, d, err := dosDateTime(s.MTime)
			if err != nil {
				return nil, err
			}
			ze := zipEntry{
				Name:    e.Name,
				Data:    e.Data,
				IsDir:   e.Kind == tarKindDir,
				Mode:    uint32(e.Mode),
				DOSTime: t,
				DOSDate: d,
			}
			if e.Kind == tarKindDir && ze.Mode == 0 {
				ze.Mode = 0o755
			}
			zEntries = append(zEntries, ze)
		}
		if err := writeZip(&buf, zEntries, s.Zip); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("未知容器格式 %q", c.Format)
	}
	return buf.Bytes(), nil
}

func splitPath(p string) []string {
	var out []string
	cur := ""
	for _, r := range p {
		if r == '/' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// sha256Hex 是 golden/manifest 里用的摘要形态（小写 hex）。
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
