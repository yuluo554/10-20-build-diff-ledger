package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/artifact"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/attribute"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/normalize"
)

// -update 重写 data/golden/attribute/*.json（生成命令见该目录 README）。
var update = flag.Bool("update", false, "rewrite data/golden/attribute/*.json")

var fixtureCases = []string{
	"c01-mtime", "c02-owner", "c03-mode", "c04-prefix", "c05-order",
	"c06-gzip-opts", "c07-archive-hdr", "c08-toolchain", "c09-mtime-owner",
	"c10-toolchain-path", "c11-real-change", "c12-mixed", "c13-signature",
	"c14-pax-unknown",
}

// fixtureArtifacts 返回 (左侧归档, 右侧归档)。样张目录里各只有一个产物文件。
func fixtureArtifacts(t *testing.T, caseName string) (string, string) {
	t.Helper()
	root := filepath.Join("..", "..", "data", "fixtures", caseName, "pair")
	pick := func(side string) string {
		dir := filepath.Join(root, side)
		ents, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", dir, err)
		}
		var files []string
		for _, e := range ents {
			if !e.IsDir() {
				files = append(files, e.Name())
			}
		}
		if len(files) != 1 {
			t.Fatalf("%s 下的产物数 = %d，期望 1", dir, len(files))
		}
		return filepath.Join(dir, files[0])
	}
	return pick("left"), pick("right")
}

type expectFile struct {
	Case   string `json:"case"`
	Expect struct {
		CountsPositive  []string `json:"counts_positive"`
		CountsAllowed   []string `json:"counts_allowed"`
		MinimalCauseSet []string `json:"minimal_cause_set"`
		ResidueBytes    int64    `json:"residue_bytes"`
		ExitCode        int      `json:"exit_code"`
	} `json:"expect"`
}

func loadExpect(t *testing.T, caseName string) expectFile {
	t.Helper()
	p := filepath.Join("..", "..", "data", "fixtures", caseName, "expect.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读 expect.json 失败: %v", err)
	}
	var e expectFile
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatalf("解析 expect.json 失败: %v", err)
	}
	if e.Case != caseName {
		t.Fatalf("expect.json 的 case = %q，期望 %q", e.Case, caseName)
	}
	return e
}

// TestFixtureMatrix 是 M3 DoD 的"14 组分类命中率 100%"断言（expect.json 驱动）：
// 正例必须出现、出现过的类别必须在 allowed 内、residue 与退出码逐项对齐。
func TestFixtureMatrix(t *testing.T) {
	for _, name := range fixtureCases {
		t.Run(name, func(t *testing.T) {
			exp := loadExpect(t, name)
			left, right := fixtureArtifacts(t, name)
			code, out, errOut := run("compare", "--left", left, "--right", right)
			if code != exp.Expect.ExitCode {
				t.Fatalf("退出码 = %d，期望 %d（stderr=%s）", code, exp.Expect.ExitCode, errOut)
			}
			var res attribute.Result
			if err := json.Unmarshal([]byte(out), &res); err != nil {
				t.Fatalf("解析输出失败: %v\n%s", err, out)
			}
			if res.ResidueBytes != exp.Expect.ResidueBytes {
				t.Fatalf("residue_bytes = %d，期望 %d", res.ResidueBytes, exp.Expect.ResidueBytes)
			}
			allowed := map[string]bool{}
			for _, r := range exp.Expect.CountsAllowed {
				allowed[r] = true
			}
			for _, r := range exp.Expect.CountsPositive {
				if res.Counts.Values[r] == 0 {
					t.Errorf("%s 未命中（负例！）diffs=%+v", r, res.Diffs)
				}
			}
			for _, r := range ruleIDs() {
				if res.Counts.Values[r] > 0 && !allowed[r] {
					t.Errorf("出现了不允许的类别 %s（计数 %d）", r, res.Counts.Values[r])
				}
			}
			if res.Counts.Values["T12"] > 0 && res.ResidueBytes <= 0 {
				t.Errorf("T12 出现但 residue = %d（§4.1 要求 > 0）", res.ResidueBytes)
			}
			if !sort.SliceIsSorted(res.Diffs, func(i, j int) bool {
				if res.Diffs[i].Path != res.Diffs[j].Path {
					return res.Diffs[i].Path < res.Diffs[j].Path
				}
				if res.Diffs[i].Key != res.Diffs[j].Key {
					return res.Diffs[i].Key < res.Diffs[j].Key
				}
				return res.Diffs[i].RuleID < res.Diffs[j].RuleID
			}) {
				t.Errorf("diffs 未按 (path, key, rule_id) 排序")
			}
		})
	}
}

// TestExpectedCauseSetEqualizesDigests 用 expect.json 的 minimal_cause_set 复算树摘要：
// 非 null → 施加该集合后两侧摘要必须相等；null（真改）→ 全集 T1–T11 后仍不等。
// 这是 M3"树摘要重算一致"的落点；**搜索算法本身**在 M4 落地。
func TestExpectedCauseSetEqualizesDigests(t *testing.T) {
	all := []string{}
	for _, r := range ruleIDs() {
		if r != "T12" {
			all = append(all, r)
		}
	}
	for _, name := range fixtureCases {
		t.Run(name, func(t *testing.T) {
			exp := loadExpect(t, name)
			left, right := fixtureArtifacts(t, name)
			le := entriesOf(t, left)
			re := entriesOf(t, right)
			if exp.Expect.MinimalCauseSet == nil {
				if normalize.TreeDigest(all, le) == normalize.TreeDigest(all, re) {
					t.Fatal("期望无解，但全集归一化后摘要相等")
				}
				return
			}
			set := append([]string(nil), exp.Expect.MinimalCauseSet...)
			if d1, d2 := normalize.TreeDigest(set, le), normalize.TreeDigest(set, re); d1 != d2 {
				t.Fatalf("施加 %v 后摘要仍不等：%s vs %s", set, d1, d2)
			}
			// 真·最小性：去掉集合里任一规则后必须不等（M4 的显式非超集断言在 M4 复跑）。
			for i := range set {
				sub := append(append([]string(nil), set[:i]...), set[i+1:]...)
				if len(sub) == 0 {
					continue
				}
				if normalize.TreeDigest(sub, le) == normalize.TreeDigest(sub, re) {
					t.Fatalf("%v 去掉 %s 后摘要仍相等——期望集不是最小", set, set[i])
				}
			}
		})
	}
}

// TestEachRuleOpIsIdempotent 是 plan/05 M3 DoD 的"逐 op 幂等"断言（T1–T11 各一条）。
// 每个 rule 都拿一个**真实触发它**的样张侧：既验证幂等，也验证 op 确实改动了输入
// （防"op 仍是 nil/no-op 却幂等地通过"）。
func TestEachRuleOpIsIdempotent(t *testing.T) {
	cases := []struct{ rule, fixture, side string }{
		{"T1", "c01-mtime", "left"},
		{"T2", "c02-owner", "left"},
		{"T3", "c03-mode", "right"},
		{"T4", "c04-prefix", "left"},
		{"T5", "c05-order", "left"},
		{"T6", "c06-gzip-opts", "right"},
		{"T7", "c14-pax-unknown", "left"},
		{"T8", "c08-toolchain", "left"},
		{"T9", "c12-mixed", "left"},
		{"T10", "c13-signature", "left"},
		{"T11", "c10-toolchain-path", "left"},
	}
	for _, c := range cases {
		t.Run(c.rule, func(t *testing.T) {
			left, right := fixtureArtifacts(t, c.fixture)
			p := left
			if c.side == "right" {
				p = right
			}
			in := entriesOf(t, p)
			once := normalize.Apply([]string{c.rule}, in)
			twice := normalize.Apply([]string{c.rule}, once)
			if !reflect.DeepEqual(once, twice) {
				t.Fatalf("%s 不幂等", c.rule)
			}
			if reflect.DeepEqual(in, once) {
				t.Fatalf("%s 在这个样张上没有改动任何条目——op 可能仍是空实现", c.rule)
			}
		})
	}
}

// TestAttributeGolden 是 data/golden/attribute/*.json 的逐字节断言，走真实 CLI 路径
// （Run → artifact.LoadDocument → facts.Extract → attribute.Attribute → MarshalValue）。
func TestAttributeGolden(t *testing.T) {
	root := filepath.Join("..", "..")
	goldenDir := filepath.Join(root, "data", "golden", "attribute")
	if *update {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range fixtureCases {
		t.Run(name, func(t *testing.T) {
			left, right := fixtureArtifacts(t, name)
			code, out, errOut := run("compare", "--left", left, "--right", right)
			if code != 0 && code != 2 && code != 3 {
				t.Fatalf("compare 退出码 %d（stderr=%s）", code, errOut)
			}
			gp := filepath.Join(goldenDir, name+".json")
			if *update {
				if err := os.WriteFile(gp, []byte(out), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("读 golden 失败（用 -update 生成）: %v", err)
			}
			if !bytes.Equal([]byte(out), want) {
				t.Fatalf("与 golden 不一致\n got=%s\nwant=%s", out, want)
			}
			// 双跑逐字节（确定性）。
			_, out2, _ := run("compare", "--left", left, "--right", right)
			if out2 != out {
				t.Fatal("同输入双跑不一致")
			}
		})
	}
}

// TestCompareUsageErrors 断言 compare 的用法错误一律 exit 1（不占用 2/3）。
func TestCompareUsageErrors(t *testing.T) {
	left, right := fixtureArtifacts(t, "c01-mtime")
	cases := [][]string{
		{"compare"},
		{"compare", "--left", left},
		{"compare", "--left", left, "--right", right, "--format", "xml"},
		{"compare", "--left", left, "--right", right, "--budget", "T13"},
		{"compare", "--left", left, "--right", right, "--out", "x"},
		{"compare", "--left", "no-such", "--right", right},
	}
	for _, args := range cases {
		code, _, errOut := run(args...)
		if code != ExitUsage {
			t.Errorf("%v 退出码 = %d，期望 1（stderr=%s）", args, code, errOut)
		}
	}
}

// TestCompareBudgetAllowsMetadataOnly：--budget 只影响 0/2，不得把 3 降级。
func TestCompareBudgetAllowsMetadataOnly(t *testing.T) {
	left, right := fixtureArtifacts(t, "c01-mtime")
	if code, _, _ := run("compare", "--left", left, "--right", right, "--budget", "T1"); code != ExitOK {
		t.Fatalf("--budget=T1 覆盖 c01 应 exit 0，实得 %d", code)
	}
	if code, _, _ := run("compare", "--left", left, "--right", right, "--budget", "T2"); code != ExitMetaOnly {
		t.Fatalf("--budget=T2 不覆盖 c01 应 exit 2，实得 %d", code)
	}
	l3, r3 := fixtureArtifacts(t, "c11-real-change")
	if code, _, _ := run("compare", "--left", l3, "--right", r3, "--budget", "all"); code != ExitResidue {
		t.Fatalf("--budget=all 不得把残留降级，实得 %d", code)
	}
}

// entriesOf 走与 CLI 相同的装载 + facts 抽取，再用 attribute 的预处理
// （公共根前缀 / 归档内序号）生成归一化条目。
func entriesOf(t *testing.T, p string) []normalize.Entry {
	t.Helper()
	doc, err := artifact.LoadDocument(p, "", 0)
	if err != nil {
		t.Fatalf("装载 %s 失败: %v", p, err)
	}
	side, err := buildAttributeSide(doc)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := attribute.Entries(side)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func ruleIDs() []string {
	return []string{"T1", "T2", "T3", "T4", "T5", "T6", "T7", "T8", "T9", "T10", "T11", "T12"}
}
