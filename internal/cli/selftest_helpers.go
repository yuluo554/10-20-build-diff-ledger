package cli

import (
	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/normalize"
)

// reportRuleIDs / lastRule / digestOf 是 selftest 用的小适配层。
// 单独成文件，避免 selftest 逻辑与命令实现互相 import。

func reportRuleIDs() []string { return model.RuleIDs }

func lastRule() string {
	if len(model.RuleIDs) == 0 {
		return ""
	}
	return model.RuleIDs[len(model.RuleIDs)-1]
}

func digestOf(set []string, entries []normalize.Entry) string {
	return normalize.TreeDigest(set, entries)
}
