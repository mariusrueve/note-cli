# Working on note

Preserve the CLI, config, template, and exit-code contracts in PLAN.md and docs.
Routine internal choices are welcome. Keep responsibilities small and direct.
Never overwrite existing notes, traverse symlinks below the canonical root,
execute note/query text, or add metadata automatically. Discovery respects rg's
ignore rules; sync includes the entire dedicated Git repository.

Use temporary roots, isolated Git configuration, and local bare remotes for tests.
Never mutate personal knowledge data or publish as part of development.
Sync commits before fetching/rebasing, never manipulates stashes or force-pushes,
and leaves conflicts to ordinary Git recovery. Keep the lock inode stable.

Use `pixi install --locked`, `pixi run --locked check`,
`pixi run --locked -e go126 check`, `pixi run --locked security`, and
`pixi run --locked lint-workflows`. Check includes module verification/tidy,
formatting, vet, coverage, native race tests, build, completion, and installation.
Integration tests require Git, rg, fzf and a native C compiler for race tests.
Use CGO_ENABLED=0 for shipped builds, and CGO enabled for race tests.
Update relevant docs and CHANGELOG.md when changing public behavior.
Keep unchecked verification items honest in PLAN.md and docs/verification.md.
Commit pixi.lock; never silently regenerate it in CI. Only release tags publish;
manual release workflow dispatch is a validation run. Publication must be part
of the user's explicit request, separate from ordinary development tests.
