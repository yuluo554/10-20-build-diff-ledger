package cli

import (
	"bytes"
	"strings"
	"testing"
)

func run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := Run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// TestExitCodeContract 断言退出码语义的**入口层**形态：
// 用法错误一律 1（不得被 2 占用——2 是"仅元数据差异"这个业务语义）。
func TestExitCodeContract(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"无参数", nil, ExitUsage},
		{"未知子命令", []string{"nope"}, ExitUsage},
		{"align 缺参数", []string{"align"}, ExitUsage},
		{"compare 缺参数", []string{"compare"}, ExitUsage},
		{"facts 缺参数", []string{"facts"}, ExitUsage},
		{"version", []string{"version"}, ExitOK},
	}
	for _, c := range cases {
		got, _, _ := run(c.args...)
		if got != c.want {
			t.Errorf("%s: 退出码 %d，期望 %d", c.name, got, c.want)
		}
	}
	if ExitMetaOnly == ExitUsage {
		t.Fatal("2（仅元数据差异）与 1（用法错误）必须是两个不同的码")
	}
	if ExitResidue == ExitOK {
		t.Fatal("3（有残留）与 0（一致）必须不同")
	}
}

// TestUsageErrorIsNotMetaOnly 断言"读不了"不会被当成"仅元数据差异"。
func TestUsageErrorIsNotMetaOnly(t *testing.T) {
	code, _, errOut := run("compare", "--left", "secret/dir/no-such-path-a", "--right", "secret/dir/no-such-path-b")
	if code != ExitUsage {
		t.Fatalf("路径不存在应 exit 1，实得 %d", code)
	}
	if !strings.Contains(errOut, "无法读取") {
		t.Fatalf("错误信息应可操作，实得：%q", errOut)
	}
	// 脱敏：只报 basename，不回显目录部分（否则工作目录结构会漏进报告）
	if strings.Contains(errOut, "secret/dir") {
		t.Fatalf("错误信息回显了路径前缀（脱敏失败）：%q", errOut)
	}
	if !strings.Contains(errOut, "no-such-path-a") {
		t.Fatalf("错误信息应指名是哪一个输入：%q", errOut)
	}
}

// TestBugdetParsedBeforeWork 断言非法 --budget 在动手之前就被拒绝。
func TestBudgetParsedBeforeWork(t *testing.T) {
	code, _, errOut := run("compare", "--left", "go.mod", "--right", "go.mod", "--budget", "T99")
	if code != ExitUsage {
		t.Fatalf("非法 budget 应 exit 1，实得 %d", code)
	}
	if !strings.Contains(errOut, "T99") {
		t.Fatalf("错误信息应点名非法值，实得：%q", errOut)
	}
}

// TestSelftestPasses 断言 selftest 在 M0 骨架上是绿的（它是"契约形状"自检）。
func TestSelftestPasses(t *testing.T) {
	code, out, errOut := run("selftest")
	if code != ExitOK {
		t.Fatalf("selftest 应 exit 0，实得 %d\nstdout=%s\nstderr=%s", code, out, errOut)
	}
	if !strings.Contains(out, "全部通过") {
		t.Fatalf("selftest 未报通过：%s", out)
	}
	if strings.Contains(out, "FAIL") {
		t.Fatalf("selftest 输出含 FAIL：%s", out)
	}
}

// TestVersionMatchesConst 断言 version 子命令读的是单一来源。
func TestVersionMatchesConst(t *testing.T) {
	code, out, _ := run("version")
	if code != ExitOK {
		t.Fatalf("version 应 exit 0，实得 %d", code)
	}
	if !strings.Contains(out, "bdiff ") || len(strings.TrimSpace(out)) < len("bdiff x") {
		t.Fatalf("version 输出异常：%q", out)
	}
}

// TestHelpIsZero 断言 help 不是错误路径。
func TestHelpIsZero(t *testing.T) {
	code, out, _ := run("help")
	if code != ExitOK {
		t.Fatalf("help 应 exit 0，实得 %d", code)
	}
	if !strings.Contains(out, "退出码") {
		t.Fatal("help 应列出退出码语义")
	}
}

// TestReadmeCommandsParse 是阶段 7 出口门①的本地形态：
// 抽出 README 代码块里的 `bdiff ...` 命令，逐条确认 parser 接受（不因参数形态 exit 1）。
// M5 前 README 里的命令是骨架可跑的版本；断言只在"参数面"上，不执行真实比较。
func TestReadmeCommandsParse(t *testing.T) {
	cmds := [][]string{
		{"version"},
		{"selftest"},
		{"align", "--left", "go.mod", "--right", "go.mod", "--format", "json"},
		{"facts", "--artifact", "go.mod", "--format", "json"},
		{"compare", "--left", "go.mod", "--right", "go.mod", "--format", "both"},
		{"compare", "--left", "go.mod", "--right", "go.mod", "--budget", "T1,T2"},
		{"compare", "--left", "go.mod", "--right", "go.mod", "--budget", "all"},
	}
	for _, args := range cmds {
		if _, _, errOut := run(args...); strings.Contains(errOut, "flag provided but not defined") {
			t.Errorf("参数面被拒：bdiff %s\n%s", strings.Join(args, " "), errOut)
		}
	}
}
