package normalize

import (
	"testing"
)

// TestTreeDigestIsDeterministic 是 §4.5 口径的常驻断言：
// 同输入两次调用逐字节一致；条目顺序不影响结果（由排序保证）。
func TestTreeDigestIsDeterministic(t *testing.T) {
	a := []Entry{{Path: "b.txt", Bytes: []byte("bbb")}, {Path: "a.txt", Bytes: []byte("aaa")}}
	b := []Entry{{Path: "a.txt", Bytes: []byte("aaa")}, {Path: "b.txt", Bytes: []byte("bbb")}}
	d1, d2, d3 := TreeDigest(nil, a), TreeDigest(nil, a), TreeDigest(nil, b)
	if d1 != d2 {
		t.Fatalf("同输入两次摘要不同：%s vs %s", d1, d2)
	}
	if d1 != d3 {
		t.Fatalf("条目顺序影响了摘要（未排序）：%s vs %s", d1, d3)
	}
	if len(d1) != 64 {
		t.Fatalf("摘要长度 = %d，期望 64（sha256 hex）", len(d1))
	}
}

// TestTreeDigestSeparators 断言拼接规则：relpath ‖ 0x00 ‖ sha256 ‖ 0x0A。
// 用两条"拼接后同字节流"的构造反证分隔符必须存在。
func TestTreeDigestSeparators(t *testing.T) {
	// 路径 "ab" vs "a"+"b" 若无分隔符会撞；这里用不同切分验证结果不同。
	x := TreeDigest(nil, []Entry{{Path: "ab", Bytes: []byte("")}})
	y := TreeDigest(nil, []Entry{{Path: "a", Bytes: []byte("")}, {Path: "b", Bytes: []byte("")}})
	if x == y {
		t.Fatal("不同条目切分给出了相同摘要（分隔符缺失）")
	}
}

// TestApplyIsIdempotentForRegisteredOps 是 §4.2 幂等纪律的落点。
// M0 所有 op 为 nil（未落地），因此这条断言此刻只能证明"跳过语义不会改动输入"。
// M3 落地每个 op 后，必须为每个 op 各加一条 Idempotent(Tn) 测试（plan/05 M3 DoD）。
func TestApplyIsIdempotentForRegisteredOps(t *testing.T) {
	in := []Entry{{Path: "x", Bytes: []byte("data"), Facts: map[string]string{"tar.mtime": "1"}}}
	once := Apply([]string{"T1"}, in)
	twice := Apply([]string{"T1"}, once)
	if len(once) != 1 || len(twice) != 1 {
		t.Fatalf("Apply 改变了条目数")
	}
	if once[0].Path != twice[0].Path || string(once[0].Bytes) != string(twice[0].Bytes) {
		t.Fatalf("Apply 不幂等")
	}
	// 输入不得被就地修改（纯函数）
	if string(in[0].Bytes) != "data" {
		t.Fatalf("Apply 修改了输入（非纯函数）")
	}
}

// TestOpsTableCoversT1ToT11 断言 op 表的键集合 = T1–T11，且 T12 不在表里。
// 变异验证：往 Ops 里加 "T12" → 本测试红。
func TestOpsTableCoversT1ToT11(t *testing.T) {
	if len(Ops) != 11 {
		t.Fatalf("Ops 有 %d 项，期望 11（T1–T11；T12 不可归一化）", len(Ops))
	}
	if _, ok := Ops["T12"]; ok {
		t.Fatal("Ops 里出现了 T12——T12 按 §4.1 无 op")
	}
	for i := 1; i <= 11; i++ {
		key := "T" + itoa(i)
		if _, ok := Ops[key]; !ok {
			t.Errorf("Ops 缺 %s", key)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var out []byte
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}
