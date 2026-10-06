# note

[![CI](https://github.com/mariusrueve/note-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/mariusrueve/note-cli/actions/workflows/ci.yml)

A small Go CLI for a personal Markdown knowledge base on macOS and Linux. Notes
remain ordinary files and folders, usable with any editor. Projects are folders.
There is no database, metadata injection, AI, service, or persistent index.

Use [Pixi](https://pixi.sh) 0.72 or newer for development. The checked-in lockfile
provides Go, Git, rg, fzf, Bash, Fish, Make, and workflow linting on macOS and
Linux, on both ARM64 and AMD64. Native race tests also need a host C compiler
(Xcode Command Line Tools on macOS; GCC or Clang on Linux). Completion checks use
the system Zsh on macOS and locked Pixi Zsh on Linux.

At runtime, opening needs your editor
(default `hx`), discovery/search needs `rg`, interactive selection needs `fzf`, and
sync needs Git. `new --no-open` needs none of these tools. See
[verification](docs/verification.md) for tested versions and platform coverage.

```sh
pixi install --locked
pixi run --locked check     # format, vet, coverage, integration, race, shell smoke
pixi run --locked security  # official Go vulnerability scanner
pixi run --locked build     # work/bin/note
pixi run --locked install   # Go installation to GOBIN or GOPATH/bin
```

Installation uses `GOBIN` when set, otherwise `$(go env GOPATH)/bin` (usually
`~/go/bin`). Add that directory to `PATH`, for example
`export PATH="$HOME/go/bin:$PATH"`. Installed binaries need no Go runtime, checkout,
or module cache. The module is `github.com/mariusrueve/note-cli`.

```sh
note init --root ~/Knowledge
note new docker/networking
note new cheminformatics/ecfp --template concept --title ECFP
note open docker/networking
note                       # fzf with a right-side source preview
note docker                # convenient lookup; offers confirmed creation
note find docker           # paths only
note grep "extended connectivity" # contents only
note search fingerprint    # both, once per note
note recent --limit 20
note search fingerprint --json
note templates
note doctor
note completion zsh
```

New basenames become ASCII kebab-case; directory spelling and existing filenames
stay intact. Repeating `new` preserves the file and asks for Enter to open it.
Templates use fixed placeholders and add no frontmatter automatically.

`note sync` commits all nonignored repository changes, fetches/rebases, and pushes
the explicit upstream. First set up a dedicated knowledge Git repository using
[the sync guide](docs/sync.md). Attachments and deletions are included. Save editor
buffers before syncing; conflicts use normal Git recovery.

To update, pull or edit the source, run `pixi run --locked check`, then reinstall
using `pixi run --locked install`. Source edits do not update an installed
executable. Removing the executable leaves config and knowledge files intact.
Portable archive instructions are in [releasing](docs/releasing.md).

CI runs the same locked tasks on native macOS ARM64/Intel and Linux ARM64/AMD64,
with both Go 1.26 and 1.27. Verified `vX.Y.Z` tags publish four portable archives,
checksums, and their build/dependency record. Manual release dispatch validates a
version without publishing. Dependabot and weekly vulnerability scans maintain
the dependency checks. See [development](CONTRIBUTING.md) and
[security](SECURITY.md).

Read [CLI](docs/cli.md), [configuration](docs/configuration.md),
[templates](docs/templates.md), [completion](docs/completion.md),
[troubleshooting](docs/troubleshooting.md), and [contributing](CONTRIBUTING.md).
Licensed under MIT.
