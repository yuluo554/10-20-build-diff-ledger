package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"debug/dwarf"
	"debug/elf"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCaseMatrixIs14AndCoversT1ToT12 钉住矩阵形态（plan/04 §二）：
// 14 组；T1–T12 每一类都至少被一组注入（"不允许有类别没有样张"）。
func TestCaseMatrixIs14AndCoversT1ToT12(t *testing.T) {
	cases := caseDefs()
	if len(cases) != 14 {
		t.Fatalf("样张数 = %d，期望 14", len(cases))
	}
	covered := map[string]bool{}
	for _, c := range cases {
		for _, r := range c.Injected {
			covered[r] = true
		}
	}
	for _, r := range allCounts() {
		if !covered[r] {
			t.Errorf("规则 %s 没有任何样张注入（题面硬纪律：不允许有类别没有样张）", r)
		}
	}
	// 名字唯一
	seen := map[string]bool{}
	for _, c := range cases {
		if seen[c.Name] {
			t.Errorf("重复 case 名 %q", c.Name)
		}
		seen[c.Name] = true
	}
}

// TestOnlyWritesNoManifest 是 K11 的核心断言：
// --only 只写该 case 的目录，绝不写全集清单（manifest.json）。
func TestOnlyWritesNoManifest(t *testing.T) {
	fresh := t.TempDir()
	if err := run(io.Discard, fresh, "c01-mtime", false); err != nil {
		t.Fatalf("--only 运行失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fresh, "manifest.json")); err == nil {
		t.Fatal("--only 写了 manifest.json（K11 违规：部分运行不得写全集清单）")
	}
	if _, err := os.Stat(filepath.Join(fresh, "c01-mtime", "expect.json")); err != nil {
		t.Fatalf("--only 没有写出目标 case: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fresh, "c02-owner")); err == nil {
		t.Fatal("--only 写出了非目标 case")
	}

	// 在已有全量树上跑 --only：manifest.json 必须逐字节不变
	full := t.TempDir()
	if err := run(io.Discard, full, "", false); err != nil {
		t.Fatalf("全量运行失败: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(full, "manifest.json"))
	if err != nil {
		t.Fatalf("全量运行没有写 manifest.json: %v", err)
	}
	if err := run(io.Discard, full, "c02-owner", false); err != nil {
		t.Fatalf("--only 运行失败: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(full, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest.json 消失了: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("--only 改写了 manifest.json（K11 违规）")
	}
}

// TestFullRunWritesManifest14 断言全量运行写出的清单恰好 14 组。
func TestFullRunWritesManifest14(t *testing.T) {
	dir := t.TempDir()
	if err := run(io.Discard, dir, "", false); err != nil {
		t.Fatalf("全量运行失败: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("缺 manifest.json: %v", err)
	}
	if got := strings.Count(string(b), `"name": "c`); got != 14 {
		t.Fatalf("manifest 里 case 数 = %d，期望 14\n%s", got, b)
	}
}

// TestCheckIsGreen 是 DoD 的重建判据：与入仓树逐字节一致。
// 它同时是本测试包的**阳性对照**——若生成逻辑漂了，这条会先红。
func TestCheckIsGreen(t *testing.T) {
	var out bytes.Buffer
	if err := run(&out, filepath.Join("..", "..", "data", "fixtures"), "", true); err != nil {
		t.Fatalf("--check 失败: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "14 cases, 0 diff") {
		t.Fatalf("--check 输出不含 '14 cases, 0 diff'：%s", out.String())
	}
}

// TestCheckIsFalsifiable 防"恒绿假门"：篡改一个字节后必须报 diff 且退出非零。
func TestCheckIsFalsifiable(t *testing.T) {
	dir := t.TempDir()
	if err := run(io.Discard, dir, "", false); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "c01-mtime", "pair", "left", "synthpkg-0.1.0.tar")
	orig, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	bad := append([]byte(nil), orig...)
	bad[len(bad)/2] ^= 0xff
	if err := os.WriteFile(target, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run(&out, dir, "", true); err == nil {
		t.Fatalf("篡改后 --check 仍通过（假门！）\n%s", out.String())
	}
	if !strings.Contains(out.String(), "1 diff") {
		t.Fatalf("--check 应报 1 diff，实得：%s", out.String())
	}
}

// TestDOSTimeKnownValues 钉住手写的日历换算（不引 time 包的代价：必须自查）。
func TestDOSTimeKnownValues(t *testing.T) {
	cases := []struct {
		unix int64
		time uint16
		date uint16
	}{
		{0, 0x0000, 0x0021},          // 1980-01-01 00:00:00
		{1700000000, 0xB1AA, 0x576E}, // 2023-11-14 22:13:20 UTC
		{1700003600, 0xB9AA, 0x576E}, // 2023-11-14 23:13:20 UTC
	}
	for _, c := range cases {
		gotT, gotD, err := dosDateTime(c.unix)
		if err != nil {
			t.Fatalf("dosDateTime(%d): %v", c.unix, err)
		}
		if gotT != c.time || gotD != c.date {
			t.Errorf("dosDateTime(%d) = (0x%04X, 0x%04X)，期望 (0x%04X, 0x%04X)",
				c.unix, gotT, gotD, c.time, c.date)
		}
	}
	if _, _, err := dosDateTime(123); err == nil {
		t.Fatal("白名单外的时间戳必须报错")
	}
}

// TestContainersReadableByStdlib 用标准库读回生成器写出的字节：
// tar / zip 的头部字段必须与注入参数完全一致（这是 fixtures 的"出厂检验"）。
func TestContainersReadableByStdlib(t *testing.T) {
	dir := t.TempDir()
	if err := run(io.Discard, dir, "", false); err != nil {
		t.Fatal(err)
	}

	// tar：mtime / uid / gid / uname / gname / mode
	f, err := os.Open(filepath.Join(dir, "c01-mtime", "pair", "left", "synthpkg-0.1.0.tar"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tr := tar.NewReader(f)
	h, err := tr.Next()
	if err != nil {
		t.Fatalf("tar 头读取失败: %v", err)
	}
	if h.ModTime.Unix() != 1700000000 {
		t.Errorf("tar mtime = %d，期望 1700000000", h.ModTime.Unix())
	}
	if h.Uid != 1000 || h.Gid != 1000 || h.Uname != "builder" || h.Gname != "builder" {
		t.Errorf("tar 属主字段与注入不符: uid=%d gid=%d uname=%q gname=%q", h.Uid, h.Gid, h.Uname, h.Gname)
	}

	// tar.gz：gzip 成员头 xfl/os（c06 两侧不同）
	for _, side := range []struct {
		dir     string
		wantXFL byte
		wantOS  byte
	}{{"left", 2, 3}, {"right", 4, 0}} {
		b, err := os.ReadFile(filepath.Join(dir, "c06-gzip-opts", "pair", side.dir, "synthpkg-0.1.0.tar.gz"))
		if err != nil {
			t.Fatal(err)
		}
		if b[8] != side.wantXFL || b[9] != side.wantOS {
			t.Errorf("%s 侧 gzip 头 xfl/os = %d/%d，期望 %d/%d", side.dir, b[8], b[9], side.wantXFL, side.wantOS)
		}
		zr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s 侧 gzip 解压失败: %v", side.dir, err)
		}
		if _, err := io.Copy(io.Discard, zr); err != nil {
			t.Fatalf("%s 侧 gzip 流损坏: %v", side.dir, err)
		}
		zr.Close()
	}

	// zip：create_system 在中央目录的 version-made-by 高字节（c07 两侧不同）
	for _, side := range []struct {
		dir  string
		want byte
	}{{"left", 3}, {"right", 0}} {
		zr, err := zip.OpenReader(filepath.Join(dir, "c07-archive-hdr", "pair", side.dir, "synthpkg-0.1.0.zip"))
		if err != nil {
			t.Fatalf("%s 侧 zip 打不开: %v", side.dir, err)
		}
		// FileHeader.CreatorVersion 的高字节 = create system
		got := byte(zr.File[0].CreatorVersion >> 8)
		zr.Close()
		if got != side.want {
			t.Errorf("%s 侧 zip create_system = %d，期望 %d", side.dir, got, side.want)
		}
	}
}

// TestGeneratedELFIsParseable 是 ELF 合成的"出厂检验"：
// debug/elf 能开、节名能取、.comment 能读、build-id 段在、
// debug/dwarf 能读出 DW_AT_producer / DW_AT_comp_dir（M2 的输入契约）。
func TestGeneratedELFIsParseable(t *testing.T) {
	spec := baseSide().ELF
	spec.Producer = "GNU C17 13.2.0 -O2"
	spec.CompDir = "/build/synthpkg"
	spec.Signature = filler("signature", 64)
	spec.Signature = spec.Signature[:64]

	b, err := buildELF(spec)
	if err != nil {
		t.Fatalf("buildELF 失败: %v", err)
	}
	ef, err := elf.NewFile(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("debug/elf 打不开生成的 ELF: %v", err)
	}
	defer ef.Close()

	if ef.Class != elf.ELFCLASS64 || ef.Machine != elf.EM_X86_64 {
		t.Fatalf("ELF 头不符: class=%v machine=%v", ef.Class, ef.Machine)
	}

	comment := ef.Section(".comment")
	if comment == nil {
		t.Fatal("缺 .comment 节")
	}
	cb, err := comment.Data()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(cb), spec.Comment) {
		t.Fatalf(".comment = %q，期望以 %q 开头", cb, spec.Comment)
	}

	note := ef.Section(".note.gnu.build-id")
	if note == nil {
		t.Fatal("缺 .note.gnu.build-id 节")
	}
	nb, err := note.Data()
	if err != nil {
		t.Fatal(err)
	}
	if len(nb) < 12 || !bytes.Contains(nb, []byte("GNU\x00")) {
		t.Fatalf("build-id note 形态不对: %x", nb)
	}

	d, err := ef.DWARF()
	if err != nil {
		t.Fatalf("debug/dwarf 打不开: %v", err)
	}
	r := d.Reader()
	e, err := r.Next()
	if err != nil || e == nil {
		t.Fatalf("DWARF 无 compile_unit: %v", err)
	}
	if got, _ := e.Val(dwarf.AttrProducer).(string); got != spec.Producer {
		t.Fatalf("DW_AT_producer = %q，期望 %q", got, spec.Producer)
	}
	if got, _ := e.Val(dwarf.AttrCompDir).(string); got != spec.CompDir {
		t.Fatalf("DW_AT_comp_dir = %q，期望 %q", got, spec.CompDir)
	}

	if ef.Section(".sig") == nil {
		t.Fatal("缺 .sig 节（c13 的 T10 载体）")
	}
}

// TestEmptySignatureStillValid 断言"无签名段"的 ELF 依旧可解析且确实没有 .sig。
func TestEmptySignatureStillValid(t *testing.T) {
	spec := baseSide().ELF
	b, err := buildELF(spec)
	if err != nil {
		t.Fatal(err)
	}
	ef, err := elf.NewFile(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("debug/elf 打不开: %v", err)
	}
	defer ef.Close()
	if ef.Section(".sig") != nil {
		t.Fatal("未注入签名时不应有 .sig 节")
	}
}
