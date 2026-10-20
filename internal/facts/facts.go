// Package facts 实现事实抽取（模块②，plan/03 §三）。
//
// 硬纪律：解析器未识别的头部/元数据区必须进 raw_header_sha256，
// 不得"没看懂就跳过"（data/README.md §4.3）。
//
// 取值格式（data/README.md §4.10）：
//   - 数字一律十进制字符串；布尔 "true"/"false"；二进制串 lowercase hex；
//   - 字符串含非 UTF-8 字节时写 "hex:" + hex；
//   - 容器级事实的 Path 为空串（`""` = 容器自身），例如 gzip.mtime / zip.create_system。
//
// 每个 key 必须有一个单测（TestEachKeyHasTest），key 集合由本文件的常量驱动：
// 新增 key 而漏写测试 → TestEveryKeyHasTest 红。
package facts

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/artifact"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
)

// SchemaVersion 是 `bdiff facts` JSON 的结构版本；字段或语义变更时 +1。
const SchemaVersion = 1

// FixedKeys 是键名与具体条目内容无关的全部 key（含容器级 key）。
// ELF 的节 key 由 TrackedSections 派生（elf.section.<name>.offset/.size）。
var FixedKeys = []string{
	"tar.mtime", "tar.uid", "tar.gid", "tar.uname", "tar.gname", "tar.mode", "tar.typeflag", "tar.size",
	"zip.mtime", "zip.external_attr", "zip.create_system", "zip.method",
	"gzip.mtime", "gzip.xfl", "gzip.os",
	"elf.build_id", "elf.comment", "elf.comp_dir", "elf.producer",
	"pe.timestamp", "pe.checksum", "pe.rich.present", "pe.rich.digest",
	"macho.uuid", "macho.minos", "macho.sdk", "macho.tools",
	"raw_header_sha256",
}

// TrackedSections 是抽取 offset/size 的 ELF 节（固定集合，便于 key 集合封闭）。
var TrackedSections = []string{".text", ".comment", ".note.gnu.build-id", ".debug_info", ".debug_abbrev", ".sig"}

// SectionKey 返回 `elf.section.<name>.<field>`。name 原样拼入（含前导点），
// 因此 ".text" 的 offset 键是 "elf.section..text.offset"（§4.10 已写明）。
func SectionKey(section, field string) string {
	return "elf.section." + section + "." + field
}

// Keys 返回全部受支持的 key（FixedKeys + TrackedSections 派生）。顺序固定。
func Keys() []string {
	out := append([]string(nil), FixedKeys...)
	for _, s := range TrackedSections {
		out = append(out, SectionKey(s, "offset"), SectionKey(s, "size"))
	}
	return out
}

// Extract 抽取一个条目的全部事实（不含容器级事实）。
//
// 返回的 []model.Fact 按 (path, key) 字节序排序（plan/03 §三）。
// 目录条目不参与 facts（plan/03 §二.5：无可抽取头部）。
func Extract(raw artifact.Raw) ([]model.Fact, error) {
	if raw.Kind == artifact.EntryDir {
		return nil, nil
	}
	var out []model.Fact
	add := func(key, val string) {
		out = append(out, model.Fact{Path: raw.Path, Key: key, Value: val})
	}

	if ti := raw.Tar; ti != nil {
		add("tar.mtime", strconv.FormatInt(ti.MTime, 10))
		add("tar.uid", strconv.FormatInt(ti.UID, 10))
		add("tar.gid", strconv.FormatInt(ti.GID, 10))
		add("tar.uname", encodeString(ti.Uname))
		add("tar.gname", encodeString(ti.Gname))
		add("tar.mode", strconv.FormatInt(ti.Mode, 10))
		add("tar.typeflag", strconv.Itoa(int(ti.Typeflag)))
		add("tar.size", strconv.FormatInt(ti.Size, 10))
	}
	if zi := raw.Zip; zi != nil {
		add("zip.mtime", dosDateTime(zi.ModifiedDate, zi.ModifiedTime))
		add("zip.external_attr", strconv.FormatUint(uint64(zi.ExternalAttrs), 10))
		add("zip.method", strconv.Itoa(int(zi.Method)))
	}

	// 内容格式（ELF / PE / Mach-O）：无论条目来自哪种容器都抽，
	// 因为 T8–T11 的被测对象是文件内容。
	cf, err := analyzeContent(raw.Data)
	if err != nil {
		return nil, fmt.Errorf("条目 %q：%v", raw.Path, err)
	}
	for _, kv := range cf.values {
		add(kv[0], kv[1])
	}

	header := raw.RawHeader
	if raw.Source == artifact.KindDir {
		// 裸文件（目录装载）：raw_header 描述文件自身的格式头。
		header = cf.header
	}
	sum := sha256.Sum256(header)
	add("raw_header_sha256", hex.EncodeToString(sum[:]))

	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

// ExtractContainer 抽取容器级事实（Path = ""），含 gzip 的 raw_header_sha256。
func ExtractContainer(c artifact.Container) []model.Fact {
	var out []model.Fact
	for _, f := range c.Facts {
		out = append(out, model.Fact{Path: "", Key: f.Key, Value: f.Value})
	}
	if len(c.RawHeader) > 0 {
		sum := sha256.Sum256(c.RawHeader)
		out = append(out, model.Fact{Path: "", Key: "raw_header_sha256", Value: hex.EncodeToString(sum[:])})
	}
	return out
}

// ExtractDocument 抽取整份产物：容器级事实 + 每个条目，按 (path, key) 排序。
func ExtractDocument(d artifact.Document) ([]model.Fact, error) {
	out := ExtractContainer(d.Container)
	for _, raw := range d.Entries {
		fs, err := Extract(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, fs...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

// encodeString 按 §三的字符串口径编码：非 UTF-8 → "hex:" + hex。
func encodeString(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return "hex:" + hex.EncodeToString([]byte(s))
}

// dosDateTime 把 MS-DOS 的 (date,time) 格式化成可手工核对的
// "YYYYMMDDHHMMSS"（UTC 口径、无时区换算）。零值写 "0"。
func dosDateTime(date, tm uint16) string {
	if date == 0 && tm == 0 {
		return "0"
	}
	year := int(date>>9) + 1980
	month := int((date >> 5) & 0x0f)
	day := int(date & 0x1f)
	hour := int(tm >> 11)
	min := int((tm >> 5) & 0x3f)
	sec := int(tm&0x1f) * 2
	return fmt.Sprintf("%04d%02d%02d%02d%02d%02d", year, month, day, hour, min, sec)
}

// contentFacts 是内容格式分析的结果。
type contentFacts struct {
	values [][2]string // 保持确定性顺序
	header []byte      // 文件自身的格式头（仅裸文件用）
	kind   string      // "elf" | "pe" | "macho" | ""
}

// analyzeContent 按字节魔数分派到 ELF / PE / Mach-O 分析器。
// 魔数不认识 → 空结果（不是错误：文本文件本就没有头部）。
func analyzeContent(data []byte) (contentFacts, error) {
	if len(data) >= 4 && string(data[:4]) == "\x7fELF" {
		return analyzeELF(data)
	}
	if len(data) >= 2 && data[0] == 'M' && data[1] == 'Z' {
		return analyzePE(data)
	}
	if len(data) >= 4 {
		magic := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
		switch magic {
		case 0xfeedface, 0xfeedfacf, 0xcefaedfe, 0xcffaedfe, // thin 32/64，两种字节序
			0xcafebabe, 0xcafebabf, 0xbebafeca: // fat 32/64 及其交换形态
			return analyzeMachO(data)
		}
	}
	return contentFacts{}, nil
}

// Output 是 `bdiff facts --format json` 的输出（字段序 = 键序，逐字节断言）。
type Output struct {
	SchemaVersion int          `json:"schema_version"`
	Kind          string       `json:"kind"`
	Entries       int          `json:"entries"`
	Facts         []model.Fact `json:"facts"`
}

// Build 把一个已装载的 Document 转成 CLI 输出。
func Build(d artifact.Document) (Output, error) {
	fs, err := ExtractDocument(d)
	if err != nil {
		return Output{}, err
	}
	if fs == nil {
		fs = []model.Fact{}
	}
	return Output{
		SchemaVersion: SchemaVersion,
		Kind:          string(d.Kind),
		Entries:       len(d.Entries),
		Facts:         fs,
	}, nil
}
