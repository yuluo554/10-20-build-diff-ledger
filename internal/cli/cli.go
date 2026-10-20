// Package cli 是 CLI 的唯一入口与退出码的唯一出口。
//
// 纪律（plan/02 §四）：子命令返回 (exitCode, error)，由 Run 统一映射；
// 深层包禁止调用 os.Exit（否则测试无法断言、退出码会散成多处）。
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/align"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/artifact"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/attribute"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/facts"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/minimize"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/normalize"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/report"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/version"
)

// 退出码语义（data/README.md §4.6）。
const (
	ExitOK       = 0 // 完全一致（或 budget 放行且归一化后一致）
	ExitUsage    = 1 // 用法错误 / 输入不可解析
	ExitMetaOnly = 2 // 仅元数据差异
	ExitResidue  = 3 // 有残留
)

// Run 执行一次 CLI 调用，返回进程退出码。
// args 不含程序名（调用方传 os.Args[1:]）。
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return ExitUsage
	}
	cmd, rest := args[0], args[1:]
	var code int
	var err error
	switch cmd {
	case "align":
		code, err = runAlign(rest, stdout)
	case "facts":
		code, err = runFacts(rest, stdout)
	case "compare":
		code, err = runCompare(rest, stdout)
	case "version":
		fmt.Fprintf(stdout, "bdiff %s\n", version.Version)
		return ExitOK
	case "selftest":
		code, err = runSelftest(stdout)
	case "help", "-h", "--help":
		usage(stdout)
		return ExitOK
	default:
		fmt.Fprintf(stderr, "未知子命令 %q\n\n", cmd)
		usage(stderr)
		return ExitUsage
	}
	if err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		if code == 0 {
			code = ExitUsage
		}
	}
	return code
}

func usage(w io.Writer) {
	fmt.Fprint(w, `bdiff — 构建产物差异归因账本

用法：
  bdiff align    --left L --right R [--format json|md] [--layer-kind=oci]
  bdiff facts    --artifact A [--format json]
  bdiff compare  --left L --right R [--out DIR] [--format json|md|both]
                 [--budget T1,T2|all] [--fail-on any|residue] [--layer-kind=oci]
  bdiff version
  bdiff selftest

退出码：0 完全一致 / 1 用法或输入不可解析 / 2 仅元数据差异 / 3 有残留
`)
}

// newFlagSet 建一个不会自己 os.Exit 的 FlagSet（用法错误统一映射为 ExitUsage）。
func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func checkFormat(v string, allowed ...string) error {
	for _, a := range allowed {
		if v == a {
			return nil
		}
	}
	return fmt.Errorf("--format 只接受 %s（收到 %q）", strings.Join(allowed, " / "), v)
}

func requirePaths(left, right string) error {
	if left == "" || right == "" {
		return errors.New("必须同时给 --left 与 --right")
	}
	for _, p := range []string{left, right} {
		if _, err := os.Stat(p); err != nil {
			// 不回显绝对路径：只报 basename + 原因
			return fmt.Errorf("无法读取 %q：路径不存在或不可访问", filepath.Base(p))
		}
	}
	return nil
}

func runAlign(args []string, stdout io.Writer) (int, error) {
	var left, right, format, layerKind string
	var maxEntryBytes int64
	fs := newFlagSet("align", os.Stderr)
	fs.StringVar(&left, "left", "", "左侧产物路径")
	fs.StringVar(&right, "right", "", "右侧产物路径")
	fs.StringVar(&format, "format", "json", "输出格式：json|md")
	fs.StringVar(&layerKind, "layer-kind", "", "容器提示：oci")
	fs.Int64Var(&maxEntryBytes, "max-entry-bytes", artifact.DefaultMaxEntryBytes, "单条目上限（字节）")
	if err := fs.Parse(args); err != nil {
		return ExitUsage, nil // flag 已把错误写进 stderr
	}
	if err := checkFormat(format, "json", "md"); err != nil {
		return ExitUsage, err
	}
	if err := requirePaths(left, right); err != nil {
		return ExitUsage, err
	}

	leftRaws, leftKind, err := artifact.Load(left, layerKind, maxEntryBytes)
	if err != nil {
		return ExitUsage, err
	}
	rightRaws, rightKind, err := artifact.Load(right, layerKind, maxEntryBytes)
	if err != nil {
		return ExitUsage, err
	}
	res, err := align.Align(leftRaws, rightRaws)
	if err != nil {
		return ExitUsage, err
	}
	res.Left.Kind = string(leftKind)
	res.Right.Kind = string(rightKind)

	if format == "md" {
		if _, err := stdout.Write(align.RenderMarkdown(res)); err != nil {
			return ExitUsage, err
		}
		return ExitOK, nil
	}
	out, err := report.MarshalValue(res)
	if err != nil {
		return ExitUsage, err
	}
	if _, err := stdout.Write(out); err != nil {
		return ExitUsage, err
	}
	return ExitOK, nil
}

func runFacts(args []string, stdout io.Writer) (int, error) {
	var artifactPath, format, layerKind string
	var maxEntryBytes int64
	fs := newFlagSet("facts", os.Stderr)
	fs.StringVar(&artifactPath, "artifact", "", "待抽取的产物路径")
	fs.StringVar(&format, "format", "json", "输出格式：json")
	fs.StringVar(&layerKind, "layer-kind", "", "容器提示：oci")
	fs.Int64Var(&maxEntryBytes, "max-entry-bytes", artifact.DefaultMaxEntryBytes, "单条目上限（字节）")
	if err := fs.Parse(args); err != nil {
		return ExitUsage, nil
	}
	if err := checkFormat(format, "json"); err != nil {
		return ExitUsage, err
	}
	if artifactPath == "" {
		return ExitUsage, errors.New("必须给 --artifact")
	}
	if _, err := os.Stat(artifactPath); err != nil {
		return ExitUsage, fmt.Errorf("无法读取 %q：路径不存在或不可访问", filepath.Base(artifactPath))
	}
	doc, err := artifact.LoadDocument(artifactPath, layerKind, maxEntryBytes)
	if err != nil {
		return ExitUsage, err
	}
	out, err := facts.Build(doc)
	if err != nil {
		return ExitUsage, err
	}
	buf, err := report.MarshalValue(out)
	if err != nil {
		return ExitUsage, err
	}
	if _, err := stdout.Write(buf); err != nil {
		return ExitUsage, err
	}
	return ExitOK, nil
}

func runCompare(args []string, stdout io.Writer) (int, error) {
	var left, right, out, format, budgetRaw, failOnRaw, layerKind string
	fs := newFlagSet("compare", os.Stderr)
	fs.StringVar(&left, "left", "", "左侧产物路径")
	fs.StringVar(&right, "right", "", "右侧产物路径")
	fs.StringVar(&out, "out", "", "输出目录（写 report.json / report.md）")
	fs.StringVar(&format, "format", "json", "stdout 格式：json|md|both")
	fs.StringVar(&budgetRaw, "budget", "", "CI 允许的差异类别：T1,T2 或 all")
	fs.StringVar(&failOnRaw, "fail-on", "any", "失败判据：any|residue")
	fs.StringVar(&layerKind, "layer-kind", "", "容器提示：oci")
	if err := fs.Parse(args); err != nil {
		return ExitUsage, nil
	}
	if err := checkFormat(format, "json", "md", "both"); err != nil {
		return ExitUsage, err
	}
	// 参数面校验排在 I/O 之前：非法值不必等读完文件才发现。
	budget, err := report.ParseBudget(budgetRaw)
	if err != nil {
		return ExitUsage, err
	}
	failOn, err := parseFailOn(failOnRaw)
	if err != nil {
		return ExitUsage, err
	}
	if err := requirePaths(left, right); err != nil {
		return ExitUsage, err
	}

	rep, err := buildReport(left, right, layerKind)
	if err != nil {
		return ExitUsage, err
	}
	jsonBuf, err := report.MarshalCanonical(rep)
	if err != nil {
		return ExitUsage, err
	}
	mdBuf := report.RenderMarkdown(rep)

	if out != "" {
		if err := writeReportDir(out, jsonBuf, mdBuf); err != nil {
			return ExitUsage, err
		}
	}
	switch format {
	case "json":
		if _, err := stdout.Write(jsonBuf); err != nil {
			return ExitUsage, err
		}
	case "md":
		if _, err := stdout.Write(mdBuf); err != nil {
			return ExitUsage, err
		}
	case "both":
		// 未给 --out 时，stdout 依次是 report.json、空行、report.md（顺序固定、可断言）。
		if _, err := stdout.Write(jsonBuf); err != nil {
			return ExitUsage, err
		}
		if _, err := io.WriteString(stdout, "\n"); err != nil {
			return ExitUsage, err
		}
		if _, err := stdout.Write(mdBuf); err != nil {
			return ExitUsage, err
		}
	}
	return compareExitCode(rep, budget, failOn), nil
}

// writeReportDir 落盘 report.json + report.md（`\n` 结尾，0644）。
// 任何写盘失败都必须冒泡成 exit 1——静默丢弃报告会让 CI 门"绿得没有依据"。
func writeReportDir(dir string, jsonBuf, mdBuf []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("无法创建输出目录 %q：写盘失败", filepath.Base(dir))
	}
	files := []struct {
		name string
		data []byte
	}{
		{"report.json", jsonBuf},
		{"report.md", mdBuf},
	}
	for _, f := range files {
		p := filepath.Join(dir, f.name)
		if err := os.WriteFile(p, f.data, 0o644); err != nil {
			return fmt.Errorf("无法写入 %q：写盘失败", f.name)
		}
	}
	return nil
}

// buildAttributeResult 是"装载 → facts → attribute"的唯一路径。
// buildReport 与 M3 的 attribute golden 都走它——golden 因此不随 compare 的输出形态变化而失效。
func buildAttributeResult(leftPath, rightPath, layerKind string) (*attribute.Result, []normalize.Entry, []normalize.Entry, artifact.Document, artifact.Document, error) {
	leftDoc, err := artifact.LoadDocument(leftPath, layerKind, 0)
	if err != nil {
		return nil, nil, nil, artifact.Document{}, artifact.Document{}, err
	}
	rightDoc, err := artifact.LoadDocument(rightPath, layerKind, 0)
	if err != nil {
		return nil, nil, nil, artifact.Document{}, artifact.Document{}, err
	}
	leftSide, err := buildAttributeSide(leftDoc)
	if err != nil {
		return nil, nil, nil, artifact.Document{}, artifact.Document{}, err
	}
	rightSide, err := buildAttributeSide(rightDoc)
	if err != nil {
		return nil, nil, nil, artifact.Document{}, artifact.Document{}, err
	}
	res, err := attribute.Attribute(leftSide, rightSide)
	if err != nil {
		return nil, nil, nil, artifact.Document{}, artifact.Document{}, err
	}
	leftEntries, err := attribute.Entries(leftSide)
	if err != nil {
		return nil, nil, nil, artifact.Document{}, artifact.Document{}, err
	}
	rightEntries, err := attribute.Entries(rightSide)
	if err != nil {
		return nil, nil, nil, artifact.Document{}, artifact.Document{}, err
	}
	return res, leftEntries, rightEntries, leftDoc, rightDoc, nil
}

// buildReport 把装载 → facts → attribute → minimize → align 串成一份报告。
// 两侧的 normalize.Entry 只构造一次，摘要闭包复用（HANDOFF-M5 §二.6）。
func buildReport(leftPath, rightPath, layerKind string) (*report.Report, error) {
	res, leftEntries, rightEntries, leftDoc, rightDoc, err := buildAttributeResult(leftPath, rightPath, layerKind)
	if err != nil {
		return nil, err
	}
	set, found, err := minimize.Search(
		func(s []string) string { return normalize.TreeDigest(s, leftEntries) },
		func(s []string) string { return normalize.TreeDigest(s, rightEntries) },
	)
	if err != nil {
		return nil, err
	}

	leftSHA, err := sideSHA256(leftPath, leftDoc)
	if err != nil {
		return nil, err
	}
	rightSHA, err := sideSHA256(rightPath, rightDoc)
	if err != nil {
		return nil, err
	}

	aligned, err := align.Align(leftDoc.Entries, rightDoc.Entries)
	if err != nil {
		return nil, err
	}
	renamed, added, removed := splitAlignEntries(aligned.Entries)

	residuals := attribute.Residuals(leftEntries, rightEntries)
	items := make([]report.ResidualItem, 0, len(residuals))
	for _, d := range residuals {
		items = append(items, report.ResidualItem{
			Path: d.Path, RuleID: d.RuleID, BytesDifferent: d.BytesDelta,
		})
	}

	return report.Build(report.Input{
		Left:                 report.Side{Basename: filepath.Base(leftPath), SHA256: leftSHA},
		Right:                report.Side{Basename: filepath.Base(rightPath), SHA256: rightSHA},
		Counts:               res.Counts.Values,
		Renamed:              renamed,
		Added:                added,
		Removed:              removed,
		ResidueBytes:         res.ResidueBytes,
		MinimalCauseSet:      set,
		MinimalCauseSetFound: found,
		ResidualItems:        items,
		Notes:                res.Notes,
	})
}

// splitAlignEntries 把对齐结果折成报告的三张清单（顺序固定：改名按 to、增删按 relpath）。
func splitAlignEntries(entries []model.Entry) (renamed []report.RenamedItem, added, removed []string) {
	renamed = []report.RenamedItem{}
	added = []string{}
	removed = []string{}
	for _, e := range entries {
		switch e.Status {
		case model.StatusRenamed:
			renamed = append(renamed, report.RenamedItem{From: e.RenamedFrom, To: e.RelPath})
		case model.StatusAdded:
			added = append(added, e.RelPath)
		case model.StatusRemoved:
			removed = append(removed, e.RelPath)
		}
	}
	sort.Slice(renamed, func(i, j int) bool { return renamed[i].To < renamed[j].To })
	sort.Strings(added)
	sort.Strings(removed)
	return renamed, added, removed
}

// sideSHA256 是报告里 `left.sha256` / `right.sha256` 的唯一算法（data/README.md §4.8）：
//   - 文件输入：该文件字节的 sha256
//   - 目录输入：条目表 `relpath ‖ 0x00 ‖ sha256 ‖ 0x0A`（relpath 升序）的 sha256
//
// 不读绝对路径、不读时钟，因此同内容同摘要。
func sideSHA256(path string, doc artifact.Document) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("无法读取 %q：路径不存在或不可访问", filepath.Base(path))
	}
	if !info.IsDir() {
		buf, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("无法读取 %q：读盘失败", filepath.Base(path))
		}
		sum := sha256.Sum256(buf)
		return hex.EncodeToString(sum[:]), nil
	}
	entries := append([]artifact.Raw(nil), doc.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	var b strings.Builder
	for _, e := range entries {
		sum := sha256.Sum256(e.Data)
		b.WriteString(e.Path)
		b.WriteByte(0x00)
		b.WriteString(hex.EncodeToString(sum[:]))
		b.WriteByte(0x0A)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:]), nil
}

// buildAttributeSide 把一份已装载文档转成 attribute 的输入：
// 容器级事实（path=""）+ 每个条目的 facts 与内容字节。
func buildAttributeSide(doc artifact.Document) (attribute.SideInput, error) {
	side := attribute.SideInput{Kind: string(doc.Kind)}
	if cf := facts.ExtractContainer(doc.Container); len(cf) > 0 {
		side.Entries = append(side.Entries, attribute.EntryInput{
			Path: "", Index: -1, Facts: factMap(cf),
		})
	}
	for _, raw := range doc.Entries {
		fs, err := facts.Extract(raw)
		if err != nil {
			return attribute.SideInput{}, err
		}
		side.Entries = append(side.Entries, attribute.EntryInput{
			Path:  raw.Path,
			Kind:  model.Kind(raw.Kind),
			Index: raw.Index,
			Data:  raw.Data,
			Facts: factMap(fs),
		})
	}
	return side, nil
}

func factMap(fs []model.Fact) map[string]string {
	m := make(map[string]string, len(fs))
	for _, f := range fs {
		m[f.Key] = f.Value
	}
	return m
}

// compareExitCode / budgetCovers / parseFailOn 见 exitcode.go（退出码唯一实现）。

// runSelftest 是**离线**自检：不读样张、不联网、不写盘。
// 它验证的是"契约形状"而不是"分类正确"——后者由 golden 测试承担。
func runSelftest(stdout io.Writer) (int, error) {
	fail := 0
	check := func(name string, ok bool) {
		status := "PASS"
		if !ok {
			status = "FAIL"
			fail++
		}
		fmt.Fprintf(stdout, "%s  %s\n", status, name)
	}

	check("version 非空", version.Version != "")
	check("schema_version >= 1", version.SchemaVersion >= 1)
	check("规则全集恰好 12 条", len(reportRuleIDs()) == 12)
	check("T12 在规则全集末尾", lastRule() == "T12")
	check("最小集上界 == 规则数", minimize.MaxSetSize == len(reportRuleIDs()))

	combos := minimize.Combinations()
	// 2^12 = 4096 个子集
	check("子集枚举总数 == 2^12", len(combos) == 4096)
	check("首个候选为空集", len(combos) > 0 && len(combos[0]) == 0)
	check("最后候选为全集", len(combos) == 4096 && len(combos[4095]) == 12)
	check("同基数按字典序（首两个单元素为 T1,T2）",
		len(combos) > 2 && combos[1][0] == "T1" && combos[2][0] == "T2")

	for _, c := range []struct {
		in   string
		want int
		ok   bool
	}{
		{"", 0, true},
		{"all", 12, true},
		{"T1", 1, true},
		{"T1,T2", 2, true},
		{"T1, T2 ,T2", 2, true}, // 去重 + 去空白
		{"T13", 0, false},
		{"mtime", 0, false},
	} {
		got, err := report.ParseBudget(c.in)
		if c.ok {
			check(fmt.Sprintf("ParseBudget(%q) 成功且长度 %d", c.in, c.want), err == nil && len(got) == c.want)
		} else {
			check(fmt.Sprintf("ParseBudget(%q) 必须拒绝", c.in), err != nil)
		}
	}

	// 树摘要：空集合 + 空条目 → 稳定常量；同一输入双跑一致
	d1 := digestOf(nil, nil)
	d2 := digestOf(nil, nil)
	check("空输入树摘要稳定", d1 == d2 && len(d1) == 64)

	if fail > 0 {
		return ExitUsage, fmt.Errorf("selftest 有 %d 项失败", fail)
	}
	fmt.Fprintln(stdout, "selftest: 全部通过（离线、不读样张）")
	return ExitOK, nil
}
