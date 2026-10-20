package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/report"
)

// TestReportGolden 是 data/golden/report/ 的逐字节断言（M5 DoD 第 1 条）：
// 14 组样张各一份 report.json + report.md，双跑逐字节。
//
// 生成：go test ./internal/cli -run TestReportGolden -update
func TestReportGolden(t *testing.T) {
	root := filepath.Join("..", "..")
	goldenDir := filepath.Join(root, "data", "golden", "report")
	if *update {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range fixtureCases {
		t.Run(name, func(t *testing.T) {
			left, right := fixtureArtifacts(t, name)
			json1, md1 := compareBoth(t, left, right)

			if *update {
				writeGolden(t, filepath.Join(goldenDir, name+".json"), json1)
				writeGolden(t, filepath.Join(goldenDir, name+".md"), md1)
				return
			}
			gotJSON := readGolden(t, filepath.Join(goldenDir, name+".json"))
			gotMD := readGolden(t, filepath.Join(goldenDir, name+".md"))
			if !bytes.Equal(json1, gotJSON) {
				t.Fatalf("report.json 与 golden 不一致\n got=%s\nwant=%s", json1, gotJSON)
			}
			if !bytes.Equal(md1, gotMD) {
				t.Fatalf("report.md 与 golden 不一致\n got=%s\nwant=%s", md1, gotMD)
			}

			// 双跑逐字节（确定性）。
			json2, md2 := compareBoth(t, left, right)
			if !bytes.Equal(json1, json2) || !bytes.Equal(md1, md2) {
				t.Fatal("同输入双跑不一致")
			}

			// golden 必须与**真实** `--format=json` / `--format=md` 逐字节一致：
			// `--format=both` 只是把两段拼起来，拼接口径错了会让"同一份报告"
			// 在两个 format 下字节不同（本测试首版就多带了一个换行）。
			_, outJSON, _ := run("compare", "--left", left, "--right", right, "--format", "json")
			if !bytes.Equal(json1, []byte(outJSON)) {
				t.Fatalf("golden .json 与 --format=json 不一致（golden=%d 字节，实际=%d 字节）",
					len(json1), len(outJSON))
			}
			_, outMD, _ := run("compare", "--left", left, "--right", right, "--format", "md")
			if !bytes.Equal(md1, []byte(outMD)) {
				t.Fatalf("golden .md 与 --format=md 不一致（golden=%d 字节，实际=%d 字节）",
					len(md1), len(outMD))
			}
		})
	}
}

// compareBoth 用 --format=both 跑一次，按固定分隔切成 JSON 与 MD。
//
// 切分口径：`both` 的输出 = report.json ‖ "\n" ‖ report.md。分隔用的那个 `\n`
// **不属于任何一段**——否则 golden 的 .json 会比 `--format=json` 多一个换行，
// 两份"同一个报告"的字节就不一致了（本测试首版踩过：cmp 报 EOF after byte 504）。
func compareBoth(t *testing.T, left, right string) ([]byte, []byte) {
	t.Helper()
	code, out, errOut := run("compare", "--left", left, "--right", right, "--format", "both")
	if code != ExitOK && code != ExitMetaOnly && code != ExitResidue {
		t.Fatalf("compare 退出码 %d（stderr=%s）", code, errOut)
	}
	i := strings.Index(out, "\n# bdiff 差异归因账本")
	if i < 0 {
		t.Fatalf("--format=both 输出里找不到 md 段的分界：%q", out)
	}
	return []byte(out[:i]), []byte(out[i+1:])
}

func writeGolden(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readGolden(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读 golden 失败（用 -update 生成）: %v", err)
	}
	return b
}

// TestReportShape 断言 report.json 的字段口径（§4.8 + §4.5）：
// 键序、counts 键集合、left/right 只有 basename（无绝对路径）、
// `minimal_cause_set` 的 null/[] 区分、residual_items 与 residue_bytes 同源。
func TestReportShape(t *testing.T) {
	// 有解：c09（[T1,T2]）
	left, right := fixtureArtifacts(t, "c09-mtime-owner")
	code, out, _ := run("compare", "--left", left, "--right", right)
	if code != ExitMetaOnly {
		t.Fatalf("c09 应 exit 2，实得 %d", code)
	}
	var rep map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("report.json 解析失败: %v\n%s", err, out)
	}
	wantOrder := []string{
		"schema_version", "tool_version", "left", "right", "counts", "renamed",
		"added", "removed", "residue_bytes", "minimal_cause_set", "residual_items", "notes",
	}
	pos := -1
	for _, k := range wantOrder {
		i := strings.Index(out, `"`+k+`":`)
		if i < 0 {
			t.Fatalf("缺键 %s：%s", k, out)
		}
		if i < pos {
			t.Fatalf("键 %s 顺序错：%s", k, out)
		}
		pos = i
	}
	if got := strings.TrimSpace(string(rep["minimal_cause_set"])); got != `["T1","T2"]` {
		t.Fatalf("c09 minimal_cause_set = %s，期望 [\"T1\",\"T2\"]", got)
	}
	for _, side := range []string{"left", "right"} {
		var s struct {
			Basename string `json:"basename"`
			SHA256   string `json:"sha256"`
		}
		if err := json.Unmarshal(rep[side], &s); err != nil {
			t.Fatalf("%s 解析失败: %v", side, err)
		}
		if strings.ContainsAny(s.Basename, `/\`) {
			t.Fatalf("%s.basename 含路径分隔符（脱敏失败）：%q", side, s.Basename)
		}
		if len(s.SHA256) != 64 {
			t.Fatalf("%s.sha256 不是 64 位 hex：%q", side, s.SHA256)
		}
	}
	if !strings.Contains(out, `"residual_items":[]`) {
		t.Fatalf("c09 的 residual_items 应为空数组：%s", out)
	}

	// 无解：c12 → null + residue>0 + residual_items 非空且和 == residue_bytes
	l12, r12 := fixtureArtifacts(t, "c12-mixed")
	code, out, _ = run("compare", "--left", l12, "--right", r12)
	if code != ExitResidue {
		t.Fatalf("c12 应 exit 3，实得 %d", code)
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("c12 report.json 解析失败: %v", err)
	}
	if got := strings.TrimSpace(string(rep["minimal_cause_set"])); got != "null" {
		t.Fatalf("c12 minimal_cause_set = %s，期望 null（无解不得写成 []）", got)
	}
	var items []struct {
		Path           string `json:"path"`
		RuleID         string `json:"rule_id"`
		BytesDifferent int64  `json:"bytes_different"`
	}
	if err := json.Unmarshal(rep["residual_items"], &items); err != nil {
		t.Fatalf("residual_items 解析失败: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("c12 有残留但 residual_items 为空")
	}
	var sum int64
	for _, it := range items {
		sum += it.BytesDifferent
	}
	var residue int64
	if err := json.Unmarshal(rep["residue_bytes"], &residue); err != nil {
		t.Fatal(err)
	}
	if sum != residue {
		t.Fatalf("residual_items 之和 %d ≠ residue_bytes %d（两者必须同源）", sum, residue)
	}

	// 完全一致：同文件自比 → [] 且 exit 0
	code, out, _ = run("compare", "--left", l12, "--right", l12)
	if code != ExitOK {
		t.Fatalf("同文件自比应 exit 0，实得 %d", code)
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(rep["minimal_cause_set"])); got != "[]" {
		t.Fatalf("自比 minimal_cause_set = %s，期望 []（完全一致不得写成 null）", got)
	}
}

// TestReportOutDir 断言 --out 落盘：两份文件、内容与 stdout 一致、可重复写、目录不存在时自建。
func TestReportOutDir(t *testing.T) {
	left, right := fixtureArtifacts(t, "c01-mtime")
	dir := filepath.Join(t.TempDir(), "nested", "report")

	code, out, errOut := run("compare", "--left", left, "--right", right, "--out", dir, "--format", "json")
	if code != ExitMetaOnly {
		t.Fatalf("退出码 %d，期望 2（stderr=%s）", code, errOut)
	}
	jsonBuf, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatalf("report.json 未落盘: %v", err)
	}
	if string(jsonBuf) != out {
		t.Fatal("report.json 与 stdout 的 JSON 不一致")
	}
	mdBuf, err := os.ReadFile(filepath.Join(dir, "report.md"))
	if err != nil {
		t.Fatalf("report.md 未落盘: %v", err)
	}
	if !strings.HasPrefix(string(mdBuf), "# bdiff 差异归因账本\n") {
		t.Fatalf("report.md 头部不符：%q", string(mdBuf[:40]))
	}
	if strings.Contains(string(mdBuf), "\r") {
		t.Fatal("report.md 含 CR（换行必须只用 \\n）")
	}

	// 再跑一次：逐字节一致（幂等落盘）。
	run("compare", "--left", left, "--right", right, "--out", dir, "--format", "json")
	jsonBuf2, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(jsonBuf, jsonBuf2) {
		t.Fatal("两次 --out 写盘不一致")
	}

	// --format=md 时 stdout 是 md（与落盘 md 相同）
	code, outMD, _ := run("compare", "--left", left, "--right", right, "--format", "md")
	if code != ExitMetaOnly {
		t.Fatalf("--format=md 退出码 %d，期望 2", code)
	}
	if outMD != string(mdBuf) {
		t.Fatal("--format=md 的 stdout 与 report.md 不一致")
	}
}

// TestReportOutDirWriteError 断言写盘失败 → exit 1（不得静默）。
func TestReportOutDirWriteError(t *testing.T) {
	left, right := fixtureArtifacts(t, "c01-mtime")
	// 拿一个已存在的**文件**当输出目录：MkdirAll 必然失败。
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run("compare", "--left", left, "--right", right, "--out", filepath.Join(blocker, "sub"))
	if code != ExitUsage {
		t.Fatalf("写盘失败应 exit 1，实得 %d（stderr=%s）", code, errOut)
	}
	if !strings.Contains(errOut, "写盘失败") {
		t.Fatalf("错误信息应说明写盘失败：%q", errOut)
	}
}

// TestExitCodeMatrix 是 M5 DoD 的"退出码 0/1/2/3 各一条样张"（G3 的关闭证据）。
// 四条都用**样张或明确用法错误**，不靠 mock：
//
//	0 → 同文件自比（完全一致）；c01 + --budget=T1（放行后一致）
//	1 → broken/truncated.tar（不可解析）；路径不存在
//	2 → c01-mtime（仅元数据差异）
//	3 → c11-real-change（真内容残留）
func TestExitCodeMatrix(t *testing.T) {
	c01L, c01R := fixtureArtifacts(t, "c01-mtime")
	c11L, c11R := fixtureArtifacts(t, "c11-real-change")
	broken := filepath.Join("..", "..", "data", "fixtures", "broken", "truncated.tar")

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"0 完全一致（自比）", []string{"compare", "--left", c01L, "--right", c01L}, ExitOK},
		{"0 budget 放行", []string{"compare", "--left", c01L, "--right", c01R, "--budget", "T1"}, ExitOK},
		{"1 不可解析（截断归档）", []string{"compare", "--left", broken, "--right", c01R}, ExitUsage},
		{"1 路径不存在", []string{"compare", "--left", "no-such-a", "--right", "no-such-b"}, ExitUsage},
		{"2 仅元数据差异", []string{"compare", "--left", c01L, "--right", c01R}, ExitMetaOnly},
		{"3 有残留", []string{"compare", "--left", c11L, "--right", c11R}, ExitResidue},
	}
	seen := map[int]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, _, errOut := run(c.args...)
			if code != c.want {
				t.Fatalf("退出码 = %d，期望 %d（stderr=%s）", code, c.want, errOut)
			}
			seen[c.want] = true
		})
	}
	// 4 个码必须都被这组用例覆盖到（防"矩阵少一行"）。
	for _, code := range []int{ExitOK, ExitUsage, ExitMetaOnly, ExitResidue} {
		if !seen[code] {
			t.Fatalf("退出码 %d 在矩阵里没有样张覆盖", code)
		}
	}
}

// TestCompareBudgetThreeFormsE2E 是 --budget 三形态的 CLI 端到端断言（§4.6 + K7）。
// 单值 / 逗号多值 / all 三形态都走真实样张，且都验证"不得把 3 降级"。
func TestCompareBudgetThreeFormsE2E(t *testing.T) {
	c09L, c09R := fixtureArtifacts(t, "c09-mtime-owner") // T1+T2
	c01L, c01R := fixtureArtifacts(t, "c01-mtime")       // T1
	c11L, c11R := fixtureArtifacts(t, "c11-real-change") // T12 + residue

	cases := []struct {
		name   string
		budget string
		left   string
		right  string
		want   int
	}{
		{"单值覆盖", "T1", c01L, c01R, ExitOK},
		{"单值不覆盖", "T2", c01L, c01R, ExitMetaOnly},
		{"逗号多值覆盖", "T1,T2", c09L, c09R, ExitOK},
		{"逗号多值部分覆盖", "T1", c09L, c09R, ExitMetaOnly},
		{"all 覆盖", "all", c09L, c09R, ExitOK},
		{"all 不降级残留", "all", c11L, c11R, ExitResidue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, _, errOut := run("compare", "--left", c.left, "--right", c.right, "--budget", c.budget)
			if code != c.want {
				t.Fatalf("--budget=%s 退出码 = %d，期望 %d（stderr=%s）", c.budget, code, c.want, errOut)
			}
		})
	}
}

// TestCompareFailOnResidue 断言 --fail-on=residue 只把 2 降为 0，**不降级 3**。
func TestCompareFailOnResidue(t *testing.T) {
	c01L, c01R := fixtureArtifacts(t, "c01-mtime")
	c11L, c11R := fixtureArtifacts(t, "c11-real-change")

	if code, _, _ := run("compare", "--left", c01L, "--right", c01R, "--fail-on", "residue"); code != ExitOK {
		t.Fatalf("--fail-on=residue 下仅元数据差异应 exit 0，实得 %d", code)
	}
	if code, _, _ := run("compare", "--left", c11L, "--right", c11R, "--fail-on", "residue"); code != ExitResidue {
		t.Fatalf("--fail-on=residue 不得把残留降级，实得 %d", code)
	}
	if code, _, _ := run("compare", "--left", c01L, "--right", c01R, "--fail-on", "nope"); code != ExitUsage {
		t.Fatalf("非法 --fail-on 应 exit 1，实得 %d", code)
	}
}

// TestBudgetCoversActualCategories 钉住"白名单覆盖的是**实际类别**，不是最小集"
// （HANDOFF-M5 §三 的已知难点；§4.6 的措辞是"实际差异类别 ⊆ 白名单"）。
//
// 反例构型：实际出现了 T1 与 T3，但**最小集只有 [T1]**——因为 T3 的差异恰好被
// T1 的归一化顺带抹平。此时 `--budget=T1` 必须**不放行**：T3 确实出现过，
// 放行它是另一件事（白名单是"我允许这些类别出现"，不是"最少改哪几项"）。
//
// 14 组样张里没有这种构型（真实样张的最小集都覆盖了全部实际类别），
// 所以这条判据只能直接打在 budgetCovers 上——用样张验它会恒绿（假门）。
func TestBudgetCoversActualCategories(t *testing.T) {
	rep := report.New(report.Side{}, report.Side{})
	rep.Counts.Values["T1"] = 1
	rep.Counts.Values["T3"] = 1
	rep.Counts.Values["total"] = 2
	rep.MinimalCauseSet = []string{"T1"}

	if budgetCovers(rep, []string{"T1"}) {
		t.Fatal("实际出现 T3 却按最小集放行了：白名单必须覆盖实际类别（§4.6）")
	}
	if !budgetCovers(rep, []string{"T1", "T3"}) {
		t.Fatal("实际类别 {T1,T3} 全在白名单内却未放行")
	}
	if budgetCovers(rep, nil) {
		t.Fatal("空白名单不得放行任何差异")
	}
}

// TestReportCountsKeyOrder 断言 counts 的 T1…T12 + total 键序（报告层，不是归因层）。
func TestReportCountsKeyOrder(t *testing.T) {
	left, right := fixtureArtifacts(t, "c01-mtime")
	_, out, _ := run("compare", "--left", left, "--right", right)
	i := strings.Index(out, `"counts":{`)
	if i < 0 {
		t.Fatalf("报告缺 counts：%s", out)
	}
	counts := out[i:]
	pos := -1
	for _, k := range append(append([]string{}, model.RuleIDs...), "total") {
		j := strings.Index(counts, `"`+k+`":`)
		if j < 0 {
			t.Fatalf("counts 缺 %s：%s", k, counts)
		}
		if j < pos {
			t.Fatalf("counts 中 %s 顺序错：%s", k, counts)
		}
		pos = j
	}
}

// TestReportMarkdownNoLeak 断言 report.md 不含绝对路径/主机名/时间性文案（K12）。
func TestReportMarkdownNoLeak(t *testing.T) {
	left, right := fixtureArtifacts(t, "c12-mixed")
	_, out, _ := run("compare", "--left", left, "--right", right, "--format", "md")
	for _, bad := range []string{"data/fixtures", "C:\\", "D:\\", "/home/", "/tmp/", "生成于", "运行时间", "UTC"} {
		if strings.Contains(out, bad) {
			t.Fatalf("report.md 泄漏 %q：\n%s", bad, out)
		}
	}
}
