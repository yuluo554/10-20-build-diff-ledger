#!/usr/bin/env bash
# Go 侧守门的变异验证（plan/05 §四 的补充：布局门之外那几条架构约束）。
#
# 每条变异：改一处 → 跑指定测试 → **必须红** → 还原 → 断言字节一致。
# 三连断言（marker 存在 / 已产生字节变化 / 还原后一致）全部在下面逐条显式写出，
# 防"变异没生效却以为守门是假门"。
#
# 用法：bash tools/verify-go-guards.sh
# 退出码：0 全部如期变红且还原干净 / 1 有变异没红或还原失败。

set -u
cd "$(dirname "$0")/.." || exit 1

# 解析 Go 可执行文件。
# 首版写成 GO="${GO:-go}" 直接 `"$GO" test`，而本机 go 不在 PATH →
# **"command not found" 也是非零**，于是 6 组变异全部"如期变红"——守护脚本自己成了假门。
# 判据修正两条：
#   ① 这里先**证明 go 真的能跑**（版本号解析成功），跑不起来直接 exit 1，不许继续；
#   ② expect_red 要求输出里出现 Go 的 `--- FAIL: <TestName>`，而不是只看退出码。
GO="${GO:-}"
if [ -z "$GO" ]; then
  if command -v go >/dev/null 2>&1; then
    GO="go"
  elif command -v cygpath >/dev/null 2>&1 && [ -x "$LOCALAPPDATA/Programs/Go/bin/go.exe" ]; then
    GO="$(cygpath -u "$LOCALAPPDATA")/Programs/Go/bin/go.exe"
  fi
fi
if [ -z "$GO" ] || ! "$GO" version >/dev/null 2>&1; then
  echo "FAIL  找不到可运行的 Go（试过 PATH、\$LOCALAPPDATA/Programs/Go/bin/go.exe）"
  echo "      处置：把 Go 加入 PATH，或 GO=<可执行文件绝对路径> 运行本脚本"
  exit 1
fi
echo "go: $("$GO" version)"

# Python 通道（变异脚本用它做"marker 存在 / 已变 / 还原"三连断言）。
# 本机是 py -3.12（商店占位符 python 不可用）；CI 是 python3。
if [ -z "${PY:-}" ]; then
  if py -3.12 -c "pass" >/dev/null 2>&1; then
    PY="py -3.12"
  elif command -v python3 >/dev/null 2>&1; then
    PY="python3"
  elif command -v python >/dev/null 2>&1 && python -c "pass" >/dev/null 2>&1; then
    PY="python"
  else
    echo "FAIL  找不到可用的 Python（试过 py -3.12 / python3 / python）"
    exit 1
  fi
fi
echo "python: $PY"
fail=0
tmp="$(mktemp -d 2>/dev/null || echo ./.mutation-tmp)"
mkdir -p "$tmp"

# apply <file> <from> <to> <label>  → 输出备份路径；失败直接退出
apply() {
  local file="$1" from="$2" to="$3" label="$4"
  local backup="$tmp/$(echo "$file" | tr '/' '_').bak"
  cp "$file" "$backup" || { echo "FAIL  $label: 备份失败"; exit 1; }
  FROM="$from" TO="$to" LABEL="$label" $PY - "$file" "$backup" <<'PY' || { echo "FAIL  $label: 变异未生效（见上）"; exit 1; }
import os, sys
file, backup = sys.argv[1], sys.argv[2]
orig = open(backup, encoding="utf-8").read()
from_s, to_s = os.environ["FROM"], os.environ["TO"]
assert from_s in orig, "marker 不存在: " + os.environ["LABEL"]
new = orig.replace(from_s, to_s, 1)
assert new != orig, "替换未产生字节变化: " + os.environ["LABEL"]
open(file, "w", encoding="utf-8", newline="\n").write(new)
PY
  echo "$backup"
}

# restore <file> <backup> <label>
restore() {
  local file="$1" backup="$2" label="$3"
  cp "$backup" "$file" || { echo "FAIL  $label: 还原失败"; exit 1; }
  cmp -s "$file" "$backup" || { echo "FAIL  $label: 还原后字节不一致"; fail=$((fail+1)); }
}

# expect_red <label> <test-package> <test-name>
# 判据 = 退出码非零 **且** 输出里出现 `--- FAIL: <TestName>`。
# 只看退出码会把"编译错/命令缺失"误当成"如期变红"（本脚本首版实测踩过）。
expect_red() {
  local label="$1" pkg="$2" name="$3"
  if "$GO" test "$pkg" -run "$name" -count=1 >"$tmp/out.txt" 2>&1; then
    echo "FAIL  $label → 期望 $name 变红，实际通过"
    fail=$((fail+1))
    return
  fi
  if grep -q -- "--- FAIL: $name" "$tmp/out.txt"; then
    echo "PASS  $label → $name 如期变红"
  elif grep -qE "build failed|undefined:|command not found" "$tmp/out.txt"; then
    echo "FAIL  $label → 测试没跑起来（编译/环境问题，不算变红）"
    sed -n '1,5p' "$tmp/out.txt"
    fail=$((fail+1))
  else
    echo "FAIL  $label → 退出非零但不是 $name 失败"
    sed -n '1,8p' "$tmp/out.txt"
    fail=$((fail+1))
  fi
}

echo "== Go 守门变异验证 =="

# G1：规则集少一条 → 闭合性断言必须红
b=$(apply internal/model/model.go '"T10", "T11", "T12",' '"T10", "T11",' "G1 规则集缺 T12")
expect_red "G1 规则集缺 T12" ./internal/guard TestRuleSetClosed
restore internal/model/model.go "$b" "G1"

# G2：align 反向依赖 cli → 依赖方向断言必须红
b=$(apply internal/align/align.go \
  '	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"' \
  '	"github.com/yuluo554/10-20-build-diff-ledger/internal/model"
	"github.com/yuluo554/10-20-build-diff-ledger/internal/cli"' \
  "G2 align→cli 反向依赖")
expect_red "G2 align→cli 反向依赖" ./internal/guard TestDependencyDirection
restore internal/align/align.go "$b" "G2"

# G3：往 normalize.Ops 里塞 T12 → op 表口径断言必须红
# marker 随 M3 把 op 从 nil 换成实现而更新（旧 marker `"T9": nil, ...` 已不存在）。
b=$(apply internal/normalize/normalize.go '"T9": opT9, "T10": opT10, "T11": opT11,' '"T9": opT9, "T10": opT10, "T11": opT11, "T12": opT1,' "G3 Ops 混入 T12")
expect_red "G3 Ops 混入 T12" ./internal/normalize TestOpsTableCoversT1ToT11
restore internal/normalize/normalize.go "$b" "G3"

# G4：报告 struct 字段换序 → 键序断言必须红
# marker 用**当前源码的真实空白**（gofmt 之后对齐列宽会变；写死成旧版空白 =
# marker 不存在，变异根本没发生——这条脚本自己踩过一次）。
b=$(apply internal/report/report.go $'\tSchemaVersion   int            `json:"schema_version"`\n\tToolVersion     string         `json:"tool_version"`' $'\tToolVersion     string         `json:"tool_version"`\n\tSchemaVersion   int            `json:"schema_version"`' "G4 schema_version 与 tool_version 换序")
expect_red "G4 report 键序互换" ./internal/report TestReportKeyOrder
restore internal/report/report.go "$b" "G4"

# G5：tree_digest 去掉 0x00 分隔符 → 已知答案（KAT）必须红
# 靶子必须选 TestTreeDigestKnownAnswer，不能选"不同切分不撞"那条——
# 后者在任何实现下都成立（hex 定长），实测变异后仍通过（"断言没在测它声称在测的东西"）。
b=$(apply internal/normalize/normalize.go $'\t\th.Write([]byte{0x00})' $'\t\t// separator removed' "G5 去掉 0x00 分隔符")
expect_red "G5 去掉 0x00 分隔符" ./internal/normalize TestTreeDigestKnownAnswer
restore internal/normalize/normalize.go "$b" "G5"

# G6：budget 接受未知规则 → 三形态断言必须红
b=$(apply internal/report/report.go $'\t\tif !isRuleID(p) {' $'\t\tif false && !isRuleID(p) {' "G6 budget 放过未知规则")
expect_red "G6 budget 放过未知规则" ./internal/report TestParseBudgetThreeForms
restore internal/report/report.go "$b" "G6"

# G7：改名判定去掉"左侧每哈希唯一"约束 → K10 断言必须红
# 变异方式要选**仍能编译**的那种：直接删掉整行会让 rightHash 变成未使用变量，
# 那属于"测试没跑起来"，不算如期变红（本脚本的判据正是防这个）。
b=$(apply internal/align/align.go \
  $'\t\tif rightHash[re.SHA256] != 1 || leftHash[re.SHA256] != 1 {' \
  $'\t\tif rightHash[re.SHA256] != 1 || leftHash[re.SHA256] != 999 {' \
  "G7 改名不看左侧哈希唯一")
expect_red "G7 改名不看哈希唯一" ./internal/align TestRenameOnlyOnEqualHash
restore internal/align/align.go "$b" "G7"

# G8：全量清单的写盘条件去掉 only 守卫 → K11 断言必须红
# 变异后 --only 也会写 manifest.json（部分运行污染全集清单）。
b=$(apply tools/gen_fixtures/main.go \
  $'\tif only == "" {\n\t\t// align golden 的输入对' \
  $'\tif true {\n\t\t// align golden 的输入对' \
  "G8 --only 写全集清单")
expect_red "G8 --only 写全集清单" ./tools/gen_fixtures TestOnlyWritesNoManifest
restore tools/gen_fixtures/main.go "$b" "G8"

# G9：把 tar.mtime 误分类成 T3 → 14 组样张矩阵（expect.json 驱动）必须红。
# 这是 M3 新增判据的变异：证明 TestFixtureMatrix 不是在"恒绿"。
b=$(apply internal/attribute/attribute.go \
  $'\tcase "tar.mtime", "zip.mtime", "gzip.mtime":\n\t\treturn "T1", false, false' \
  $'\tcase "tar.mtime", "zip.mtime", "gzip.mtime":\n\t\treturn "T3", false, false' \
  "G9 tar.mtime 误分类")
expect_red "G9 tar.mtime 误分类" ./internal/cli TestFixtureMatrix
restore internal/attribute/attribute.go "$b" "G9"

# G10：把候选枚举改成基数降序 → 首个命中退化成全集（"最小集"变超集）。
# 这是 M4 新增判据（最小性 / 非超集）的变异：证明 TestMinimizeDoDCases 不是在"恒绿"。
b=$(apply internal/minimize/minimize.go \
  $'\tfor size := 0; size <= MaxSetSize; size++ {' \
  $'\tfor size := MaxSetSize; size >= 0; size-- {' \
  "G10 枚举基数降序")
expect_red "G10 枚举基数降序" ./internal/cli TestMinimizeDoDCases
restore internal/minimize/minimize.go "$b" "G10"

rm -rf "$tmp"

if [ "$fail" -ne 0 ]; then
  echo "结论：$fail 条变异未如期变红"
  exit 1
fi
echo "结论：10/10 组 Go 守门变异如期变红，还原后源码字节一致"
