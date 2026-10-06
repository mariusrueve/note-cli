# Verification record

Implementation and local checks performed on 2026-10-06, macOS ARM64.
This records executed checks separately from configured future CI runs.

Tested tools: Go 1.26.8 and 1.27.1, Git 2.54.0 (Apple Git-157), ripgrep 15.2.0,
fzf 0.74.4, Helix 25.07.1, Fish 4.9.3, system Bash and Zsh. Temporary Go/fzf/Fish
tooling and build/cache data live under ignored `work/`, outside the runtime design.
Official Go/fzf downloads were checksum-verified.

The production-readiness follow-up adds Pixi 0.72.0 with a checked-in four-platform
lock. Local checks use its Go 1.27.1/1.26.7, Git 2.56.0, rg 15.2.0, fzf 0.74.4,
and Fish 4.9.3 environments. Updated module dependencies passed the official
govulncheck scan. The added automated PTY checks exercise real preview rendering,
selection/editor handoff, creation, existing-note preservation, and Ctrl-C.
Release archives now include a checksummed build/dependency record, and clean
source is required for versioned builds. Workflow lint passes locally.

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

Pending external/native verification:

- The GitHub Actions configuration covers Go 1.26.x/1.27.x on native macOS ARM64,
  macOS Intel, Linux AMD64, and Linux ARM64. These remote jobs have not been run
  in this local implementation session. No release was published.
- Native execution on Intel macOS and both Linux architectures, plus interactive
  Linux picker/editor/completion smoke checks, remain required before declaring
  those release targets verified. Cross-compilation alone is not execution proof.
- A v1 publication/tag/versioned remote-install workflow remains a separate action.

The plan's cross-platform completion/preview and final all-platform acceptance
checkboxes remain unchecked for these reasons. The CLI is ready for local source
installation and macOS ARM64 use; CI and the manual recipes provide the remaining
platform checks.
