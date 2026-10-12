# Verification record

Implementation and local checks performed on 2026-10-06, macOS ARM64.
This records executed local checks and native GitHub Actions results.

Version 0.2.0 preparation on 2026-10-12, macOS ARM64, Go 1.27.2:

- Research templates, `--in DIR`, inbox capture, backlinks/deadlinks, and
  `note cite` with `internal/bib` are covered by unit and CLI tests; all checks
  below ran after rebasing onto the Go 1.27.2 lock update.
- Local `check`, `-e go126 check`, `security` (no vulnerabilities found), and
  `lint-workflows` passed. Main CI passed for the feature merge
  ([mariusrueve/note-cli#5](https://github.com/mariusrueve/note-cli/pull/5)).
- `note doctor` checks for directories and bibliography are covered by CLI
  tests. Real-fzf PTY tests now also cover the citation picker (creation and
  editor), the backlinks picker (opening the linking note), and deadlinks
  cancellation; they pass locally and also run against archived binaries.
- Public release verification completed on 2026-10-13: [main CI](https://github.com/mariusrueve/note-cli/actions/runs/38312890305)
  passed for release commit `48128078b665280e28b76a4f88638a41c7e3b6e9`, and the
  [tag-triggered release run](https://github.com/mariusrueve/note-cli/actions/runs/38312905893)
  revalidated and published [v0.2.0](https://github.com/mariusrueve/note-cli/releases/tag/v0.2.0)
  with four archives, `checksums.txt`, `install.sh`, and `build-info.json`. The
  public installer, downloaded without authentication, installed 0.2.0 with a
  verified checksum, and `note self-update` updated a real 0.1.0 release
  installation to 0.2.0 on macOS ARM64.
- `note cite` was smoke-tested read-only against a real 116-entry Better BibTeX
  export in a temporary root: every entry rendered without leftover braces or
  escapes, and the bibliography checksum was unchanged.

The release installation extension adds unit tests for stable version comparison,
bounded release metadata/downloads, checksum and archive validation, gzip trailer
corruption, managed receipts, symlink/ownership guards, lock contention, failure
preservation, cancellation, and daily/offline cache behavior. CLI tests verify
config-independent manual checks and no automatic requests for headless, JSON,
print, help, version, completion, development, disabled, or failed operations.
Native installer tests use real release binaries with local curl fixtures,
including pinned/latest installation, reinstallation, invalid versions, failed
checksums, and preservation of unrelated binaries. Stable archived-binary
terminal fixtures explicitly disable automatic checks and use temporary caches.
Release packaging also selects notices from the actual linked modules and Go
standard library/vendor sources. Unit tests cover bundled notices, determinism,
missing licenses, exclusion of unrelated data, and module replacement refusal;
native archive smoke checks verify the bundled notices are present.

Public release verification completed on 2026-10-06:

- The repository is public and main retains enforced, strict `CI` protection.
- [Main CI](https://github.com/mariusrueve/note-cli/actions/runs/37466480191)
  passed for release commit `ca09cb0aaa046cbcf5383e2a87f3bcd491106a48`.
- [Tag-triggered release CI/CD](https://github.com/mariusrueve/note-cli/actions/runs/37467780272)
  passed every gate and published [v0.1.0](https://github.com/mariusrueve/note-cli/releases/tag/v0.1.0).
  All eight native Go/platform jobs, security/workflow checks, the portable
  build, and all four native archive/installer/terminal checks passed.
- Anonymous downloads of all four archives, the installer, build record, and
  checksums succeeded. Every published checksum verified. The clean local macOS
  build and GitHub Linux build produced byte-identical checksum manifests for
  all four archives, build record, and installer.
- The documented latest-release one-liner installed into an isolated directory.
  The installed binary reported version `0.1.0` and the exact release commit,
  created configuration and a note with no runtime tools on PATH, and completed
  `self-update --check` and `self-update` without authentication. Both correctly
  reported the installed version as current. Replacement/failure paths are
  exercised by the automated update tests; no later release existed to install.
- These publication checks used only ignored workspace data under
  `work/public-release/0.1.0`; no personal installation or knowledge data changed.

Tested tools: Go 1.26.8 and 1.27.1, Git 2.54.0 (Apple Git-157), ripgrep 15.2.0,
fzf 0.74.4, Helix 25.07.1, Fish 4.9.3, system Bash and Zsh. Temporary Go/fzf/Fish
tooling and build/cache data live under ignored `work/`, outside the runtime design.
Official Go/fzf downloads were checksum-verified.

The production-readiness follow-up adds Pixi 0.72.0 with a checked-in four-platform
lock. Local checks use its Go 1.27.2/1.26.7, Git 2.56.0, rg 15.2.0, fzf 0.74.4,
and Fish 4.9.3 environments. Updated module dependencies passed the official
govulncheck scan. The added automated PTY checks exercise real preview rendering,
selection/editor handoff, creation, existing-note preservation, and Ctrl-C.
Release archives now include a checksummed build/dependency record, and clean
source is required for versioned builds. Workflow lint passes locally.

On 2026-10-12 the locked Go 1.27 toolchain moved from 1.27.1 to 1.27.2 on all
four platforms for standard-library fixes (GO-2026-6603, -6605, -6607, -6608,
-6610, -6611, -6613, -6617) reachable through the self-update HTTP client. On
macOS ARM64, `pixi install --locked`, both environments' `check`, `security`, and
`lint-workflows` passed; govulncheck reports no vulnerabilities for Go 1.27.2.
The Go 1.26 compatibility environment stays on 1.26.7, the newest conda-forge
1.26 build at that date; govulncheck still reports these fixes (released in
Go 1.26.9) as reachable there. Release builds and the CI security job use the
default Go 1.27 environment. The
[CI run](https://github.com/mariusrueve/note-cli/actions/runs/38297227124) for
commit `ab5bb7191ecc516a63d1d323a06fc05805044d97` passed on 2026-10-12 with
Pixi 0.72.0, including all native jobs, vulnerability scanning, and workflow lint.

Completed checks:

- Pinned module download/verification; module tidy stability; failing format
  check, vet, full tests, native race tests, and CGO-disabled binary builds.
- Full tests on both supported Go lines. Core tests cover config/schema/XDG,
  templates/escaping, slug/path/symlink/collision rules, exclusive-write races,
  routing, headless modes, existing-note preservation/prompts, editor argv/line
  positions/failure preservation, JSON, ignore rules, distinct literal scopes,
  search provenance/deduplication, recent ordering, and completion fallbacks.
- Real fzf filter selection with NUL IDs, hostile-looking/Unicode/control
  filenames, environment isolation, cancellation, and temporary manifest cleanup.
  Installed preview tests cover quoted binary/manifest paths, schema/ID/containment,
  missing/symlinked/large files, later-line hits, encoding/control sanitization,
  and bounded output.
- A built binary runs outside the checkout with PATH containing no note tools,
  using embedded templates and preview without a language runtime/module cache.
- The real Git matrix uses a local bare remote and two temporary clones: unchanged
  and remote-only clones, unpublished commits, full-repository attachments/deletions,
  independent edits, conflict/add-add preservation, continue/abort/retry recovery,
  ignored exact/ancestor/case/Unicode collisions, personal stash preservation,
  fetch/hook/identity/push failures, remote push races, repeated rewrite refusal,
  unsupported/missing/active repository states, worktrees, and stable locks.
- Installed CLI processes verify concurrent-sync refusal, interrupt after commit
  and during a real conflicted rebase, lock release, reported commit IDs, and
  recovery. A headless receiving server works with only an absolute Git executable
  and no local identity/editor/fzf/rg.
- macOS PTY checks: no-argument picker displayed the right preview, Enter opened
  Helix, creation opened Helix, repeated creation asked for Enter and preserved
  contents, and Ctrl-C canceled the picker/prompt with 130.
- Generated Bash/Zsh/Fish scripts loaded and completed a root directory through
  their actual callback (`scripts/completion-smoke.sh`). Zsh Tab was also exercised
  in an interactive PTY. The automated Zsh smoke stubs only its final UI sink.
- Isolated fresh-source dependency download/verification and local Go installation.
  Editing the copied source left the existing binary unchanged until reinstall;
  reinstall changed the version stamp. No user installation/config was modified.
- All four target archives build locally, contain executable `note`, README, and
  MIT license, and pass SHA-256 checks. The ARM64 macOS archive runs with stamped
  version/commit metadata. Local snapshots include uncommitted implementation
  changes and are development artifacts, not published releases.

GitHub Actions verification:

- [Full passing CI run](https://github.com/mariusrueve/note-cli/actions/runs/37439390788)
  for commit `2d7664c4e3fe3c99acf6f5d9d07f90af5507b6a7` completed on 2026-10-06.
- All eight native jobs passed: macOS ARM64/Intel and Linux ARM64/AMD64, each
  with locked Go 1.26.7 and 1.27.1. Each ran module verification/tidy, formatting,
  vet, coverage, the full integration suite, native race checks, real-terminal
  preview/editor/cancellation checks, three shell completions, and installation.
- Vulnerability scanning and workflow lint passed. Reachable vulnerabilities:
  none reported by the current Go vulnerability database. Detailed coverage
  profiles and function reports are retained as workflow artifacts; the local
  primary-toolchain total was 72.3% (subprocess coverage is not attributed back
  to the parent test process).
- All four cross-built archives passed SHA-256/content checks and native smoke
  tests on their matching platform, including stamped version/commit, creation
  with no runtime tools on PATH, and PTY tests of the actual archived executable.
- CI exposed and resolved a hostname-dependent missing-identity fixture and a
  cancellation edge case involving an interrupt-resistant descendant retaining
  output pipes. The regression now deliberately ignores SIGINT and verifies
  bounded cancellation, preserved commits, released locks, and Git recovery.

The native completion, preview, installation, archive and final acceptance
checkboxes in the [v1 specification](history/v1-spec.md) are complete. The initial production-readiness checkpoint
prepared the source and verified archives for the first 0.1.0 release. The Release workflow repeats every gate for versioned
archives; manual dispatch validates without publishing, and a version tag
publishes only after success. Publication was subsequently explicitly authorized
and completed as recorded above. See [releasing](releasing.md) for the procedure.

Current checks and release rehearsals remain visible on the
[Actions page](https://github.com/mariusrueve/note-cli/actions); use the run's
commit SHA when associating a binary with its verification results.
