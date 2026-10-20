// gen_fixtures 是样张生成器（plan/04 §三）。
//
// M0 状态：可运行的桩——参数、`--check` 通路、退出码契约已定；
// 13 组样张的实际合成在 M1 落地。
//
// 纪律：不读系统时钟、不读环境变量（除显式 --out/--only）、
// 部分运行（--only）绝不改写全集清单。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// cases 是样张矩阵的 case 名（顺序即清单顺序，plan/04 §二）。
var cases = []string{
	"c01-mtime",
	"c02-owner",
	"c03-mode",
	"c04-prefix",
	"c05-order",
	"c06-gzip-opts",
	"c07-archive-hdr",
	"c08-toolchain",
	"c09-mtime-owner",
	"c10-toolchain-path",
	"c11-real-change",
	"c12-mixed",
	"c13-signature",
}

func main() {
	out := flag.String("out", "data/fixtures", "样张输出目录")
	only := flag.String("only", "", "只生成指定 case（不写全集清单）")
	check := flag.Bool("check", false, "校验模式：生成到临时目录并与入仓样张逐字节比对")
	flag.Parse()

	if err := run(*out, *only, *check); err != nil {
		fmt.Fprintf(os.Stderr, "gen_fixtures: %v\n", err)
		os.Exit(1)
	}
}

func run(out, only string, check bool) error {
	if only != "" && !known(only) {
		return fmt.Errorf("未知 case %q（合法值见 plan/04 §二）", only)
	}
	if check {
		// M1：生成到临时目录 → 与 out 逐字节比对 → 打印 "N cases, M diff"
		return fmt.Errorf("--check 尚未实现（计划于 M1 落地）")
	}
	if only != "" {
		fmt.Printf("gen_fixtures: 局部生成 %s → %s（不写全集清单）\n", only, out)
		return fmt.Errorf("样张合成尚未实现（计划于 M1 落地）")
	}
	fmt.Printf("gen_fixtures: 计划生成 %d 组样张 → %s\n", len(cases), out)
	_ = filepath.Join(out, "manifest.json")
	return fmt.Errorf("样张合成尚未实现（计划于 M1 落地）")
}

func known(name string) bool {
	for _, c := range cases {
		if c == name {
			return true
		}
	}
	return false
}
