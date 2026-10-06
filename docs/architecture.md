# Architecture

`cmd/note` installs signal cancellation, creates a fresh Cobra command tree,
reports errors, and exits after cleanup. `internal/cli` owns routing, lazy config
loading, prompts, editor streams, output policy, and completion.

`internal/config` validates TOML and resolves XDG/config paths independently of
dependency availability. `internal/notes` owns root containment, symlink checks,
basename normalization/collisions, and exclusive writes. `internal/templates`
embeds resources and implements a single-pass fixed substitution renderer.

`internal/search` creates the shared rg-discovered collection, performs path
matching/content JSON decoding, and orders/serializes result records. A result
carries the relative path, provenance, optional line/snippet, and optional mtime.
Absolute paths are derived only from the canonical root through safety checks.

`internal/picker` writes a private temporary manifest, sends NUL-framed numeric
IDs and escaped display text to fzf, and resolves selection by ID. The hidden
installed-binary preview validates the manifest independently of config and
reads bounded source text. The fixed preview wrapper POSIX-quotes binary and
manifest paths; only a numeric ID is substituted by fzf. `internal/process`
provides direct argv execution, context cancellation, and bounded child waiting.
Noninteractive search/Git commands have their own process group: cancellation
first interrupts the group, then removes any surviving descendants after waiting.
This also bounds hooks/wrappers that ignore SIGINT or keep output pipes open.
`internal/errs` carries public exit codes with original errors.

`internal/syncgit` handles dedicated-repository preflight, advisory locking,
commit-first fetch/rebase, collision protection, and explicit upstream push.
Recovery is Git's commits/index/operation state; there is no stash journal.

`internal/update` reads bounded public GitHub release metadata, compares stable
numeric versions, verifies archive hashes/contents, and stages atomic executable
replacement under a stable advisory lock. A matching installation receipt and
binary hash identify installer-owned destinations. The CLI owns explicit update
routing and the interactive-only, bounded, daily cached hint. Release builds
carry an archive distribution stamp; source/package builds cannot self-replace.
`scripts/install.sh` bootstraps from the same verified archives and delegates
filesystem installation to their hidden `__install` command. Update state lives
in the XDG cache and beside the executable, separate from knowledge/config data.

There are no package-init filesystem operations, index, database, daemon, Markdown
parser, general extension system, or dependency-injection container. Seams are
limited to streams, terminal status, executable/home resolution, release HTTP
transport, and clocks.
Tests use temporary data and real subprocesses. Add general features at the
existing responsibility boundary and document public contract changes.

Path checks protect ordinary CLI use. They do not claim protection against a
hostile process replacing filesystem components concurrently. Exclusive writes
protect normal concurrent creation; symlinks and containment are rechecked before
creation/opening.

The actual Git origin supplies the module address, and the existing MIT license
supplies copyright metadata. This resolves the plan's original placeholders.
