package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/artifact"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/attribute"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/minimize"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/normalize"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/report"
)

// TestMinimizeFixtureMatrix 是 M4 的样张矩阵断言（expect.json 驱动）：
// `minimal_cause_set` 必须与搜索输出逐项一致（null = 无解，[] = 完全一致）。
func TestMinimizeFixtureMatrix(t *testing.T) {
	for _, name := range fixtureCases {
		t.Run(name, func(t *testing.T) {
			exp := loadExpect(t, name)
			set, found := searchFixture(t, name)
			if exp.Expect.MinimalCauseSet == nil {
				if found {
					t.Fatalf("期望无解，搜索却给出 %v", set)
				}
				return
			}
			if !found {
				t.Fatalf("期望 %v，搜索报告无解", exp.Expect.MinimalCauseSet)
			}
			if !reflect.DeepEqual(set, exp.Expect.MinimalCauseSet) {
				t.Fatalf("最小集 = %v，期望 %v", set, exp.Expect.MinimalCauseSet)
			}
		})
	}
}

// TestMinimizeDoDCases 钉住 plan/05 M4 DoD 的四条点名断言。
func TestMinimizeDoDCases(t *testing.T) {
	assertSet := func(name string, want []string) {
		t.Helper()
		got, found := searchFixture(t, name)
		if !found {
			t.Fatalf("%s：搜索报告无解，期望 %v", name, want)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s：最小集 = %v，期望 %v", name, got, want)
		}
	}

	// c01 → [T1]
	assertSet("c01-mtime", []string{"T1"})

	// c09 → [T1,T2] 且 len==2（显式非超集）
	got, found := searchFixture(t, "c09-mtime-owner")
	if !found || !reflect.DeepEqual(got, []string{"T1", "T2"}) {
		t.Fatalf("c09-mtime-owner：最小集 = %v（found=%v），期望 [T1 T2]", got, found)
	}
	if len(got) != 2 {
		t.Fatalf("c09-mtime-owner：最小集基数 = %d，期望 2（不得是超集）", len(got))
	}

	// c11 → null 且 residue_bytes > 0（§4.5 的"两者同真"）
	c11, found11 := searchFixture(t, "c11-real-change")
	if found11 || c11 != nil {
		t.Fatalf("c11-real-change：期望无解，实得 %v", c11)
	}
	if res := residueOf(t, "c11-real-change"); res <= 0 {
		t.Fatalf("c11-real-change：无解但 residue_bytes = %d，期望 > 0（两者必须同真）", res)
	}

	// c12 → null（真改不可归一化）；且 T1/T3/T9 已把元数据差异抹平，
	// 残留只来自 table.csv 的 1 字节。
	c12, found12 := searchFixture(t, "c12-mixed")
	if found12 || c12 != nil {
		t.Fatalf("c12-mixed：期望无解，实得 %v", c12)
	}
	if res := residueOf(t, "c12-mixed"); res != 1 {
		t.Fatalf("c12-mixed：residue_bytes = %d，期望 1", res)
	}
	diffPaths, diffBytes := residualDetail(t, "c12-mixed")
	if len(diffPaths) != 1 || diffPaths[0] != "synthpkg/data/table.csv" || diffBytes != 1 {
		t.Fatalf("c12-mixed：归一化后残留 = %v / %d 字节，期望只有 synthpkg/data/table.csv 的 1 字节", diffPaths, diffBytes)
	}
}

// minimizeGolden 是 data/golden/minimize/*.json 的字段序（= JSON 键序）。
// found=false 时 MinimalCauseSet 为 null；digest_set 取 T1–T11（证明无解时摘要仍不等）。
type minimizeGolden struct {
	Case            string   `json:"case"`
	MinimalCauseSet []string `json:"minimal_cause_set"`
	Found           bool     `json:"found"`
	ResidueBytes    int64    `json:"residue_bytes"`
	DigestSet       []string `json:"digest_set"`
	DigestLeft      string   `json:"digest_left"`
	DigestRight     string   `json:"digest_right"`
}

// TestMinimizeGolden 是 data/golden/minimize/*.json 的逐字节断言。
// 生成：go test ./internal/cli -run TestMinimizeGolden -update
func TestMinimizeGolden(t *testing.T) {
	root := filepath.Join("..", "..")
	goldenDir := filepath.Join(root, "data", "golden", "minimize")
	if *update {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range fixtureCases {
		t.Run(name, func(t *testing.T) {
			want := minimizeGoldenOf(t, name)
			buf, err := report.MarshalValue(want)
			if err != nil {
				t.Fatal(err)
			}
			gp := filepath.Join(goldenDir, name+".json")
			if *update {
				if err := os.WriteFile(gp, buf, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			got, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("读 golden 失败（用 -update 生成）: %v", err)
			}
			if !bytes.Equal(buf, got) {
				t.Fatalf("与 golden 不一致\n got=%s\nwant=%s", buf, got)
			}
			// 双跑逐字节（确定性）。
			buf2, err := report.MarshalValue(minimizeGoldenOf(t, name))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(buf, buf2) {
				t.Fatal("同输入双跑不一致")
			}
		})
	}
}

func minimizeGoldenOf(t *testing.T, name string) minimizeGolden {
	t.Helper()
	le := entriesOf(t, mustFixture(t, name, "left"))
	re := entriesOf(t, mustFixture(t, name, "right"))
	set, found, err := minimize.Search(
		func(s []string) string { return normalize.TreeDigest(s, le) },
		func(s []string) string { return normalize.TreeDigest(s, re) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if found && set == nil {
		t.Fatal("found=true 时 set 不得为 nil")
	}
	digestSet := set
	if !found {
		digestSet = normalizationRules()
	}
	return minimizeGolden{
		Case:            name,
		MinimalCauseSet: set,
		Found:           found,
		ResidueBytes:    residueOf(t, name),
		DigestSet:       digestSet,
		DigestLeft:      normalize.TreeDigest(digestSet, le),
		DigestRight:     normalize.TreeDigest(digestSet, re),
	}
}

// BenchmarkMinimizeSearch 实测最小原因集搜索耗时（回填 `plan/00` §八；**不进断言**）。
// 两个代表：早期命中（c01：候选 ∅/{T1}）与无解全扫（c12：4096 个候选 × 两侧摘要）。
func BenchmarkMinimizeSearch(b *testing.B) {
	for _, name := range []string{"c01-mtime", "c12-mixed"} {
		b.Run(name, func(b *testing.B) {
			le := entriesOf(b, mustFixture(b, name, "left"))
			re := entriesOf(b, mustFixture(b, name, "right"))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, err := minimize.Search(
					func(s []string) string { return normalize.TreeDigest(s, le) },
					func(s []string) string { return normalize.TreeDigest(s, re) },
				); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// searchFixture 返回 expect.json 同名样张的最小原因集。
func searchFixture(t *testing.T, name string) ([]string, bool) {
	t.Helper()
	le := entriesOf(t, mustFixture(t, name, "left"))
	re := entriesOf(t, mustFixture(t, name, "right"))
	set, found, err := minimize.Search(
		func(s []string) string { return normalize.TreeDigest(s, le) },
		func(s []string) string { return normalize.TreeDigest(s, re) },
	)
	if err != nil {
		t.Fatalf("%s：Search 报错: %v", name, err)
	}
	return set, found
}

// residueOf 走真实 attribute 路径取 residue_bytes（不由 expect.json 抄）。
func residueOf(t *testing.T, name string) int64 {
	t.Helper()
	left, right := sidesOf(t, name)
	res, err := attribute.Attribute(left, right)
	if err != nil {
		t.Fatalf("%s：attribute 失败: %v", name, err)
	}
	return res.ResidueBytes
}

// residualDetail 返回 S=T1–T11 归一化后仍不同的路径（升序）与不同字节总数。
func residualDetail(t *testing.T, name string) ([]string, int64) {
	t.Helper()
	ops := normalizationRules()
	le := normalize.Apply(ops, entriesOf(t, mustFixture(t, name, "left")))
	re := normalize.Apply(ops, entriesOf(t, mustFixture(t, name, "right")))
	lm := map[string]normalize.Entry{}
	for _, e := range le {
		lm[e.Path] = e
	}
	rm := map[string]normalize.Entry{}
	for _, e := range re {
		rm[e.Path] = e
	}
	var paths []string
	var total int64
	for _, p := range unionPathKeys(lm, rm) {
		l, lok := lm[p]
		r, rok := rm[p]
		var n int64
		switch {
		case lok && rok:
			n = differentBytes(normalize.EntryStream(l), normalize.EntryStream(r))
		case lok:
			n = int64(len(normalize.EntryStream(l)))
		default:
			n = int64(len(normalize.EntryStream(r)))
		}
		if n > 0 {
			paths = append(paths, p)
			total += n
		}
	}
	return paths, total
}

func differentBytes(a, b []byte) int64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var d int64
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			d++
		}
	}
	if len(a) > len(b) {
		d += int64(len(a) - len(b))
	} else {
		d += int64(len(b) - len(a))
	}
	return d
}

func unionPathKeys(l, r map[string]normalize.Entry) []string {
	seen := map[string]bool{}
	for k := range l {
		seen[k] = true
	}
	for k := range r {
		seen[k] = true
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sidesOf 装载一对样张并构造 attribute 输入。
func sidesOf(t *testing.T, name string) (attribute.SideInput, attribute.SideInput) {
	t.Helper()
	build := func(side string) attribute.SideInput {
		doc, err := artifact.LoadDocument(mustFixture(t, name, side), "", 0)
		if err != nil {
			t.Fatalf("%s/%s 装载失败: %v", name, side, err)
		}
		s, err := buildAttributeSide(doc)
		if err != nil {
			t.Fatalf("%s/%s facts 失败: %v", name, side, err)
		}
		return s
	}
	return build("left"), build("right")
}

func mustFixture(t testing.TB, name, side string) string {
	t.Helper()
	left, right := fixtureArtifacts(t, name)
	if side == "right" {
		return right
	}
	return left
}

// normalizationRules 返回 T1–T11（§4.5 的 residue / normalized_all 口径）。
func normalizationRules() []string {
	out := make([]string, 0, len(ruleIDs())-1)
	for _, r := range ruleIDs() {
		if r != "T12" {
			out = append(out, r)
		}
	}
	return out
}
