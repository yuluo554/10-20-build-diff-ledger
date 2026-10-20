package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// TestReadmeCommandsParse 是阶段 7 出口门①的本地形态（M5 落地为**真执行**）：
//
//  1. 抽出 `README.md` 里所有 fenced bash 块中的 `./bdiff ...` 命令（含 `\` 续行）；
//  2. 每条命令都**真的跑一遍**（样张路径按仓库根解析，`--out` 改写到临时目录）；
//  3. 退出码必须与紧随其后的 `# 期望 exit=N` 注释一致。
//
// 只断言"参数面能被接受"会让 README 与行为脱钩（文档写 exit 0、程序返回 2 也能绿）；
// 这里断言的是**可执行 + 退出码**，所以改语义而不同步 README 会红。
func TestReadmeCommandsParse(t *testing.T) {
	root := filepath.Join("..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("读 README 失败: %v", err)
	}
	cmds := extractReadmeCommands(t, string(raw))
	if len(cmds) < 4 {
		t.Fatalf("README 只抽出 %d 条 ./bdiff 命令（下界 4：安装 2 条 + 三场景）", len(cmds))
	}
	t.Logf("README 抽出 %d 条 ./bdiff 命令", len(cmds))
	outDir := t.TempDir()
	for _, c := range cmds {
		args := localizeArgs(t, root, outDir, c.args)
		code, _, errOut := run(args...)
		if strings.Contains(errOut, "flag provided but not defined") {
			t.Errorf("参数面被拒：bdiff %s\n%s", strings.Join(args, " "), errOut)
			continue
		}
		if code != c.want {
			t.Errorf("bdiff %s\n退出码 = %d，README 写的是 %d（stderr=%s）",
				strings.Join(args, " "), code, c.want, errOut)
		}
	}
}

type readmeCmd struct {
	args []string
	want int
}

// extractReadmeCommands 解析 fenced bash 块：把 `\` 续行拼成一条，
// 取 `./bdiff ` 开头的行，并按块内出现顺序与 `期望 exit=N` 配对。
func extractReadmeCommands(t *testing.T, md string) []readmeCmd {
	t.Helper()
	var out []readmeCmd
	for _, block := range fencedBashBlocks(md) {
		var logical []string
		var cur string
		for _, line := range strings.Split(block, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasSuffix(trimmed, "\\") {
				cur += strings.TrimSuffix(trimmed, "\\") + " "
				continue
			}
			logical = append(logical, cur+trimmed)
			cur = ""
		}
		if cur != "" {
			logical = append(logical, cur)
		}

		var wants []int
		for _, line := range logical {
			if m := regexp.MustCompile(`期望 exit=(\d)`).FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(m[1])
				wants = append(wants, n)
			}
		}
		var args [][]string
		for _, line := range logical {
			if !strings.HasPrefix(line, "./bdiff ") {
				continue
			}
			fields := strings.Fields(strings.TrimPrefix(line, "./bdiff "))
			args = append(args, fields)
		}
		if len(args) == 0 {
			continue
		}
		if len(wants) != len(args) {
			t.Fatalf("块内 %d 条 bdiff 命令但 %d 个『期望 exit=N』标注（必须 1:1）:\n%s", len(args), len(wants), block)
		}
		for i := range args {
			out = append(out, readmeCmd{args: args[i], want: wants[i]})
		}
	}
	return out
}

func fencedBashBlocks(md string) []string {
	var blocks []string
	lines := strings.Split(md, "\n")
	in, isBash := false, false
	var cur []string
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if !in {
				in = true
				isBash = strings.HasPrefix(line, "```bash")
				cur = nil
				continue
			}
			if isBash {
				blocks = append(blocks, strings.Join(cur, "\n"))
			}
			in, isBash = false, false
			continue
		}
		if in {
			cur = append(cur, line)
		}
	}
	return blocks
}

// localizeArgs 把 README 里的仓库相对路径改成测试可用的路径：
// `data/...` 前置仓库根；`--out` 的值改写到临时目录（不污染工作区）。
func localizeArgs(t *testing.T, root, outDir string, args []string) []string {
	t.Helper()
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--out":
			out = append(out, a, outDir)
			i++
		case strings.HasPrefix(a, "data/"):
			out = append(out, filepath.Join(root, a))
		default:
			out = append(out, a)
		}
	}
	return out
}
