// Package facts 实现事实抽取（模块②，plan/03 §三）。
//
// 硬纪律：解析器未识别的头部/元数据区必须进 raw_header_sha256，
// 不得"没看懂就跳过"（data/README.md §4.3）。
package facts

import (
	"errors"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/artifact"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
)

// ErrNotImplemented 表示 facts 抽取尚未落地（M2）。
var ErrNotImplemented = errors.New("facts 尚未实现（计划于 M2 落地）")

// Keys 是全部受支持的 key。每个 key 必须有一个单测（plan/03 §三），
// 且 key 集合由代码常量驱动——新增 key 漏测即红（TestEveryKeyHasTest）。
var Keys = []string{
	"tar.mtime", "tar.uid", "tar.gid", "tar.uname", "tar.gname", "tar.mode", "tar.typeflag", "tar.size",
	"zip.mtime", "zip.external_attr", "zip.create_system", "zip.method",
	"gzip.mtime", "gzip.xfl", "gzip.os",
	"elf.build_id", "elf.comment", "elf.comp_dir", "elf.producer",
	"pe.timestamp", "pe.checksum", "pe.rich.present", "pe.rich.digest",
	"macho.uuid", "macho.minos", "macho.sdk", "macho.tools",
	"raw_header_sha256",
}

// Extract 抽取一个条目的全部事实。M2 实现。
//
// 返回的 []Fact 必须按 (path, key) 字节序排序（plan/03 §三）。
func Extract(raw artifact.Raw) ([]model.Fact, error) {
	return nil, ErrNotImplemented
}
