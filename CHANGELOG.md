# Changelog

## Unreleased

## [0.2.0] - 2026-10-12

- Configuration errors for unknown keys now name each key and its line, and
  suggest updating `note` when the key comes from a newer version.
- Add `note cite [KEY]` and `note new --cite KEY` for literature notes from a
  read-only BibTeX/BibLaTeX bibliography (e.g. a Zotero Better BibTeX export),
  new `bibliography` and `literature_dir` settings, and the `{{citekey}}`,
  `{{authors}}`, `{{year}}`, `{{doi}}`, and `{{url}}` placeholders used by `paper`.
- Add `note backlinks PATH` and `note deadlinks`, computed from ordinary relative
  Markdown links at invocation time. Dead-link checks require exact on-disk
  spelling and exit 1 when broken links exist.
- `note new` without PATH creates a capture note in the new `inbox_dir` setting
  (default `inbox`), named after `--title` or the local timestamp.
- Add `--in DIR` to the picker, `note QUERY`, `open`, `find`, `grep`, `search`,
  and `recent` to limit results to notes below a root-relative directory.
  Results stay root-relative; completion suggests directories.
- Add embedded research templates `paper`, `experiment`, `meeting`, and `idea`,
  and a fixed `{{date}}` placeholder for the local creation date (`YYYY-MM-DD`).
- Build with Go 1.27.2 to pick up standard-library security fixes in
  `net/http`, `net/textproto`, and `crypto/tls` reachable from `self-update`
  (GO-2026-6603, -6605, -6607, -6608, -6610, -6611, -6613, -6617).

## [0.1.0] - 2026-10-06

- Add a checksum-verified macOS/Linux release installer, explicit `self-update`
  and `--check`, daily cached interactive update hints with an opt-out, and
  protection of installations owned by source installers/package managers.
- Bundle Go standard-library and linked dependency license/notice files in
  release archives, and keep stable archive terminal tests offline and isolated.
- Add `note` with XDG/TOML configuration, initialization, safe exclusive creation,
  ASCII basename slugging, literal legacy opening, and embedded/user templates.
- Add shared ignore-aware discovery, numeric-ID fzf selection and source preview,
  path/content/combined literal search, recent notes, print/JSON, and shortcut lookup.
- Add Bash/Zsh/Fish completion and read-only doctor diagnostics.
- Add commit-first dedicated-repository Git sync with locking, guarded fetch/rebase,
  explicit upstream push, stash preservation, and normal Git conflict recovery.
- Add local installation, four-target snapshots/checksums, native CI configuration,
  integration tests, and complete user/developer documentation.
- Add locked four-platform Pixi development environments for Go 1.26/1.27,
  native terminal tests, coverage reports, dependency/security scans, pinned
  Actions, and gated tag-based release automation with build/dependency metadata.
- Bound canceled search/Git subprocesses and clean up interrupt-resistant hooks
  and wrappers without losing committed work or Git recovery state.
