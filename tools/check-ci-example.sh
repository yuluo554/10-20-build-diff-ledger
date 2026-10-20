#!/usr/bin/env bash
# ci/example.yml 的**本地等价校验**（M5 DoD 第 4 条：不联网的部分）。
#
# 做法：不解析 YAML（避免引入依赖），而是把示例里那条门禁命令按**同样的参数形态**
# 在本地样张上跑一遍，并逐条断言示例承诺的结果：
#
#   ① `compare --budget T1,T2 --out report` 在"仅 T1/T2 差异"的样张上 → exit 0
#   ② 同一条命令在"有真内容残留"的样张上 → exit 3（白名单不得降级）
#   ③ `report/report.md` 与 `report/report.json` 都落盘，且 md 里**仍逐条列出被放行的差异**
#      （"放行 ≠ 消失"这条承诺）
#   ④ 退出码 1 的分支（读不了）也在示例的 case 里 → 用 broken 样张验一次
#
# 用法：bash tools/check-ci-example.sh
# 退出码：0 全部符合示例承诺 / 1 有断言失败 / 2 环境缺 Go。

set -u
cd "$(dirname "$0")/.." || exit 2

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
  exit 2
fi

BIN=".tmp-ci-example/bdiff"
mkdir -p .tmp-ci-example
trap 'rm -rf .tmp-ci-example' EXIT

echo "== 构建 =="
CGO_ENABLED=0 "$GO" build -trimpath -o "$BIN" ./cmd/bdiff || { echo "FAIL 构建失败"; exit 1; }
"$BIN" version || { echo "FAIL version 失败"; exit 1; }
"$BIN" selftest >/dev/null || { echo "FAIL selftest 失败"; exit 1; }

fail=0
pass() { echo "PASS  $1"; }
bad()  { echo "FAIL  $1"; fail=$((fail+1)); }

# 与 ci/example.yml 的 "Gate" 步骤同一套参数。
gate() { # gate <left> <right> <budget> <outdir>
  set +e
  "$BIN" compare --left "$1" --right "$2" --budget "$3" --out "$4" >/dev/null 2>&1
  local code=$?
  set -e
  echo "$code"
}

echo "== ① 仅元数据差异且在白名单内 → 放行 =="
c09l=data/fixtures/c09-mtime-owner/pair/left/synthpkg-0.1.0.tar
c09r=data/fixtures/c09-mtime-owner/pair/right/synthpkg-0.1.0.tar
code=$(gate "$c09l" "$c09r" T1,T2 .tmp-ci-example/report-ok)
[ "$code" = "0" ] && pass "c09 + --budget T1,T2 → exit 0（示例的 notice 分支）" \
                  || bad "c09 + --budget T1,T2 → exit $code，期望 0"

echo "== ② 白名单不得降级残留 =="
c11l=data/fixtures/c11-real-change/pair/left/synthpkg-0.1.0.tar.gz
c11r=data/fixtures/c11-real-change/pair/right/synthpkg-0.1.0.tar.gz
code=$(gate "$c11l" "$c11r" T1,T2 .tmp-ci-example/report-residue)
[ "$code" = "3" ] && pass "c11 + --budget T1,T2 → exit 3（示例的 error 分支）" \
                  || bad "c11 + --budget T1,T2 → exit $code，期望 3"

echo "== ③ 报告落盘 + 放行不等于消失 =="
for f in report.json report.md; do
  if [ -s ".tmp-ci-example/report-ok/$f" ]; then pass "report-ok/$f 非空"; else bad "report-ok/$f 缺失或为空"; fi
done
if grep -q '"T1":[1-9]' .tmp-ci-example/report-ok/report.json &&
   grep -q '"T2":[1-9]' .tmp-ci-example/report-ok/report.json; then
  pass "report.json 仍记录被放行的 T1/T2 计数（放行 ≠ 消失）"
else
  bad "report.json 丢了被放行的计数"
fi
if grep -q 'T1 | 时间戳 | [1-9]' .tmp-ci-example/report-ok/report.md; then
  pass "report.md 仍逐类列出被放行的差异"
else
  bad "report.md 未列出被放行的差异"
fi

echo "== ④ 读不了 → exit 1（示例的 error 分支） =="
broken=data/fixtures/broken/truncated.tar
code=$(gate "$broken" "$c09r" T1,T2 .tmp-ci-example/report-broken)
[ "$code" = "1" ] && pass "broken 输入 → exit 1" || bad "broken 输入 → exit $code，期望 1"

echo "== ⑤ 白名单外元数据差异 → 2（示例的 warning 分支） =="
c01l=data/fixtures/c01-mtime/pair/left/synthpkg-0.1.0.tar
c01r=data/fixtures/c01-mtime/pair/right/synthpkg-0.1.0.tar
code=$(gate "$c01l" "$c01r" T2 .tmp-ci-example/report-meta)
[ "$code" = "2" ] && pass "c01 + --budget T2 → exit 2（白名单外）" \
                  || bad "c01 + --budget T2 → exit $code，期望 2"

if [ "$fail" -ne 0 ]; then
  echo "结论：$fail 条断言失败"
  exit 1
fi
echo "结论：ci/example.yml 的四个分支（0/1/2/3）在本地全部按示例承诺复现"
