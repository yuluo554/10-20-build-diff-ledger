package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestAlignGoldenMatchesCLI 是 `data/golden/align/*.json` 的逐字节断言：
// 走的是**真实 CLI 路径**（Run → artifact.Load → align.Align → MarshalValue），
// 不是包内的直调，因此也钉住了键序与序列化口径。
func TestAlignGoldenMatchesCLI(t *testing.T) {
	root := filepath.Join("..", "..")
	cases := []struct {
		name   string
		golden string
	}{
		{"rename-add", "basic.json"},
		{"dup-hash", "dup-hash.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(root, "data", "golden", "align", c.golden))
			if err != nil {
				t.Fatalf("读 golden 失败: %v", err)
			}
			pair := filepath.Join(root, "data", "fixtures", "align-golden", c.name, "pair")
			code, out, errOut := run("align",
				"--left", filepath.Join(pair, "left"),
				"--right", filepath.Join(pair, "right"),
				"--format", "json",
			)
			if code != ExitOK {
				t.Fatalf("align 退出码 %d，stderr=%s", code, errOut)
			}
			if !bytes.Equal([]byte(out), want) {
				t.Fatalf("与 golden 不一致\n got=%s\nwant=%s", out, want)
			}
			// 双跑逐字节（DoD：TestAlignDeterministic 的 CLI 侧镜像）
			code2, out2, _ := run("align",
				"--left", filepath.Join(pair, "left"),
				"--right", filepath.Join(pair, "right"),
				"--format", "json",
			)
			if code2 != ExitOK || out2 != out {
				t.Fatalf("同输入双跑不一致（exit %d）", code2)
			}
		})
	}
}

// TestAlignMarkdownIsDeterministic 断言 --format md 也双跑一致，且不含时间性文案。
func TestAlignMarkdownIsDeterministic(t *testing.T) {
	pair := filepath.Join("..", "..", "data", "fixtures", "align-golden", "rename-add", "pair")
	args := []string{"align", "--left", filepath.Join(pair, "left"), "--right", filepath.Join(pair, "right"), "--format", "md"}
	code, out, errOut := run(args...)
	if code != ExitOK {
		t.Fatalf("align --format md 退出码 %d，stderr=%s", code, errOut)
	}
	_, out2, _ := run(args...)
	if out != out2 {
		t.Fatal("align --format md 双跑不一致")
	}
	for _, bad := range []string{"运行时间", "生成于", "主机"} {
		if bytes.Contains([]byte(out), []byte(bad)) {
			t.Fatalf("align md 含时间性/身份性文案 %q", bad)
		}
	}
}

// TestAlignOnFixtureArchives 断言 align 能真实装载我们生成的三种容器并给出计数。
func TestAlignOnFixtureArchives(t *testing.T) {
	cases := []struct {
		name     string
		left     string
		right    string
		wantKind string
		wantSame int
	}{
		{
			name:     "c01-mtime",
			left:     "c01-mtime/pair/left/synthpkg-0.1.0.tar",
			right:    "c01-mtime/pair/right/synthpkg-0.1.0.tar",
			wantKind: "tar",
		},
		{
			name:     "c07-archive-hdr",
			left:     "c07-archive-hdr/pair/left/synthpkg-0.1.0.zip",
			right:    "c07-archive-hdr/pair/right/synthpkg-0.1.0.zip",
			wantKind: "zip",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := filepath.Join("..", "..", "data", "fixtures")
			code, out, errOut := run("align",
				"--left", filepath.Join(base, filepath.FromSlash(c.left)),
				"--right", filepath.Join(base, filepath.FromSlash(c.right)),
			)
			if code != ExitOK {
				t.Fatalf("exit=%d stderr=%s", code, errOut)
			}
			if !bytes.Contains([]byte(out), []byte(`"kind":"`+c.wantKind+`"`)) {
				t.Fatalf("输出未报容器形态 %q：%s", c.wantKind, out)
			}
			if !bytes.Contains([]byte(out), []byte(`"SAME":`)) {
				t.Fatalf("输出缺 SAME 计数：%s", out)
			}
		})
	}
}
