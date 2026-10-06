# Development

Install [Pixi](https://pixi.sh) 0.72 or newer, clone the repository, and use the
checked-in environment lock. Pixi provides the Go toolchain and integration tools;
there is no system Go requirement. Native race tests require a C compiler and SDK:
Xcode Command Line Tools on macOS (`xcode-select --install`), or GCC/Clang on Linux.

```sh
pixi install --locked
pixi run --locked check
pixi run --locked -e go126 check
pixi run --locked security
pixi run --locked lint-workflows
pixi run --locked build                  # work/bin/note, CGO disabled
pixi run --locked go run ./cmd/note --help
pixi run --locked fmt                    # apply gofmt after edits
pixi run --locked install                # GOBIN or GOPATH/bin
```

The default environment uses locked Go 1.27; `go126` checks compatibility with
Go 1.26. Exact versions and SHA-256 package hashes for macOS ARM64/AMD64 and Linux
ARM64/AMD64 are in `pixi.lock`. `--locked` refuses manifest/lock drift. Host compiler
and SDK versions remain platform prerequisites. Completion checks use macOS's
system Zsh because the conda-forge macOS Zsh package omits required dynamic
modules; Linux Zsh is locked in Pixi. Caches, binaries, test reports,
and downloaded development scanners live under ignored `work/`; environments
live under ignored `.pixi/`. Neither directory belongs in a commit.

`check` verifies module checksums and tidy stability, checks formatting, runs vet,
collects coverage, runs the integration and race suites, builds a standalone
binary, exercises three shell completions, and installs outside the checkout.
Tests require their tools and fail if missing. The terminal tests use a real PTY,
real fzf preview/selection, a Go editor helper, and cancellation. Tests use local
bare Git remotes, isolated Git configuration, and temporary knowledge roots.
Never use personal knowledge data or publish as part of routine tests.

GitHub Actions runs these tasks with both toolchains on four native platforms.
It retains coverage reports and builds portable archives only after tests and
security checks pass. Those archives then run native installed-binary and PTY
smoke tests on every target. `CI` is the aggregate status suitable for a required
branch check. Workflow actions use full commit hashes, read-only permissions by
default, bounded job timeouts, and lockfile-based environment caches.

The official Go vulnerability scanner is pinned in the separate `tools` module.
Dependabot proposes Go module and workflow Action updates weekly; a separate
weekly scan catches newly disclosed vulnerabilities. Review updates before merge.
For Go dependencies, choose explicit compatible versions, run `pixi run go mod
tidy`, review go.mod/go.sum, and run both toolchains. For development packages,
run `pixi update`, review manifest/lock changes on all four platforms, and rerun
checks. CI must never solve a new lockfile as part of installation.

After changing source, rebuild/reinstall: source edits do not update installed
executables. Ordinary Go users can still run `make check`, `make build`, and
`go install -mod=readonly ./cmd/note` with the documented tools on PATH.

Read [architecture](docs/architecture.md), [CLI](docs/cli.md), AGENTS.md, and
[releasing](docs/releasing.md). Update docs, CHANGELOG, and tests when changing
public behavior. Keep PLAN.md and [verification](docs/verification.md) accurate.
