#!/usr/bin/env bash
# benchAccuracy/run.sh：外部精度基准 harness（m1，C1/C3/C4/C10）。
# 用法: bash scripts/benchAccuracy/run.sh --selfcheck
# 退出码: 0=selfcheck PASS；3=参考器缺失/校验失败（BLOCKED）；1=语料/selftest 失败；2=用法错误
set -euo pipefail
cd "$(dirname "$0")/../.."

REF="third_party/reference/ngspice"
if [ ! -x "$REF" ]; then
  printf '{"blocked":true,"reason":"reference missing:%s"}\n' "$REF"
  exit 3
fi
if ! (cd third_party/reference && sha256sum -c SHA256SUMS >/dev/null 2>&1); then
  printf '{"blocked":true,"reason":"sha256 mismatch"}\n'
  exit 3
fi

VERSION="$("$REF" --version 2>&1 | grep -o 'ngspice-[0-9]*' | head -1 || true)"
SHA="$(sha256sum "$REF" | awk '{print $1}')"

case "${1:-}" in
  --selfcheck)
    test -s cmd/circuits/acc-corpus.txt || { printf '{"ok":false,"reason":"corpus missing"}\n'; exit 1; }
    printf '{"ok":true,"reference":"%s","version":"%s","sha256":"%s","corpus":"%s"}\n' \
      "$REF" "${VERSION:-unknown}" "$SHA" "cmd/circuits/acc-corpus.txt"
    exit 0
    ;;
  *)
    echo "usage: bash scripts/benchAccuracy/run.sh --selfcheck" >&2
    exit 2
    ;;
esac
