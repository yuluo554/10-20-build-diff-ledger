// 合成内容表（plan/04 §三，data/README.md §三）。
//
// 纪律：
//   - 不引用任何真实发行版二进制、真实公司、真实人员、真实域名；
//   - 填充字节由 sha256(seed‖index) 派生（不用 math/rand），跨语言/跨版本稳定；
//   - 时间戳只从 {0, 1700000000, 1700003600} 取（见 TimeTable）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
)

// 合成项目固定元数据（data/README.md §三）。
const (
	synthName    = "synthpkg"
	synthVersion = "0.1.0"
	synthAuthor  = "Fixture Bot <fixture@example.invalid>"
)

// TimeTable 是样张允许使用的全部时间戳（data/README.md §三 / §4.7）。
var TimeTable = []int64{0, 1700000000, 1700003600}

// filler 用 sha256(seed‖n) 派生 n 个确定性字节。
// 不用 math/rand：同一 seed 在任何机器、任何 Go 版本上给出同一串字节。
func filler(seed string, n int) []byte {
	out := make([]byte, 0, n)
	for i := 0; len(out) < n; i++ {
		sum := sha256.Sum256([]byte(seed + "|" + itoa(i)))
		out = append(out, sum[:]...)
	}
	return out[:n]
}

// hexBytes 把确定性填充转成可读的十六进制文本行。
func hexBytes(seed string, n int) string {
	return hex.EncodeToString(filler(seed, n))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// 逻辑树：所有非 ELF 样张共用同一份源码树，只换容器与头部字段。
func pythonFiles() []fileSpec {
	return []fileSpec{
		{Path: "synthpkg/__init__.py", Data: []byte("__version__ = \"" + synthVersion + "\"\n")},
		{Path: "synthpkg/core.py", Data: []byte(
			"\"\"\"synthpkg core — 合成样张，不含任何真实实现。\"\"\"\n" +
				"\n" +
				"VERSION = \"" + synthVersion + "\"\n" +
				"\n" +
				"def digest(payload):\n" +
				"    return payload\n")},
		{Path: "synthpkg/data/table.csv", Data: []byte(
			"key,value\n" +
				"alpha," + hexBytes("table-alpha", 8) + "\n" +
				"beta," + hexBytes("table-beta", 8) + "\n")},
		{Path: "synthpkg/METADATA", Data: []byte(
			"Metadata-Version: 2.1\n" +
				"Name: " + synthName + "\n" +
				"Version: " + synthVersion + "\n" +
				"Author: " + synthAuthor + "\n")},
		{Path: "synthpkg/README.md", Data: []byte(
			"# " + synthName + "\n\n合成样张；由 `tools/gen_fixtures` 程序化生成，不含真实内容。\n")},
	}
}

// elfFiles 是含可执行文件的逻辑树（T8 / T9 / T10 / T11 的载体）。
func elfFiles() []fileSpec {
	return append([]fileSpec{
		{Path: "synthpkg/bin/synthpkg", Kind: kindELF},
	}, pythonFiles()...)
}

// alignGoldenFiles 是 align golden 对的逻辑树（同树 + 1 改名 + 1 新增 + 1 删除）。
func alignGoldenFiles() []fileSpec {
	return []fileSpec{
		{Path: "pkg/__init__.py", Data: []byte("__version__ = \"" + synthVersion + "\"\n")},
		{Path: "pkg/core.py", Data: []byte("def run():\n    return 1\n")},
		{Path: "pkg/data.csv", Data: []byte("a,1\nb,2\n")},
		{Path: "pkg/old_name.py", Data: []byte("RENAMED_PAYLOAD = \"" + hexBytes("rename", 8) + "\"\n")},
		{Path: "pkg/legacy.txt", Data: []byte("only on the left side\n")},
	}
}

// alignGoldenRight 由左树派生：old_name.py → new_name.py、legacy.txt 换成 extra.txt。
func alignGoldenRight() []fileSpec {
	var out []fileSpec
	for _, f := range alignGoldenFiles() {
		switch f.Path {
		case "pkg/old_name.py":
			out = append(out, fileSpec{Path: "pkg/new_name.py", Data: f.Data})
		case "pkg/legacy.txt":
			out = append(out, fileSpec{Path: "pkg/extra.txt", Data: []byte("only on the right side\n")})
		default:
			out = append(out, f)
		}
	}
	return out
}
