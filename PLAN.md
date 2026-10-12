# PLAN.md — `note` roadmap

Status: v0.1.0 is published and verified on all four native targets (see
[verification](docs/verification.md)). Updated: 2026-10-12.

The original v1 build specification is archived in
[docs/history/v1-spec.md](docs/history/v1-spec.md). Current public contracts live
in `docs/`. This file holds product direction and the prioritized roadmap only.
Checkboxes stay unchecked until the work, docs, and verification are complete.

## Purpose

A distraction-free, plain-Markdown knowledge base used for:

- everyday notes on any topic, captured with minimal friction;
- PhD research notes (literature, experiments, supervisor meetings, ideas);
- context on remote servers over SSH, synced with `note sync`.

## Product boundaries

- Notes are ordinary Markdown files in ordinary folders; projects are folders.
- No AI/LLM features, embeddings, databases, persistent indexes, daemons, or GUI.
- No automatic metadata or frontmatter; text appears only where a user template puts it.
- Never overwrite notes; never rename notes automatically.
- One root per configuration; Git changes happen only through `note sync`.

## Decisions

- **No agent-specific tooling.** Agents already read the knowledge root well with
  `rg`, `find`, and `cat`, and `note search --json` provides structured results.
  No MCP server, `note context`, or agent commands.
- **No daily/journal commands.** Dated capture is covered by the inbox default and
  the `{{date}}` placeholder.
- Every roadmap item specifies its contract in the matching `docs/` file before
  implementation and ships with tests, docs, and a CHANGELOG entry.

## Roadmap

Ordered by value and effort; implement one item per change.

### 1. Research templates

- [x] Embedded templates `paper`, `experiment`, `meeting`, and `idea` in
  `internal/templates/defaults/`, overridable like existing templates.
- [x] New fixed placeholder `{{date}}`: local date `YYYY-MM-DD` from the injected
  clock. Unknown-placeholder behavior is unchanged.
- [x] Document in `docs/templates.md`; `note templates` lists the new entries.

### 2. Scoped views: `--in DIR`

- [x] `--in DIR` for `find`, `grep`, `search`, `recent`, the bare `note` picker,
  `open` without a path, and `note QUERY`.
- [x] DIR is root-relative and passes the existing path-safety rules (no absolute
  paths, `..`, symlinks, nested repositories, or `.git`).
- [x] Results remain root-relative, so `--print`, JSON, and `note open` are unchanged.
- [x] Invalid DIR syntax exits 2; a nonexistent or non-directory DIR, or a
  spelling that differs from the on-disk name, exits 1; symlinked or nested-repo
  directories exit 3 like note paths.
- [x] Shell completion suggests directories; document in `docs/cli.md`.

### 3. Inbox default for capture

- [x] `note new` without PATH creates `<inbox_dir>/<slug(--title)>.md`, or
  `<inbox_dir>/YYYY-MM-DD-HHMMSS.md` (local time) without `--title` or when the
  title has no ASCII letters or digits; `/` in titles never adds folders.
- [x] New config key `inbox_dir` (default `"inbox"`, root-relative, validated like
  note paths); documented in `docs/configuration.md` and `examples/config.toml`.
- [x] Exclusive creation and existing-file rules (exit 4, `--open-existing`) apply
  unchanged; nothing is ever overwritten.
- [x] Filing remains manual (`git mv`/`mv`); rename with link updates stays later work.

### 4. Backlinks and dead links

- [x] `note backlinks PATH`: read-only, computed at invocation from ordinary
  relative Markdown links (inline and reference style), with `--print`/`--json`,
  `--in DIR`, and the standard exit codes (1 when there are none).
- [x] `note deadlinks [--in DIR]` (named instead of `links --check`): report local
  link targets that do not exist with their exact spelling; no network access;
  anchors are not checked. Exit 1 when broken links exist.
- [x] No index, no wiki-links, no rewriting. Documented in `docs/cli.md`.

### 5. Zotero / Better BibTeX citations

- [ ] Config `bibliography` (path to a Better BibTeX auto-exported `.bib` file)
  and `literature_dir` (default `"literature"`).
- [ ] Minimal read-only BibTeX reader in `internal/bib` (braced and quoted values,
  `@string`-free entries are sufficient); the `.bib` file is never modified.
- [ ] `note cite`: fzf over entries; selecting opens `<literature_dir>/<citekey>.md`
  or creates it from the `paper` template. `note new PATH --cite KEY` fills a
  template from one entry.
- [ ] Placeholders `{{citekey}}`, `{{authors}}`, `{{year}}`, `{{doi}}`, `{{url}}`;
  `{{title}}` comes from the entry unless `--title` is given. Empty when absent.
- [ ] Document in `docs/templates.md`, `docs/configuration.md`, and `docs/cli.md`.

## Later extensions

Unprioritized; each needs its own specification first.

- Rename with relative-link updates.
- Markdown lint/format integration (mutating only when requested).
- Read-only Git status and clearer conflict diagnostics.
- Editor adapters for line jumps beyond the current editors.
- Homebrew or distribution packages.
