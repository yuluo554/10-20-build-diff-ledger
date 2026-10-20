package align

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/yuluo554/10-20-build-diff-ledger/internal/artifact"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
)

func raw(path, data string) artifact.Raw {
	return artifact.Raw{Path: path, Kind: artifact.EntryFile, Size: int64(len(data)), Data: []byte(data)}
}

func rawDir(path string) artifact.Raw {
	return artifact.Raw{Path: path, Kind: artifact.EntryDir}
}

func mustAlign(t *testing.T, left, right []artifact.Raw) *Result {
	t.Helper()
	res, err := Align(left, right)
	if err != nil {
		t.Fatalf("Align 报错: %v", err)
	}
	return res
}

// TestRenameOnlyOnEqualHash 是 K10 的机器断言（data/README.md §4.4）：
// 改名只在「两侧同哈希、且该哈希两侧各出现 1 次」时成立。
func TestRenameOnlyOnEqualHash(t *testing.T) {
	t.Run("唯一同哈希 → RENAMED", func(t *testing.T) {
		res := mustAlign(t,
			[]artifact.Raw{raw("a.txt", "A"), raw("old.py", "MOVE")},
			[]artifact.Raw{raw("a.txt", "A"), raw("new.py", "MOVE")},
		)
		if res.Counts.Renamed != 1 || res.Counts.Same != 1 || res.Counts.Added != 0 || res.Counts.Removed != 0 {
			t.Fatalf("计数错误: %+v", res.Counts)
		}
		var got *model.Entry
		for i := range res.Entries {
			if res.Entries[i].Status == model.StatusRenamed {
				got = &res.Entries[i]
			}
		}
		if got == nil {
			t.Fatal("没有 RENAMED 条目")
		}
		if got.RelPath != "new.py" || got.RenamedFrom != "old.py" {
			t.Fatalf("改名行的路径口径错误: relpath=%q renamed_from=%q（期望 relpath=右侧、renamed_from=左侧）",
				got.RelPath, got.RenamedFrom)
		}
	})

	t.Run("同哈希在左侧出现两次 → 不判改名", func(t *testing.T) {
		res := mustAlign(t,
			[]artifact.Raw{raw("dup1.bin", "X"), raw("dup2.bin", "X")},
			[]artifact.Raw{raw("moved.bin", "X")},
		)
		if res.Counts.Renamed != 0 {
			t.Fatalf("同哈希多处出现不得判改名，实得 %+v", res.Counts)
		}
		if res.Counts.Removed != 2 || res.Counts.Added != 1 {
			t.Fatalf("应记 2 个 REMOVED + 1 个 ADDED，实得 %+v", res.Counts)
		}
	})

	t.Run("同哈希在右侧出现两次 → 不判改名", func(t *testing.T) {
		res := mustAlign(t,
			[]artifact.Raw{raw("old.bin", "X")},
			[]artifact.Raw{raw("new1.bin", "X"), raw("new2.bin", "X")},
		)
		if res.Counts.Renamed != 0 || res.Counts.Removed != 1 || res.Counts.Added != 2 {
			t.Fatalf("实得 %+v", res.Counts)
		}
	})

	t.Run("哈希不等 → 不判改名", func(t *testing.T) {
		res := mustAlign(t,
			[]artifact.Raw{raw("old.bin", "X")},
			[]artifact.Raw{raw("new.bin", "Y")},
		)
		if res.Counts.Renamed != 0 || res.Counts.Removed != 1 || res.Counts.Added != 1 {
			t.Fatalf("实得 %+v", res.Counts)
		}
	})

	t.Run("改名 + 新增可同时成立（plan/03 §二 示例）", func(t *testing.T) {
		res := mustAlign(t,
			[]artifact.Raw{raw("keep.txt", "K"), raw("old.py", "M")},
			[]artifact.Raw{raw("keep.txt", "K"), raw("new.py", "M"), raw("extra.txt", "E")},
		)
		if res.Counts.Same != 1 || res.Counts.Renamed != 1 || res.Counts.Added != 1 || res.Counts.Removed != 0 {
			t.Fatalf("实得 %+v（示例要求 ADDED:1 + RENAMED:1）", res.Counts)
		}
	})
}

// TestSamePathStaysSame 断言对齐层的"结构"语义：同路径即 SAME，
// 内容差异由 attribute 归因（T12 / residue_bytes），不在 align 层重复计成增删。
func TestSamePathStaysSame(t *testing.T) {
	res := mustAlign(t,
		[]artifact.Raw{raw("pkg/a.py", "old content")},
		[]artifact.Raw{raw("pkg/a.py", "new content")},
	)
	if res.Counts.Same != 1 || res.Counts.Added != 0 || res.Counts.Removed != 0 {
		t.Fatalf("同路径不同内容应记 SAME（结构对齐），实得 %+v", res.Counts)
	}
	if res.Entries[0].SHA256 != Digest(raw("pkg/a.py", "new content")) {
		t.Fatal("SAME 行应取右侧 sha256")
	}
}

// TestKindMismatchIsExplicit 断言同路径异形态不能被说成 SAME。
func TestKindMismatchIsExplicit(t *testing.T) {
	res := mustAlign(t,
		[]artifact.Raw{rawDir("pkg/x")},
		[]artifact.Raw{raw("pkg/x", "now a file")},
	)
	if res.Counts.Removed != 1 || res.Counts.Added != 1 || res.Counts.Same != 0 {
		t.Fatalf("同名异形态应记 REMOVED + ADDED，实得 %+v", res.Counts)
	}
}

// TestEmptyDigestConstant 断言目录条目的 sha256 常量与 data/README.md §4.9 一致。
func TestEmptyDigestConstant(t *testing.T) {
	if got := Digest(rawDir("d")); got != EmptySHA256 {
		t.Fatalf("目录条目 sha256 = %s，期望 §4.9 常量 %s", got, EmptySHA256)
	}
}

// TestCountsAreConsistent 断言计数与条目表互相对得上（防"计数是另算的"）。
func TestCountsAreConsistent(t *testing.T) {
	res := mustAlign(t,
		[]artifact.Raw{raw("a", "1"), raw("b", "2"), raw("c", "3"), rawDir("d")},
		[]artifact.Raw{raw("a", "1"), raw("b", "2b"), raw("e", "4"), rawDir("d")},
	)
	var same, added, removed, renamed int
	for _, e := range res.Entries {
		switch e.Status {
		case model.StatusSame:
			same++
		case model.StatusAdded:
			added++
		case model.StatusRemoved:
			removed++
		case model.StatusRenamed:
			renamed++
		default:
			t.Fatalf("非法状态 %q", e.Status)
		}
	}
	if same != res.Counts.Same || added != res.Counts.Added || removed != res.Counts.Removed || renamed != res.Counts.Renamed {
		t.Fatalf("计数与条目表不一致: %+v vs same=%d added=%d removed=%d renamed=%d",
			res.Counts, same, added, removed, renamed)
	}
}

// TestEntriesAreByteSorted 断言条目按 relpath 的 UTF-8 字节序排列。
func TestEntriesAreByteSorted(t *testing.T) {
	res := mustAlign(t,
		[]artifact.Raw{raw("b/x", "1"), raw("a", "2"), raw("B", "3"), raw("a/y", "4")},
		[]artifact.Raw{raw("b/x", "1"), raw("a", "2"), raw("B", "3"), raw("a/y", "4")},
	)
	for i := 1; i < len(res.Entries); i++ {
		if bytes.Compare([]byte(res.Entries[i-1].RelPath), []byte(res.Entries[i].RelPath)) > 0 {
			t.Fatalf("第 %d 条 %q 排在 %q 之后（未按字节序）",
				i, res.Entries[i-1].RelPath, res.Entries[i].RelPath)
		}
	}
	if res.Entries[0].RelPath != "B" {
		t.Fatalf("UTF-8 字节序下 'B'(0x42) 应排在最前，实得 %q", res.Entries[0].RelPath)
	}
}

// TestAlignDeterministic 是 M1 DoD：同输入双跑逐字节一致。
func TestAlignDeterministic(t *testing.T) {
	root := filepath.Join("..", "..", "data", "fixtures", "align-golden", "rename-add", "pair")
	left, _, err := artifact.Load(filepath.Join(root, "left"), "", artifact.DefaultMaxEntryBytes)
	if err != nil {
		t.Fatalf("装载左侧失败: %v（先跑 go run ./tools/gen_fixtures --out data/fixtures）", err)
	}
	right, _, err := artifact.Load(filepath.Join(root, "right"), "", artifact.DefaultMaxEntryBytes)
	if err != nil {
		t.Fatalf("装载右侧失败: %v", err)
	}
	encode := func() []byte {
		res, err := Align(left, right)
		if err != nil {
			t.Fatalf("Align 失败: %v", err)
		}
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(res); err != nil {
			t.Fatalf("序列化失败: %v", err)
		}
		return b.Bytes()
	}
	if a, b := encode(), encode(); !bytes.Equal(a, b) {
		t.Fatalf("双跑输出不一致（存在非确定性来源）\nA=%s\nB=%s", a, b)
	}
}
