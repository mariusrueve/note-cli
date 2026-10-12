# Working on note

`note` is a small Go/Cobra CLI for a plain-Markdown knowledge base: files and
folders plus an editor, rg, fzf, and Git. Product direction and the roadmap are in
PLAN.md; the archived v1 specification is docs/history/v1-spec.md.

## Product boundaries

- No AI/LLM features, databases, persistent indexes, daemons, or GUI.
- Never add metadata automatically; projects are ordinary folders.
- Never overwrite existing notes, traverse symlinks below the canonical root, or
  execute note/query text.
- Discovery respects rg's ignore rules; sync includes the entire dedicated Git repository.

## Contracts

Preserve the CLI, config, template, and exit-code contracts. They live in
docs/cli.md (commands, output, exit codes), docs/configuration.md,
docs/templates.md, docs/sync.md, and docs/completion.md. Rationale and finer
details are in docs/architecture.md and docs/history/v1-spec.md; docs/ wins on
conflict. Routine internal choices are welcome. Keep responsibilities small and direct.

## Code map

- `cmd/note`: signal setup, fresh command tree, error report, exit code.
- `internal/cli`: Cobra routing, flags, prompts, output policy, completion;
  `update.go` handles self-update routing and daily hints.
- `internal/config`: XDG resolution, TOML decoding, defaults, validation.
- `internal/errs`: errors carrying public exit codes.
- `internal/notes`: root containment, symlink checks, basename slugging, exclusive create.
- `internal/templates`: embedded `defaults/*.md`, user templates, fixed placeholders.
- `internal/search`: rg discovery, path/content matching, ordering, JSON records.
- `internal/links`: Markdown link extraction for backlinks and deadlinks (no index).
- `internal/picker`: fzf manifest, numeric IDs, hidden preview helper.
- `internal/process`: direct argv execution, process groups, cancellation.
- `internal/syncgit`: dedicated-repo preflight, lock, commit-first fetch/rebase/push.
- `internal/update`: release metadata, verified archives, atomic self-replacement.
- `cmd/release`: release archives, checksums, dependency notices.

## Commands

Fast loop while editing:

```sh
pixi run --locked go test ./internal/search -run TestName
pixi run --locked fmt
pixi run --locked go run ./cmd/note --help
```

Before finishing: `pixi install --locked`, `pixi run --locked check`,
`pixi run --locked -e go126 check`, `pixi run --locked security`, and
`pixi run --locked lint-workflows`. Check includes module verification/tidy,
formatting, vet, coverage, native race tests, build, completion, and installation.
Integration tests require Git, rg, fzf and a native C compiler for race tests.
Use CGO_ENABLED=0 for shipped builds, and CGO enabled for race tests.

## Tests and data safety

Use temporary roots, isolated Git configuration, and local bare remotes for tests.
Never mutate personal knowledge data or publish as part of development.
Sync commits before fetching/rebasing, never manipulates stashes or force-pushes,
and leaves conflicts to ordinary Git recovery. Keep the lock inode stable.

## Definition of done for public behavior changes

- Tests beside the changed code, then the full checks above.
- The matching docs/*.md contract, README if user-facing, and CHANGELOG.md (Unreleased).
- The roadmap checkbox in PLAN.md, checked only when work and verification are complete.
- Keep unchecked verification items honest in PLAN.md and docs/verification.md;
  record only checks that actually ran.

## Releases and dependencies

Commit pixi.lock; never silently regenerate it in CI. Only release tags publish;
manual release workflow dispatch is a validation run. Publication must be part
of the user's explicit request, separate from ordinary development tests.
