package guard

import (
	"strings"
	"testing"
)

// TestParseImportsCoversBothForms 是 parseImports 的探针自证：
// 单行、块内带别名、块内裸路径三种写法都必须被检出。
// （方法论"AST/文本类守门必须逐形态自证"：只测一种写法 = 只证明一种被检出。）
func TestParseImportsCoversBothForms(t *testing.T) {
	src := `package demo

import "fmt"

import (
	"os"
	alias "path/filepath"
	_ "embed"
)

// import "net"  ← 注释里的不算
var s = "import \"net\""
`
	got := parseImports([]byte(src))
	want := []string{"fmt", "os", "path/filepath", "embed"}
	if len(got) != len(want) {
		t.Fatalf("检出 %d 条 import（%v），期望 %d 条", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 条 = %q，期望 %q（全部：%v）", i, got[i], want[i], got)
		}
	}
}

// TestForbiddenListIsNotEmpty 防"名单被清空 → 门恒绿"。
func TestForbiddenListIsNotEmpty(t *testing.T) {
	if len(ForbiddenImports) < 5 {
		t.Fatalf("ForbiddenImports 只剩 %d 条，门形同虚设", len(ForbiddenImports))
	}
	joined := strings.Join(ForbiddenImports, ",")
	for _, must := range []string{"net", "time", "os/exec"} {
		if !strings.Contains(joined, must) {
			t.Fatalf("ForbiddenImports 缺 %q", must)
		}
	}
}
