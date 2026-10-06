#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
go version
git --version
rg --version | head -1
fzf --version
go mod download
go mod verify
go mod tidy -diff
make fmt-check
go vet -mod=readonly ./...
note_report_dir="work/reports/${PIXI_ENVIRONMENT_NAME:-default}"
mkdir -p "$note_report_dir"
go test -mod=readonly -count=1 -coverprofile="$note_report_dir/coverage.out" ./...
go tool cover -func="$note_report_dir/coverage.out" > "$note_report_dir/coverage.txt"
tail -1 "$note_report_dir/coverage.txt"
CGO_ENABLED=1 go test -mod=readonly -count=1 -race ./...
make build
bash scripts/completion-smoke.sh "$PWD/work/bin/note"
# Exercise the ordinary installation route and installed binary outside its source.
note_install_dir="$(mktemp -d)"
trap 'rm -rf "$note_install_dir"' EXIT
GOBIN="$note_install_dir" CGO_ENABLED=0 go install -mod=readonly ./cmd/note
cd "$note_install_dir"
./note --help > /dev/null
./note --version
