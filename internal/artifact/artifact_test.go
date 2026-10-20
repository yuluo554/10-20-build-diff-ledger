package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtures 返回仓库内的样张路径（测试工作目录是包目录）。
func fixture(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", "data", "fixtures"}, parts...)...)
}

// TestNormalizeRel 钉住 plan/03 §二.2 的路径规范化口径。
func TestNormalizeRel(t *testing.T) {
	ok := []struct{ in, want string }{
		{"./a/b", "a/b"},
		{"././a/b", "a/b"},
		{"a//b", "a/b"},
		{"a/./b", "a/b"},
		{"a/b/", "a/b"},
		{".", ""},
		{"a/b/c.txt", "a/b/c.txt"},
	}
	for _, c := range ok {
		got, err := NormalizeRel(c.in)
		if err != nil {
			t.Errorf("NormalizeRel(%q) 报错: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizeRel(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}

	bad := []string{"../x", "a/../../b", "/abs/path", "C:/windows", "a/.."}
	for _, in := range bad {
		if _, err := NormalizeRel(in); err == nil {
			t.Errorf("NormalizeRel(%q) 必须报错（越界/绝对路径），实际通过", in)
		}
	}
}

// TestLoadContainers 断言四种容器都能装载，且条目内容与归档一致。
func TestLoadContainers(t *testing.T) {
	cases := []struct {
		name string
		path string
		kind Kind
	}{
		{"tar", fixture("c01-mtime", "pair", "left", "synthpkg-0.1.0.tar"), KindTar},
		{"tar.gz", fixture("c06-gzip-opts", "pair", "left", "synthpkg-0.1.0.tar.gz"), KindTarGz},
		{"zip", fixture("c07-archive-hdr", "pair", "left", "synthpkg-0.1.0.zip"), KindZip},
		{"dir", fixture("align-golden", "rename-add", "pair", "left"), KindDir},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raws, kind, err := Load(c.path, "", DefaultMaxEntryBytes)
			if err != nil {
				t.Fatalf("Load 失败: %v", err)
			}
			if kind != c.kind {
				t.Errorf("kind = %q，期望 %q", kind, c.kind)
			}
			if len(raws) == 0 {
				t.Fatal("装载出 0 个条目——判据是空的")
			}
			seen := map[string]bool{}
			for _, r := range raws {
				if seen[r.Path] {
					t.Errorf("条目路径重复: %q", r.Path)
				}
				seen[r.Path] = true
				switch r.Kind {
				case EntryFile, EntryDir, EntrySymlink:
				default:
					t.Errorf("条目 %q 的形态 %q 非法", r.Path, r.Kind)
				}
				if r.Kind == EntryFile && int64(len(r.Data)) != r.Size {
					t.Errorf("条目 %q：Size=%d 与内容长度 %d 不符", r.Path, r.Size, len(r.Data))
				}
			}
		})
	}
}

// TestLoadTwiceIsIdentical 断言装载是确定性的（同输入双跑逐字段一致）。
func TestLoadTwiceIsIdentical(t *testing.T) {
	p := fixture("c13-signature", "pair", "left", "synthpkg-0.1.0.tar")
	a, _, err := Load(p, "", DefaultMaxEntryBytes)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := Load(p, "", DefaultMaxEntryBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) {
		t.Fatalf("两次装载条目数不同: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Path != b[i].Path || a[i].Kind != b[i].Kind || a[i].Size != b[i].Size ||
			string(a[i].Data) != string(b[i].Data) {
			t.Fatalf("第 %d 个条目两次装载不一致: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// TestOCILayerKind 断言 --layer-kind=oci 把 tar 标成 oci-layer-tar，且条目不变。
func TestOCILayerKind(t *testing.T) {
	p := fixture("c01-mtime", "pair", "left", "synthpkg-0.1.0.tar")
	plain, _, err := Load(p, "", DefaultMaxEntryBytes)
	if err != nil {
		t.Fatal(err)
	}
	oci, kind, err := Load(p, "oci", DefaultMaxEntryBytes)
	if err != nil {
		t.Fatal(err)
	}
	if kind != KindOCI {
		t.Fatalf("kind = %q，期望 %q", kind, KindOCI)
	}
	if len(oci) != len(plain) {
		t.Fatalf("OCI 层条目数 %d != 普通 tar %d", len(oci), len(plain))
	}
	if _, _, err := Load(p, "docker", DefaultMaxEntryBytes); err == nil {
		t.Fatal("未知 --layer-kind 必须报错")
	}
}

// TestMaxEntryBytesIsExplicit 断言超限是错误、且错误里指名条目（不静默丢弃）。
func TestMaxEntryBytesIsExplicit(t *testing.T) {
	p := fixture("c01-mtime", "pair", "left", "synthpkg-0.1.0.tar")
	_, _, err := Load(p, "", 8)
	if err == nil {
		t.Fatal("超过单条目上限必须报错")
	}
	if !strings.Contains(err.Error(), "超过上限") {
		t.Fatalf("错误信息应指名超限原因，实得：%v", err)
	}
}

// TestTruncatedIsRejected 断言"读不了的输入"是错误（exit 1 的来源），
// 不会被当成"空树 / 一致"（静默通过的经典漏洞）。
func TestTruncatedIsRejected(t *testing.T) {
	p := fixture("broken", "truncated.tar")
	if _, _, err := Load(p, "", DefaultMaxEntryBytes); err == nil {
		t.Fatal("截断的 tar 必须报错")
	}
}

// TestUnsupportedFormat 断言非容器输入是显式错误。
func TestUnsupportedFormat(t *testing.T) {
	if _, _, err := Load(filepath.Join("..", "..", "go.mod"), "", DefaultMaxEntryBytes); err == nil {
		t.Fatal("普通文本文件必须报 '无法识别容器格式'")
	}
	if _, _, err := Load(filepath.Join("..", "..", "no-such-file"), "", DefaultMaxEntryBytes); err == nil {
		t.Fatal("不存在的路径必须报错")
	}
}

// TestErrorDoesNotLeakAbsolutePath 断言错误信息不回显绝对路径（脱敏纪律）。
func TestErrorDoesNotLeakAbsolutePath(t *testing.T) {
	abs, err := filepath.Abs(filepath.Join("..", "..", "data", "fixtures", "no-such", "x.tar"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = Load(abs, "", DefaultMaxEntryBytes)
	if err == nil {
		t.Fatal("路径不存在必须报错")
	}
	if strings.Contains(err.Error(), abs) {
		t.Fatalf("错误信息回显了绝对路径：%v", err)
	}
}

// TestRootEntryNormalizesToEmpty 断言根条目（"." / ""）归一为空串，
// 由调用方跳过——不会被当成一个名为 "" 的条目。
func TestRootEntryNormalizesToEmpty(t *testing.T) {
	for _, in := range []string{"", ".", "./", "./."} {
		got, err := NormalizeRel(in)
		if err != nil {
			t.Errorf("NormalizeRel(%q) 报错: %v", in, err)
			continue
		}
		if got != "" {
			t.Errorf("NormalizeRel(%q) = %q，期望空串（根条目）", in, got)
		}
	}
}

// TestWhlJarAreZipByContent 断言形态识别只认**字节**，不认扩展名：
// 同一个 zip 换成 .whl / .jar 后缀，装载结果必须与 .zip 完全一致
// （plan/03 §二：whl = zip + dist-info 视为普通条目，jar = zip）。
func TestWhlJarAreZipByContent(t *testing.T) {
	src := fixture("c07-archive-hdr", "pair", "left", "synthpkg-0.1.0.zip")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	want, wantKind, err := Load(src, "", DefaultMaxEntryBytes)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, ext := range []string{".whl", ".jar"} {
		p := filepath.Join(dir, "synthpkg-0.1.0"+ext)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		got, kind, err := Load(p, "", DefaultMaxEntryBytes)
		if err != nil {
			t.Fatalf("%s 装载失败: %v", ext, err)
		}
		if kind != KindZip || kind != wantKind {
			t.Fatalf("%s 的 kind = %q，期望 %q", ext, kind, wantKind)
		}
		if len(got) != len(want) {
			t.Fatalf("%s 条目数 %d != zip %d", ext, len(got), len(want))
		}
		for i := range want {
			if got[i].Path != want[i].Path || got[i].Kind != want[i].Kind ||
				got[i].Size != want[i].Size || string(got[i].Data) != string(want[i].Data) {
				t.Fatalf("%s 第 %d 个条目与 zip 不一致", ext, i)
			}
		}
	}
}
