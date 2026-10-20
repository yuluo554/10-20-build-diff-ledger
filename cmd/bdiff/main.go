// bdiff 是构建产物差异归因账本的 CLI 入口。
//
// 这个文件只做两件事：把 os.Args 交给 internal/cli，把返回码交给 os.Exit。
// 任何业务逻辑都不写在这里（plan/02 §四：退出码唯一出口在 internal/cli）。
package main

import (
	"os"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
