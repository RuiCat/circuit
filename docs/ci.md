# CI 与工程化（m5，c6）

- 入口：`bash scripts/ci/run.sh --clean --offline`（干净副本 + 离线 vendor + build/测试）
- Makefile：`make build` / `make test` / `make test-quick` / `make ci` / `make snapshot`
- CI：`.github/workflows/ci.yml`（push/PR 触发，`--clean --offline`）
- 依赖：`vendor/`（`go mod vendor` 生成，`GOFLAGS=-mod=vendor`、`GOPROXY=off`）
- 数字一致性门：`go test -run 'TestCPU8v2SerialLoad' ./cmd/`（C7，约 33s）
- 逐位快照门：`scripts/regressSnapshot.sh` + `regressDiff.py --bit-exact`（重跑由 `make snapshot` 记录 commit/sha）

## 干净 + 离线定义

- 干净：`rsync` 复制到临时目录（排除 `.git/.local/.cache/runs`），不在源树里跑。
- 离线：`GOPROXY=off` + `GOFLAGS=-mod=vendor`，只读 `vendor/`；不读本机 `~/.cache`/绝对路径。
- 证据：脚本输出 `runner=… commit=… offline=… clean=…`，结尾 `CI PASS`。

## 已知边界

- 本机无 GitHub runner，`--clean --offline` 是本地可执行的等价验收；GitHub 上由 `ci.yml` 调同一脚本。
- 慢门（V2 271s、57 夜间）不进 CI；由 nightly 脚本覆盖。
