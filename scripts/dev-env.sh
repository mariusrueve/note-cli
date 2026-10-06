# Sourced by Pixi before running tasks. Keep caches and tooling inside the checkout.
note_dev_root="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
export GOTOOLCHAIN=local
export GOCACHE="$note_dev_root/work/go-cache/${PIXI_ENVIRONMENT_NAME:-default}"
export GOMODCACHE="$note_dev_root/work/go-mod"
# Race tests use the native compiler and SDK supplied by the host/CI runner.
export CC=cc
unset note_dev_root
