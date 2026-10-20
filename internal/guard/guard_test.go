package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	// internal/guard → 上溯两级
	return filepath.Clean(filepath.Join("..", ".."))
}

// TestDependencyDirection 断言 plan/02 §二 的依赖方向。
// 变异验证：把 align 加一行 import "internal/cli" → 本测试必须红。
func TestDependencyDirection(t *testing.T) {
	root := repoRoot(t)
	imports, err := ScanImports(root)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	if len(imports) < 8 {
		t.Fatalf("只扫到 %d 个内部包，判据是空的（阳性对照失败）", len(imports))
	}
	for pkg, deps := range imports {
		allowed, known := LayerOrder[pkg]
		if !known {
			t.Errorf("包 %q 未在 LayerOrder 登记（新增内部包必须登记依赖方向）", pkg)
			continue
		}
		for _, dep := range deps {
			if !contains(allowed, dep) {
				t.Errorf("依赖方向违规: internal/%s → internal/%s（%s 只允许 import %v）", pkg, dep, pkg, allowed)
			}
		}
	}
}

// TestNoForbiddenImports 断言 cmd/ 与 internal/ 不 import 受限包。
func TestNoForbiddenImports(t *testing.T) {
	root := repoRoot(t)
	files := 0
	violations := []string{}
	for _, dir := range []string{"cmd", "internal"} {
		base := filepath.Join(root, dir)
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			files++
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, imp := range parseImports(src) {
				if contains(ForbiddenImports, imp) {
					violations = append(violations, path+" → "+imp)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("遍历 %s 失败: %v", dir, err)
		}
	}
	if files == 0 {
		t.Fatal("没有扫到任何 .go 文件——判据是空的")
	}
	if len(violations) > 0 {
		t.Fatalf("受限 import %d 处：\n%s", len(violations), strings.Join(violations, "\n"))
	}
	t.Logf("扫描 %d 个 Go 文件，0 处受限 import", files)
}

func contains(hay []string, needle string) bool {
	for _, s := range hay {
		if s == needle {
			return true
		}
	}
	return false
}
