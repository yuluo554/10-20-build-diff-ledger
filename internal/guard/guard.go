// Package guard 是"架构约束"的常驻测试载体（不是运行时代码）。
//
// 它把 plan/02 §二 的依赖方向与 plan/02 §五 的确定性纪律钉成可跑红的断言：
//  1. 叶子包不得反向 import 上层包（依赖方向守卫）
//  2. cmd/ 与 internal/ 不得 import 受限包（零网络 / 不读时钟 / 不读主机身份）
//  3. RuleIDs 的全集长度与 T12 末尾位置（分类法闭合）
//
// 判据基于源码文本而非 GOPATH 查询，因此在 `go vet ./...` 之前也能跑。
package guard

import (
	"os"
	"path/filepath"
	"strings"
)

// ForbiddenImports 是 cmd/ 与 internal/ 下禁止出现的 import 路径。
// 这些包会破坏"零网络 / 不读时钟 / 不读主机身份"的静态可证性。
var ForbiddenImports = []string{
	"net",
	"net/http",
	"net/url",
	"time",
	"math/rand",
	"math/rand/v2",
	"os/exec",
	"os/user",
}

// LayerOrder 是允许的依赖方向：key 可以 import value 中的包，反之不行。
// 未列出的包 = 叶子层，不得 import 任何内部包。
var LayerOrder = map[string][]string{
	"cli":       {"report", "minimize", "attribute", "facts", "align", "artifact", "normalize", "model", "version"},
	"report":    {"model", "version"},
	"minimize":  {"model", "normalize"},
	"attribute": {"model", "facts", "normalize"},
	"facts":     {"model", "artifact"},
	"align":     {"model", "artifact"},
	"normalize": {"model"},
	"artifact":  {"model"},
	"model":     {},
	"version":   {},
	"guard":     {"model"},
}

// ScanImports 返回 root 下每个内部包的 import 路径集合。
// key = 包名（如 "align"），value = 该包所有 .go 文件里的内部 import 短名。
//
// 本函数只读文件系统，不联网。
func ScanImports(root string) (map[string][]string, error) {
	internal := filepath.Join(root, "internal")
	out := make(map[string][]string)
	entries, err := os.ReadDir(internal)
	if err != nil {
		return nil, err
	}
	const prefix = "github.com/yuluo554/10-20-build-diff-ledger/internal/"
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		pkg := ent.Name()
		if _, ok := out[pkg]; !ok {
			out[pkg] = []string{} // 无内部依赖的包也要出现在扫描面上（否则"叶子层"从不被检查）
		}
		files, err := os.ReadDir(filepath.Join(internal, pkg))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") {
				continue
			}
			src, err := os.ReadFile(filepath.Join(internal, pkg, f.Name()))
			if err != nil {
				return nil, err
			}
			for _, imp := range parseImports(src) {
				if strings.HasPrefix(imp, prefix) {
					out[pkg] = append(out[pkg], strings.TrimPrefix(imp, prefix))
				}
			}
		}
	}
	return out, nil
}

// parseImports 从源码里抽出 import 路径（单行与块两种形态）。
// 不扫注释与字符串字面量（与布局门同一判据口径）。
func parseImports(src []byte) []string {
	var out []string
	text := string(src)
	lines := strings.Split(text, "\n")
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inBlock {
			if strings.HasPrefix(trimmed, ")") {
				inBlock = false
				continue
			}
			if p, ok := importPath(trimmed); ok {
				out = append(out, p)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "import (") {
			inBlock = true
			continue
		}
		if strings.HasPrefix(trimmed, "import ") {
			if p, ok := importPath(strings.TrimPrefix(trimmed, "import ")); ok {
				out = append(out, p)
			}
		}
	}
	return out
}

func importPath(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "//") {
		return "", false
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "", false
	}
	last := fields[len(fields)-1]
	if len(last) < 2 || last[0] != '"' || last[len(last)-1] != '"' {
		return "", false
	}
	return last[1 : len(last)-1], true
}
