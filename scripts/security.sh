#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Keep the development-only scanner and its dependency lock separate from runtime.
mkdir -p work/tools/bin
(cd tools && GOBIN="$PWD/../work/tools/bin" go install -mod=readonly golang.org/x/vuln/cmd/govulncheck)
work/tools/bin/govulncheck ./...
