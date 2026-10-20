package facts

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/artifact"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
)

// fixture 返回仓库内样张路径（测试工作目录是包目录）。
func fixture(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", "data", "fixtures"}, parts...)...)
}

func loadDoc(t *testing.T, parts ...string) artifact.Document {
	t.Helper()
	doc, err := artifact.LoadDocument(fixture(parts...), "", artifact.DefaultMaxEntryBytes)
	if err != nil {
		t.Fatalf("装载 %v 失败: %v", parts, err)
	}
	return doc
}

func valueOf(fs []model.Fact, path, key string) (string, bool) {
	for _, f := range fs {
		if f.Path == path && f.Key == key {
			return f.Value, true
		}
	}
	return "", false
}

func mustValue(t *testing.T, fs []model.Fact, path, key string) string {
	t.Helper()
	v, ok := valueOf(fs, path, key)
	if !ok {
		t.Fatalf("缺事实 (path=%q, key=%q)", path, key)
	}
	return v
}

// assertEntry 在指定样张里断言一个条目的 (path,key) 值。
func assertEntry(t *testing.T, want string, parts []string, path, key string) {
	t.Helper()
	doc := loadDoc(t, parts...)
	fs, err := ExtractDocument(doc)
	if err != nil {
		t.Fatalf("抽取失败: %v", err)
	}
	got := mustValue(t, fs, path, key)
	if got != want {
		t.Fatalf("%s @ %s = %q，期望 %q", key, path, got, want)
	}
}

// assertContainer 断言容器级事实（path = ""）。
func assertContainer(t *testing.T, want string, parts []string, key string) {
	t.Helper()
	doc := loadDoc(t, parts...)
	fs, err := ExtractDocument(doc)
	if err != nil {
		t.Fatalf("抽取失败: %v", err)
	}
	got := mustValue(t, fs, "", key)
	if got != want {
		t.Fatalf("%s = %q，期望 %q", key, got, want)
	}
}

func tarLeft() []string { return []string{"c01-mtime", "pair", "left", "synthpkg-0.1.0.tar"} }
func zipLeft() []string { return []string{"c07-archive-hdr", "pair", "left", "synthpkg-0.1.0.zip"} }
func gzLeft() []string  { return []string{"c06-gzip-opts", "pair", "left", "synthpkg-0.1.0.tar.gz"} }
func elfLeft() []string { return []string{"c08-toolchain", "pair", "left", "synthpkg-0.1.0.tar"} }

// assertELFFact 断言 c08 左侧 ELF 条目的一条事实。
func assertELFFact(t *testing.T, key, want string) {
	t.Helper()
	assertEntry(t, want, elfLeft(), "synthpkg/bin/synthpkg", key)
}

// keyTests 是"每个 key 一个表驱动单测"的登记表（plan/03 §三）。
// 新增 key 而漏写这里 → TestEveryKeyHasTest 红。
var keyTests = map[string]func(t *testing.T){
	"tar.mtime":    func(t *testing.T) { assertEntry(t, "1700000000", tarLeft(), "synthpkg/METADATA", "tar.mtime") },
	"tar.uid":      func(t *testing.T) { assertEntry(t, "1000", tarLeft(), "synthpkg/METADATA", "tar.uid") },
	"tar.gid":      func(t *testing.T) { assertEntry(t, "1000", tarLeft(), "synthpkg/METADATA", "tar.gid") },
	"tar.uname":    func(t *testing.T) { assertEntry(t, "builder", tarLeft(), "synthpkg/METADATA", "tar.uname") },
	"tar.gname":    func(t *testing.T) { assertEntry(t, "builder", tarLeft(), "synthpkg/METADATA", "tar.gname") },
	"tar.mode":     func(t *testing.T) { assertEntry(t, "420", tarLeft(), "synthpkg/METADATA", "tar.mode") },    // 0644
	"tar.typeflag": func(t *testing.T) { assertEntry(t, "48", tarLeft(), "synthpkg/METADATA", "tar.typeflag") }, // '0'
	"tar.size":     func(t *testing.T) { assertEntry(t, "98", tarLeft(), "synthpkg/METADATA", "tar.size") },     // METADATA 的字节数

	"zip.mtime":         func(t *testing.T) { assertEntry(t, "20231114221320", zipLeft(), "synthpkg/METADATA", "zip.mtime") },
	"zip.external_attr": func(t *testing.T) { assertEntry(t, "27525120", zipLeft(), "synthpkg/METADATA", "zip.external_attr") }, // 0644<<16
	"zip.create_system": func(t *testing.T) { assertContainer(t, "3", zipLeft(), "zip.create_system") },
	"zip.method":        func(t *testing.T) { assertEntry(t, "0", zipLeft(), "synthpkg/METADATA", "zip.method") }, // Store

	"gzip.mtime": func(t *testing.T) { assertContainer(t, "0", gzLeft(), "gzip.mtime") },
	"gzip.xfl":   func(t *testing.T) { assertContainer(t, "2", gzLeft(), "gzip.xfl") },
	"gzip.os":    func(t *testing.T) { assertContainer(t, "3", gzLeft(), "gzip.os") },

	"elf.build_id": func(t *testing.T) {
		// 与生成器的 filler("build-id", 20) 同源：sha256("build-id|0") 的前 20 字节。
		sum := sha256.Sum256([]byte("build-id|0"))
		assertELFFact(t, "elf.build_id", hex.EncodeToString(sum[:20]))
	},
	"elf.comment":  func(t *testing.T) { assertELFFact(t, "elf.comment", "GCC: (GNU) 13.2.0") },
	"elf.comp_dir": func(t *testing.T) { assertELFFact(t, "elf.comp_dir", "/build/synthpkg") },
	"elf.producer": func(t *testing.T) { assertELFFact(t, "elf.producer", "GNU C17 13.2.0 -O2 -fno-ident") },

	"pe.timestamp":    peKeyTest("pe.timestamp", "1700000000"),
	"pe.checksum":     peKeyTest("pe.checksum", "3735928559"), // 0xdeadbeef
	"pe.rich.present": peKeyTest("pe.rich.present", "true"),
	"pe.rich.digest":  peKeyTest("pe.rich.digest", peRichWant()),

	"macho.uuid":  machoKeyTest("macho.uuid", "00112233445566778899aabbccddeeff"),
	"macho.minos": machoKeyTest("macho.minos", "13.2.0"),
	"macho.sdk":   machoKeyTest("macho.sdk", "14.0.0"),
	"macho.tools": machoKeyTest("macho.tools", "clang:15.0.0,ld:711.0.0"),

	"raw_header_sha256": func(t *testing.T) {
		doc := loadDoc(t, tarLeft()...)
		fs, err := ExtractDocument(doc)
		if err != nil {
			t.Fatal(err)
		}
		v := mustValue(t, fs, "synthpkg/METADATA", "raw_header_sha256")
		if len(v) != 64 {
			t.Fatalf("raw_header_sha256 不是 64 位 hex：%q", v)
		}
	},
}

// ELF 节的 offset/size 由生成器的布局确定（tools/gen_fixtures/write_elf.go 逐字段计算）。
func init() {
	for _, c := range []struct{ name, off, size string }{
		{".text", "64", "8"},
		{".comment", "72", "18"},
		{".note.gnu.build-id", "92", "36"},
		{".debug_abbrev", "128", "10"},
		{".debug_info", "138", "58"},
	} {
		c := c
		keyTests[SectionKey(c.name, "offset")] = func(t *testing.T) {
			assertELFFact(t, SectionKey(c.name, "offset"), c.off)
		}
		keyTests[SectionKey(c.name, "size")] = func(t *testing.T) {
			assertELFFact(t, SectionKey(c.name, "size"), c.size)
		}
	}
	// .sig 只存在于 c13（左侧注入签名段）。
	keyTests[SectionKey(".sig", "offset")] = func(t *testing.T) {
		doc := loadDoc(t, "c13-signature", "pair", "left", "synthpkg-0.1.0.tar")
		fs, err := ExtractDocument(doc)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := valueOf(fs, "synthpkg/bin/synthpkg", SectionKey(".sig", "offset")); !ok {
			t.Fatal("c13 左侧应含 elf.section..sig.offset")
		}
	}
	keyTests[SectionKey(".sig", "size")] = func(t *testing.T) {
		doc := loadDoc(t, "c13-signature", "pair", "left", "synthpkg-0.1.0.tar")
		fs, err := ExtractDocument(doc)
		if err != nil {
			t.Fatal(err)
		}
		if got := mustValue(t, fs, "synthpkg/bin/synthpkg", SectionKey(".sig", "size")); got != "64" {
			t.Fatalf(".sig size = %q，期望 64", got)
		}
	}
}

// TestEveryKeyHasTest 是"key 集合由代码常量驱动"的守门：
// 新增 key 而漏写测试 → 红（HANDOFF-M2 §三.1 的硬要求）。
func TestEveryKeyHasTest(t *testing.T) {
	for _, k := range Keys() {
		if _, ok := keyTests[k]; !ok {
			t.Errorf("key %q 没有单测（keyTests 缺项）", k)
		}
	}
}

// TestNoOrphanKeyTests 反向守门：测试表里不许有已不在 Keys() 里的 key。
func TestNoOrphanKeyTests(t *testing.T) {
	live := map[string]bool{}
	for _, k := range Keys() {
		live[k] = true
	}
	for k := range keyTests {
		if !live[k] {
			t.Errorf("keyTests 里的 %q 不在 Keys() 中", k)
		}
	}
}

// TestKeyListHasNoDuplicates 防"同一 key 登记两次 → 漏测另一个 key 也看不出来"。
func TestKeyListHasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range Keys() {
		if seen[k] {
			t.Errorf("Keys() 重复登记 %q", k)
		}
		seen[k] = true
	}
}

// TestEachKeyHasTest 跑表驱动用例（一个 key 一个子测试）。
func TestEachKeyHasTest(t *testing.T) {
	for k, fn := range keyTests {
		k, fn := k, fn
		t.Run(k, fn)
	}
}

// TestRawHeaderUnknownPAXIsHandRecomputed 是 c14 的核心断言：
// 未知 PAX keyword 的条目，raw_header_sha256 必须等于**手工按字节重算**的值。
//
// 重算完全不调用本包或 artifact 的代码：直接扫描 tar 块——
// 找 typeflag='x' 的扩展头 → 其 size 后的数据 + 填充 → 下一条目的头部[345:512]。
func TestRawHeaderUnknownPAXIsHandRecomputed(t *testing.T) {
	p := fixture("c14-pax-unknown", "pair", "left", "synthpkg-0.1.0.tar")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	raw, path, err := handRecomputePAXRawHeader(data)
	if err != nil {
		t.Fatalf("手工重算失败: %v", err)
	}
	if path != "synthpkg/METADATA" {
		t.Fatalf("手工重算定位到 %q，期望 synthpkg/METADATA", path)
	}
	sum := sha256.Sum256(raw)
	want := hex.EncodeToString(sum[:])

	doc := loadDoc(t, "c14-pax-unknown", "pair", "left", "synthpkg-0.1.0.tar")
	fs, err := ExtractDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	got := mustValue(t, fs, path, "raw_header_sha256")
	if got != want {
		t.Fatalf("raw_header_sha256 = %q，手工重算 = %q", got, want)
	}
	// 未知 keyword 本身必须在抽出来的事实里可指认（原文在 PAX 块中）。
	if !bytes.Contains(data, []byte("SCHILY.xattr.user.marker=fixture-unknown")) {
		t.Fatal("样张里找不到注入的未知 PAX keyword")
	}
}

// handRecomputePAXRawHeader 只按 POSIX tar 规范扫描字节，不借用被测代码。
func handRecomputePAXRawHeader(data []byte) (raw []byte, path string, err error) {
	const blk = 512
	oct := func(b []byte) int {
		n := 0
		for _, c := range b {
			if c == 0 || c == ' ' {
				break
			}
			n = n*8 + int(c-'0')
		}
		return n
	}
	for pos := 0; pos+blk <= len(data); {
		head := data[pos : pos+blk]
		if head[156] != 'x' {
			pos += blk
			continue
		}
		size := oct(head[124:136])
		extEnd := pos + blk + size
		if pad := (blk - size%blk) % blk; pad > 0 {
			extEnd += pad
		}
		if extEnd+blk > len(data) {
			return nil, "", os.ErrInvalid
		}
		entry := data[extEnd : extEnd+blk]
		name := entry[0:100]
		if i := bytes.IndexByte(name, 0); i >= 0 {
			name = name[:i]
		}
		// 扩展头原文（块 + 数据 + 填充）+ 条目头保留区 [345:512]
		raw = append(append([]byte(nil), data[pos:extEnd]...), entry[345:blk]...)
		return raw, string(name), nil
	}
	return nil, "", os.ErrNotExist
}

// TestRawHeaderDiffersOnlyOnPAXEntry 断言 c14 的差异只落在注入了 PAX 的条目上。
func TestRawHeaderDiffersOnlyOnPAXEntry(t *testing.T) {
	left := loadDoc(t, "c14-pax-unknown", "pair", "left", "synthpkg-0.1.0.tar")
	right := loadDoc(t, "c14-pax-unknown", "pair", "right", "synthpkg-0.1.0.tar")
	lf, err := ExtractDocument(left)
	if err != nil {
		t.Fatal(err)
	}
	rf, err := ExtractDocument(right)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range left.Entries {
		if raw.Kind == artifact.EntryDir {
			continue
		}
		l := mustValue(t, lf, raw.Path, "raw_header_sha256")
		r := mustValue(t, rf, raw.Path, "raw_header_sha256")
		if raw.Path == "synthpkg/METADATA" {
			if l == r {
				t.Fatal("注入 PAX 的条目 raw_header 必须不同")
			}
		} else if l != r {
			t.Fatalf("条目 %q 的 raw_header 不应变化", raw.Path)
		}
	}
}

// TestExtractDeterministic 断言同一输入双跑逐字段一致。
func TestExtractDeterministic(t *testing.T) {
	d1 := loadDoc(t, "c14-pax-unknown", "pair", "left", "synthpkg-0.1.0.tar")
	d2 := loadDoc(t, "c14-pax-unknown", "pair", "left", "synthpkg-0.1.0.tar")
	a, err := ExtractDocument(d1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ExtractDocument(d2)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) {
		t.Fatalf("两次抽取事实数不同：%d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("第 %d 条事实不一致：%+v vs %+v", i, a[i], b[i])
		}
	}
}

// TestDirEntriesHaveNoFacts 钉住 plan/03 §二.5：目录条目不参与 facts。
func TestDirEntriesHaveNoFacts(t *testing.T) {
	doc := loadDoc(t, "align-golden", "rename-add", "pair", "left")
	for _, raw := range doc.Entries {
		if raw.Kind != artifact.EntryDir {
			continue
		}
		fs, err := Extract(raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(fs) != 0 {
			t.Fatalf("目录条目 %q 不应产生 facts，实得 %d 条", raw.Path, len(fs))
		}
	}
}

// TestMalformedELFIsExplicit 断言"看得懂魔数但读不了"必须报错（exit 1 的来源）。
func TestMalformedELFIsExplicit(t *testing.T) {
	raw := artifact.Raw{Path: "x", Kind: artifact.EntryFile, Source: artifact.KindDir,
		Data: []byte("\x7fELF\x02\x01\x01")}
	if _, err := Extract(raw); err == nil {
		t.Fatal("截断的 ELF 必须报错，不得静默返回空事实")
	}
}

// ---------- PE / Mach-O 抽样样张（测试内合成，不入仓） ----------

func buildTestPE() []byte {
	b := make([]byte, 0x200)
	copy(b[0:2], "MZ")
	le := binary.LittleEndian
	le.PutUint32(b[0x3C:], 0x80) // e_lfanew
	copy(b[0x40:], "DanS")       // Rich 头起点
	copy(b[0x60:], "Rich")
	le.PutUint32(b[0x64:], 0xa5a5a5a5) // XOR key
	copy(b[0x80:], "PE\x00\x00")
	coff := 0x84
	le.PutUint32(b[coff+4:], 1700000000) // TimeDateStamp
	le.PutUint16(b[coff+16:], 224)       // SizeOfOptionalHeader（PE32）
	opt := coff + 20
	le.PutUint16(b[opt:], 0x10b)
	le.PutUint32(b[opt+64:], 0xdeadbeef)
	return b
}

func peRichWant() string {
	sum := sha256.Sum256(buildTestPE()[0x40:0x68]) // DanS .. Rich + key
	return hex.EncodeToString(sum[:])
}

func peFacts(t *testing.T) []model.Fact {
	t.Helper()
	fs, err := Extract(artifact.Raw{Path: "bin/app.exe", Kind: artifact.EntryFile,
		Source: artifact.KindDir, Data: buildTestPE()})
	if err != nil {
		t.Fatalf("PE 抽取失败: %v", err)
	}
	return fs
}

func peKeyTest(key, want string) func(t *testing.T) {
	return func(t *testing.T) {
		if got := mustValue(t, peFacts(t), "bin/app.exe", key); got != want {
			t.Fatalf("%s = %q，期望 %q", key, got, want)
		}
	}
}

func buildTestMachO() []byte {
	le := binary.LittleEndian
	uuid := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	ncmds := 2
	cmds := 24 + 40
	b := make([]byte, 32+cmds)
	le.PutUint32(b[0:], 0xfeedfacf) // MH_MAGIC_64（小端文件里字节是 cf fa ed fe）
	le.PutUint32(b[4:], 0x01000007) // CPU_TYPE_X86_64
	le.PutUint32(b[8:], 3)
	le.PutUint32(b[12:], 2) // MH_EXECUTE
	le.PutUint32(b[16:], uint32(ncmds))
	le.PutUint32(b[20:], uint32(cmds))
	// LC_UUID
	pos := 32
	le.PutUint32(b[pos:], lcUUID)
	le.PutUint32(b[pos+4:], 24)
	copy(b[pos+8:pos+24], uuid)
	// LC_BUILD_VERSION
	pos += 24
	le.PutUint32(b[pos:], lcBuildVersion)
	le.PutUint32(b[pos+4:], 40)
	le.PutUint32(b[pos+8:], 1)           // platform macOS
	le.PutUint32(b[pos+12:], 0x000d0200) // minos 13.2.0
	le.PutUint32(b[pos+16:], 0x000e0000) // sdk 14.0.0
	le.PutUint32(b[pos+20:], 2)          // ntools
	le.PutUint32(b[pos+24:], 1)          // clang
	le.PutUint32(b[pos+28:], 0x000f0000) // 15.0.0
	le.PutUint32(b[pos+32:], 3)          // ld
	le.PutUint32(b[pos+36:], 0x02c70000) // 711.0.0
	return b
}

func machoFacts(t *testing.T) []model.Fact {
	t.Helper()
	fs, err := Extract(artifact.Raw{Path: "bin/app", Kind: artifact.EntryFile,
		Source: artifact.KindDir, Data: buildTestMachO()})
	if err != nil {
		t.Fatalf("Mach-O 抽取失败: %v", err)
	}
	return fs
}

func machoKeyTest(key, want string) func(t *testing.T) {
	return func(t *testing.T) {
		if got := mustValue(t, machoFacts(t), "bin/app", key); got != want {
			t.Fatalf("%s = %q，期望 %q", key, got, want)
		}
	}
}
