#!/usr/bin/env bash
# ci/run.sh：干净副本 + 离线 vendor 的 CI（m5，c6/c11）。
# 用法: bash scripts/ci/run.sh [--clean] [--offline] [--snapshot-check]
set -euo pipefail
cd "$(dirname "$0")/../.."
ROOT="$(pwd)"
CLEAN=0
OFFLINE=0
SNAP=0
for a in "$@"; do
  case "$a" in
    --clean) CLEAN=1 ;;
    --offline) OFFLINE=1 ;;
    --snapshot-check) SNAP=1 ;;
  esac
done

COMMIT="$(git rev-parse HEAD 2>/dev/null || echo nogit)"
if [ "$SNAP" = 1 ]; then
  echo "commit=$COMMIT"
  if [ -x third_party/reference/ngspice ]; then
    sha256sum third_party/reference/ngspice
  fi
  exit 0
fi

WORK="$(mktemp -d /tmp/circuit-ci.XXXXXX)"
trap 'rm -rf "$WORK"' EXIT
if [ "$CLEAN" = 1 ]; then
  rsync -a --exclude=.git --exclude=.local --exclude=.cache --exclude=runs --exclude=__pycache__ "$ROOT/" "$WORK/"
else
  WORK="$ROOT"
fi

export GOFLAGS="-mod=vendor"
export GOPROXY=off
if [ "$OFFLINE" = 1 ]; then
  unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy || true
fi
cd "$WORK"

echo "runner=$(hostname 2>/dev/null || echo unknown) commit=$COMMIT offline=$OFFLINE clean=$CLEAN"
go build ./...
go test -count=1 ./load/ ./element/semiconductor/ ./analysis/ ./maths/
CIRCUIT_SERIAL_NET=59_cpu8v2_serial_macro.net go test -count=1 -run 'TestCPU8v2SerialLoad' ./cmd/
echo "CI PASS"
