// gen_fixtures 是样张生成器（plan/04 §三）。
//
// 纪律（全部由测试与布局门守）：
//  1. **不读系统时钟、不读环境变量**：唯一输入是 --out / --golden / --only / --check。
//     时间戳只从 TimeTable{0, 1700000000, 1700003600} 取；
//  2. **不真跑构建**：所有容器（tar / gzip / zip / ELF）都由本包手写字节流合成，
//     头部字段逐一显式赋值，不依赖标准库 writer 的默认值；
//  3. **部分运行不改全集清单**：--only 只写该 case 的目录，绝不写 manifest.json（K11）；
//  4. **--check 不落临时目录**：全量生成到内存 → 与入仓树逐字节比对，
//     一致打印 "13 cases, 0 diff" 并退出 0。
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	out := flag.String("out", "data/fixtures", "样张输出目录")
	only := flag.String("only", "", "只生成指定 case（不写全集清单）")
	check := flag.Bool("check", false, "校验模式：在内存里重生成，与入仓样张逐字节比对")
	flag.Parse()

	if err := run(os.Stdout, *out, *only, *check); err != nil {
		fmt.Fprintf(os.Stderr, "gen_fixtures: %v\n", err)
		os.Exit(1)
	}
}

// run 是 main 的可测试入口（stdout 由调用方给，便于测试断言输出）。
func run(stdout io.Writer, out, only string, check bool) error {
	if only != "" && !knownCase(only) {
		return fmt.Errorf("未知 case %q（合法值见 plan/04 §二）", only)
	}
	if check {
		return runCheck(stdout, out, only)
	}

	tree, err := buildTree(only)
	if err != nil {
		return err
	}
	if err := writeTree(out, tree); err != nil {
		return err
	}
	if only == "" {
		fmt.Fprintf(stdout, "gen_fixtures: %d 组样张 + %d 组 align golden 输入 → %s\n",
			len(caseDefs()), len(goldenPairs()), out)
	} else {
		fmt.Fprintf(stdout, "gen_fixtures: 局部生成 %s → %s（不写全集清单）\n", only, out)
	}
	return nil
}

func runCheck(stdout io.Writer, out, only string) error {
	tree, err := buildTree(only)
	if err != nil {
		return err
	}
	diffs, err := compareTree(out, tree)
	if err != nil {
		return err
	}
	nCases := len(caseDefs())
	if only != "" {
		nCases = 1
	}
	fmt.Fprintf(stdout, "%d cases, %d diff\n", nCases, len(diffs))
	if len(diffs) > 0 {
		for _, d := range diffs {
			fmt.Fprintf(stdout, "  DIFF %s\n", d)
		}
		return fmt.Errorf("样张与生成器输出不一致（%d 处）", len(diffs))
	}
	return nil
}

func knownCase(name string) bool {
	for _, c := range caseDefs() {
		if c.Name == name {
			return true
		}
	}
	return false
}

// buildTree 在内存里生成完整输出树（相对路径 → 字节）。
// only != "" 时只生成该 case，且不含 manifest.json。
func buildTree(only string) (map[string][]byte, error) {
	tree := make(map[string][]byte)

	for _, c := range caseDefs() {
		if only != "" && c.Name != only {
			continue
		}
		left, err := c.render(c.Left)
		if err != nil {
			return nil, fmt.Errorf("%s（左侧）：%v", c.Name, err)
		}
		right, err := c.render(c.Right)
		if err != nil {
			return nil, fmt.Errorf("%s（右侧）：%v", c.Name, err)
		}
		base := c.Name
		tree[base+"/pair/left/"+c.artifactName()] = left
		tree[base+"/pair/right/"+c.artifactName()] = right
		tree[base+"/expect.json"] = renderExpect(c)
		tree[base+"/README.md"] = renderReadme(c)
	}

	if only == "" {
		// align golden 的输入对（目录形态）：不属于 13 组分类样张。
		for _, p := range goldenPairs() {
			base := "align-golden/" + p.Name
			if err := putDirTree(tree, base+"/pair/left", p.Left); err != nil {
				return nil, err
			}
			if err := putDirTree(tree, base+"/pair/right", p.Right); err != nil {
				return nil, err
			}
			tree[base+"/README.md"] = renderGoldenPairReadme(p)
		}

		// 读不了的输入（反例样张，data/README.md §三）：截断的 tar。
		tree["broken/truncated.tar"] = truncatedTar()
		tree["broken/README.md"] = []byte("# broken · 读不了的输入\n\n" +
			"`truncated.tar` 是一个声明 100 字节内容、实际只有 10 字节的 tar。\n" +
			"期望：`bdiff align/compare --left broken/truncated.tar ...` 必须 **exit 1**（用法或输入不可解析），\n" +
			"不得当成\"一致\"（静默通过的经典漏洞，见 data/README.md §4.6）。\n\n" +
			"手工复现：\n\n```bash\ntar -tvf data/fixtures/broken/truncated.tar   # 报 \"Unexpected EOF\"\n```\n")

		tree["manifest.json"] = renderManifest(tree)
	}
	return tree, nil
}

// putDirTree 把逻辑树写成一个目录形态的样张（align golden 用）。
func putDirTree(tree map[string][]byte, base string, files []fileSpec) error {
	for _, f := range files {
		switch f.Kind {
		case kindDir:
			tree[base+"/"+f.Path+"/.keep"] = nil
		case kindSymlink:
			return fmt.Errorf("align golden 目前不支持软链条目：%q", f.Path)
		default:
			tree[base+"/"+f.Path] = f.Data
		}
	}
	return nil
}

// truncatedTar 造一个"读不了的输入"：声明 100 字节、实际 10 字节。
func truncatedTar() []byte {
	blk, err := encodeTarHeader(tarHeader{
		Name: "a.txt", Mode: 0o644, Size: 100, MTime: 1700000000,
		Typeflag: '0', Uname: "builder", Gname: "builder",
	})
	if err != nil {
		panic(err) // 常量输入，不可能失败
	}
	return append(blk, []byte("0123456789")...)
}

// renderExpect 产出 expect.json：只含 inputs 与 expect（plan/04 §四）。
func renderExpect(c caseDef) []byte {
	doc := struct {
		Case      string         `json:"case"`
		Title     string         `json:"title"`
		Container string         `json:"container"`
		Injected  []string       `json:"injected"`
		Inputs    map[string]any `json:"inputs"`
		Expect    expectBody     `json:"expect"`
	}{
		Case:      c.Name,
		Title:     c.Title,
		Container: c.Format,
		Injected:  c.Injected,
		Inputs:    map[string]any{"left": sideInputs(c.Left), "right": sideInputs(c.Right)},
		Expect:    c.Expect,
	}
	return marshalJSON(doc)
}

// sideInputs 是一侧的注入参数（全部转成字符串，便于逐字段手工核对）。
func sideInputs(s sideSpec) map[string]string {
	m := map[string]string{
		"prefix":            s.Prefix,
		"mtime":             fmt.Sprintf("%d", s.MTime),
		"uid":               fmt.Sprintf("%d", s.UID),
		"gid":               fmt.Sprintf("%d", s.GID),
		"uname":             s.Uname,
		"gname":             s.Gname,
		"file_mode":         fmt.Sprintf("0%o", s.FileMode),
		"dir_mode":          fmt.Sprintf("0%o", s.DirMode),
		"entry_order":       "listed",
		"gzip_xfl":          fmt.Sprintf("%d", s.GzipXFL),
		"gzip_os":           fmt.Sprintf("%d", s.GzipOS),
		"zip_create_system": fmt.Sprintf("%d", s.Zip.CreateSystem),
		"elf_comment":       s.ELF.Comment,
		"elf_producer":      s.ELF.Producer,
		"elf_comp_dir":      s.ELF.CompDir,
		"elf_build_id":      hexOf(s.ELF.BuildID),
	}
	if s.Reverse {
		m["entry_order"] = "reversed"
	}
	if s.ELF.Signature == nil {
		m["elf_signature"] = "absent"
	} else {
		m["elf_signature"] = hexOf(s.ELF.Signature)
	}
	if len(s.Overrides) > 0 {
		paths := make([]string, 0, len(s.Overrides))
		for p := range s.Overrides {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		m["content_overrides"] = strings.Join(paths, ",")
	}
	return m
}

// hexOf 把字节原样转成 lowercase hex（build-id / 签名段的注入值就是这些字节本身，
// 不是它们的 sha256——写成摘要会让 expect.json 的 inputs 无法直接对照归档）。
func hexOf(b []byte) string { return hex.EncodeToString(b) }

// renderReadme 产出该 case 的 README（注入 / 期望 / 手工复现）。
func renderReadme(c caseDef) []byte {
	var b strings.Builder
	name := c.artifactName()
	fmt.Fprintf(&b, "# %s · %s\n\n", c.Name, c.Title)
	b.WriteString("> 由 `tools/gen_fixtures` 程序化生成，**禁止手改**。\n")
	b.WriteString("> 重建校验：`go run ./tools/gen_fixtures --out data/fixtures --check`（应输出 `13 cases, 0 diff`）。\n\n")

	b.WriteString("## 注入的差异\n\n")
	fmt.Fprintf(&b, "- 容器：`%s`\n- 注入类别：%s\n", c.Format, strings.Join(c.Injected, "、"))
	b.WriteString("- 两侧参数：见 `expect.json` 的 `inputs`\n\n")

	b.WriteString("## 期望\n\n")
	fmt.Fprintf(&b, "- `counts_positive`：%s\n", strings.Join(c.Expect.CountsPositive, "、"))
	if c.Expect.MinimalCauseSet == nil {
		b.WriteString("- `minimal_cause_set`：`null`（§4.5：真改不可归一化，任何 S 都无法让树摘要相等）\n")
	} else {
		fmt.Fprintf(&b, "- `minimal_cause_set`：`[%s]`\n", strings.Join(c.Expect.MinimalCauseSet, ", "))
	}
	fmt.Fprintf(&b, "- `residue_bytes`：%d\n- 退出码：%d\n\n", c.Expect.ResidueBytes, c.Expect.ExitCode)

	b.WriteString("## 手工复现\n\n```bash\n")
	fmt.Fprintf(&b, "cd data/fixtures/%s\n", c.Name)
	fmt.Fprintf(&b, "sha256sum pair/left/%s pair/right/%s\n", name, name)
	switch c.Format {
	case "tar":
		fmt.Fprintf(&b, "tar -tvf pair/left/%s\n", name)
		fmt.Fprintf(&b, "tar -tvf pair/right/%s\n", name)
	case "tar.gz":
		fmt.Fprintf(&b, "tar -tzvf pair/left/%s\n", name)
		fmt.Fprintf(&b, "tar -tzvf pair/right/%s\n", name)
	case "zip":
		fmt.Fprintf(&b, "unzip -l pair/left/%s\n", name)
		fmt.Fprintf(&b, "unzip -l pair/right/%s\n", name)
	}
	if c.Manual != "" {
		b.WriteString(c.Manual + "\n")
	}
	b.WriteString("```\n")
	return []byte(b.String())
}

func renderGoldenPairReadme(p goldenPair) []byte {
	var b strings.Builder
	b.WriteString("# align golden 输入 · " + p.Name + "\n\n")
	b.WriteString("> 由 `tools/gen_fixtures` 程序化生成，**禁止手改**。\n\n")
	b.WriteString("两侧是**目录形态**的产物（不是归档），用于 `data/golden/align/` 的逐字节比对。\n\n")
	b.WriteString("手工复现条目表：\n\n```bash\n")
	fmt.Fprintf(&b, "find data/fixtures/align-golden/%s/pair/left -type f | sort\n", p.Name)
	fmt.Fprintf(&b, "find data/fixtures/align-golden/%s/pair/right -type f | sort\n", p.Name)
	fmt.Fprintf(&b, "cd data/fixtures/align-golden/%s/pair/left && find . -type f -exec sha256sum {} + | sort\n", p.Name)
	b.WriteString("```\n")
	return []byte(b.String())
}

// renderManifest 是全集清单：只在"无 --only"的全量运行里写盘（K11）。
func renderManifest(tree map[string][]byte) []byte {
	paths := make([]string, 0, len(tree))
	for p := range tree {
		if p == "manifest.json" {
			continue
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)

	type file struct {
		Path   string `json:"path"`
		Size   int    `json:"size"`
		SHA256 string `json:"sha256"`
	}
	type c struct {
		Name  string `json:"name"`
		Files []file `json:"files"`
	}
	cases := make([]c, 0, len(caseDefs()))
	for _, def := range caseDefs() {
		files := []file{}
		for _, p := range paths {
			if !strings.HasPrefix(p, def.Name+"/") {
				continue
			}
			files = append(files, file{Path: p, Size: len(tree[p]), SHA256: sha256Hex(tree[p])})
		}
		cases = append(cases, c{Name: def.Name, Files: files})
	}
	extra := []string{}
	for _, p := range paths {
		if strings.HasPrefix(p, "align-golden/") || strings.HasPrefix(p, "broken/") {
			extra = append(extra, p)
		}
	}
	doc := struct {
		SchemaVersion int      `json:"schema_version"`
		Generator     string   `json:"generator"`
		Cases         []c      `json:"cases"`
		Extra         []string `json:"extra"` // 非分类样张：align golden 输入 + 反例
	}{SchemaVersion: 1, Generator: "tools/gen_fixtures", Cases: cases, Extra: extra}
	return marshalJSON(doc)
}

func marshalJSON(v any) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		panic(err) // 只编码本文件的固定结构，不可能失败
	}
	return b.Bytes()
}

// writeTree 把内存树写盘（只写不删；--only 不会碰其它 case 的文件）。
func writeTree(root string, tree map[string][]byte) error {
	for p, data := range tree {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// compareTree 逐字节比对 on-disk 树与内存树，返回差异清单（空 = 一致）。
func compareTree(root string, want map[string][]byte) ([]string, error) {
	var diffs []string
	got := make(map[string]bool, len(want))
	if _, err := os.Stat(root); err != nil {
		for p := range want {
			diffs = append(diffs, p+"（缺失）")
		}
		sort.Strings(diffs)
		return diffs, nil
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		got[rel] = true
		w, ok := want[rel]
		if !ok {
			diffs = append(diffs, rel+"（多余）")
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if !bytes.Equal(b, w) {
			diffs = append(diffs, rel+"（内容不同）")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for p := range want {
		if !got[p] {
			diffs = append(diffs, p+"（缺失）")
		}
	}
	sort.Strings(diffs)
	return diffs, nil
}
