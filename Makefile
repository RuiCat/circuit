.PHONY: build test test-quick ci snapshot clean

build:
	go build ./...

test:
	go test -count=1 ./maths/ ./load/ ./element/logic/ ./element/semiconductor/ ./element/time/ ./analysis/

test-quick:
	go test -count=1 ./load/ ./element/semiconductor/ ./analysis/

ci:
	bash scripts/ci/run.sh --clean --offline

snapshot:
	@mkdir -p runs/engine-production/evidence
	@git rev-parse HEAD > runs/engine-production/evidence/ci-commit.txt
	@sha256sum .cache/regress-cli 2>/dev/null > runs/engine-production/evidence/ci-binary.sha256 || true

clean:
	go clean -cache
