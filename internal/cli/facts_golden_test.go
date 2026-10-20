package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestFactsGoldenMatchesCLI 是 `data/golden/facts/*.json` 的逐字节断言：
// 走真实 CLI 路径（Run → artifact.LoadDocument → facts.Build → report.MarshalValue），
// 因而同时钉住键序、容器级事实的 path 约定与序列化口径。
func TestFactsGoldenMatchesCLI(t *testing.T) {
	root := filepath.Join("..", "..")
	cases := []struct {
		name   string
		input  string
		golden string
	}{
		{"tar", "c01-mtime/pair/left/synthpkg-0.1.0.tar", "c01-tar-left.json"},
		{"gzip", "c06-gzip-opts/pair/left/synthpkg-0.1.0.tar.gz", "c06-gzip-left.json"},
		{"zip", "c07-archive-hdr/pair/left/synthpkg-0.1.0.zip", "c07-zip-left.json"},
		{"elf", "c08-toolchain/pair/left/synthpkg-0.1.0.tar", "c08-elf-left.json"},
		{"pax", "c14-pax-unknown/pair/left/synthpkg-0.1.0.tar", "c14-pax-left.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(root, "data", "golden", "facts", c.golden))
			if err != nil {
				t.Fatalf("读 golden 失败: %v", err)
			}
			artifact := filepath.Join(root, "data", "fixtures", filepath.FromSlash(c.input))
			code, out, errOut := run("facts", "--artifact", artifact, "--format", "json")
			if code != ExitOK {
				t.Fatalf("facts 退出码 %d，stderr=%s", code, errOut)
			}
			if !bytes.Equal([]byte(out), want) {
				t.Fatalf("与 golden 不一致\n got=%s\nwant=%s", out, want)
			}
			// 双跑逐字节（确定性）
			code2, out2, _ := run("facts", "--artifact", artifact, "--format", "json")
			if code2 != ExitOK || out2 != out {
				t.Fatalf("同输入双跑不一致（exit %d）", code2)
			}
		})
	}
}

// TestFactsUsageErrors 断言 facts 的用法错误一律 exit 1（不得占用 2）。
func TestFactsUsageErrors(t *testing.T) {
	cases := [][]string{
		{"facts"},                                      // 缺 --artifact
		{"facts", "--artifact", "no-such-file"},        // 路径不存在
		{"facts", "--artifact", "x", "--format", "md"}, // 非法格式
		{"facts", "--artifact", "x", "--layer-kind", "docker"},
	}
	for _, args := range cases {
		code, _, errOut := run(args...)
		if code != ExitUsage {
			t.Errorf("%v 退出码 = %d，期望 %d（stderr=%s）", args, code, ExitUsage, errOut)
		}
	}
}

// TestFactsRejectsUnreadableInput 断言损坏归档是 exit 1 而不是"空事实/一致"。
func TestFactsRejectsUnreadableInput(t *testing.T) {
	p := filepath.Join("..", "..", "data", "fixtures", "broken", "truncated.tar")
	code, out, _ := run("facts", "--artifact", p)
	if code != ExitUsage {
		t.Fatalf("截断归档退出码 = %d，期望 1（stdout=%s）", code, out)
	}
}
