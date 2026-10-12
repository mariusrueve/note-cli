# CLI contract

Global options: `--config PATH`, `--help`, `--version`. Put global flags before
the subcommand in scripts. Help, version, and completion generation need no config
or tools. Quote queries containing spaces: each query is one argument. Empty
queries, NUL, and line breaks are invalid. Searches are literal substrings;
additional filtering in fzf is fuzzy.

| Command | Behavior/options |
| --- | --- |
| `note` / `note open` | Pick from discovered Markdown notes; `--in DIR` |
| `note QUERY` | Exact existing path (optional `.md`), exact discoverable basename/stem, then relative-path substring; one opens, multiple pick, none offers creation; `--in DIR` |
| `note open PATH` | Literal existing root-relative filename, optional `.md`; supports ignored notes and preserves legacy spelling |
| `note new [PATH]` | Normalize basename, create exclusively, open; without PATH, create in `inbox_dir`; `--template NAME`, `--title TITLE`, `--no-open`, `--open-existing`, `--cite KEY` |
| `note cite [KEY]` | Open or create `literature_dir/<key>.md` for a bibliography entry; fzf picker without KEY; `--template NAME` (`paper`), `--no-open` |
| `note find QUERY` | Paths only; `--case-sensitive`, `--print`, `--json`, `--in DIR` |
| `note grep QUERY` | Contents only, one result per matching line; same options |
| `note search QUERY` | Path/content union, one result per note with first content line; same options |
| `note backlinks PATH` | Notes linking to an existing note, one result per link; `--print`, `--json`, `--in DIR` |
| `note deadlinks` | Local links whose target is missing; exit 1 when any exist; `--print`, `--json`, `--in DIR` |
| `note recent` | Newest mtime first, path order for ties; `--limit N` (20, positive), `--print`, `--json`, `--in DIR` |
| `note templates` | Effective templates, sources, embedded overrides, default |
| `note init` | Create config/root; `--root PATH` (`~/Knowledge`), `--editor EXECUTABLE` (`hx`) |
| `note doctor` | Read-only effective config/tools/root/Git readiness/completion guidance |
| `note completion SHELL` | Print Bash, Zsh, or Fish script |
| `note sync` | Commit-first sync; `--message TEXT` literal nonempty override |
| `note self-update` | Install the latest stable release for a release-installer installation; `--check` checks availability without replacing the executable |

`note new` without PATH captures into the configured `inbox_dir` (default
`inbox`). With `--title`, the basename is the title's normalized slug, with `/`
treated as a space so the title never adds folders. Without `--title`, or when the
title has no ASCII letters or digits, the basename is the local time
`YYYY-MM-DD-HHMMSS`; an untitled note's heading is `YYYY-MM-DD HH:MM:SS`. All
existing-note rules apply unchanged (a same-second repeat returns 4 headlessly).
File notes later with `mv` or `git mv`.

`note cite KEY` reads the configured `bibliography` (exit 3 when unset or
unreadable, exit 1 for an unknown key; keys are exact and case-sensitive). The
note path is `literature_dir/KEY` with the usual basename normalization, so
`smithDeepLearning2024` becomes `smithdeeplearning2024.md`; the exact key is
available as `{{citekey}}`. An existing note opens without a prompt and is never
retemplated; with `--no-open` it returns 4. Without KEY, a terminal shows an fzf
picker of key, year, authors, and title, marking entries that already have a
literature note with `✓`. The preview shows title, authors, year, venue, DOI, URL,
and abstract with controls escaped, read through a private temporary manifest.
Headless use exits 2. Completion offers
keys. `note new PATH --cite KEY` fills any template from the entry (default
template `paper`); without PATH it uses `literature_dir/KEY`.

Backlinks and deadlinks read ordinary Markdown links at invocation time; there is
no index and nothing is rewritten. They recognize inline links and images
(`[text](target)`, `![alt](target)`, `<target>` form, optional titles) and
reference definitions (`[label]: target`), skipping fenced code blocks and inline
code spans. URLs with a scheme, `//host` links, and anchor-only links are not
local. Fragments and queries are ignored, percent-escapes are decoded, targets
resolve relative to the linking note, and a leading `/` means the root. Wiki
links are not recognized. Notes larger than 8 MiB are skipped with a warning.

`backlinks PATH` takes the same literal path as `open`; a missing note exits 1,
as do zero backlinks. `--in DIR` limits the linking notes. Printed lines are
`path:line:source line`; in a terminal the picker opens the linking note at its line.

`deadlinks` checks every local target, including attachments and directories,
without following symlinks and with the exact on-disk spelling, so links that
only work on case-insensitive filesystems are reported. Problems are `missing`,
`case differs`, `outside root`, or `symlink`. Printed lines are
`path:line:target (problem)`; JSON results carry the target as `snippet` and a
`problem` field. `--in DIR` limits the notes checked, not the targets. Headless or
with `--print`/`--json`, broken links exit 1 and none exit 0 (`[]` for JSON,
`No broken links.` on stderr otherwise), so it suits scripts before `note sync`.
In a terminal, broken links open the picker at the link. Both commands report
`match_kinds` as `["link"]`.

`--in DIR` limits the picker, `note QUERY`, find/grep/search, and recent to
discoverable notes below a root-relative directory, e.g. `note search --in phd
fingerprint`. One trailing slash is accepted. Results, `--print`, and JSON keep
root-relative paths. Discovery and its ignore rules still start at the root.
Invalid directory syntax exits 2. A missing directory, a non-directory, or a
spelling that differs from the on-disk name (including case) exits 1. Symlinked
or nested-repository directories are rejected like note paths (exit 3).
With `note --in DIR QUERY`, an exact path opens only when it lies inside DIR, and
confirmed creation makes `DIR/QUERY`. `note open --in DIR` applies only to the
picker; combining it with PATH exits 2. Completion offers directories.

`--print` and `--json` are mutually exclusive. `--no-open` and `--open-existing`
are mutually exclusive. Path arguments reject absolute paths, `.`, `..`, empty
segments, NUL, and `.git`; creation additionally rejects controls. Note paths may
not traverse symlinked files/directories or nested Git repositories. New paths
preserve directory names, normalize only the basename, and check legacy case
collisions even on Linux. Existing names are never renamed automatically.

Registered command names, `help`, `completion`, and internal commands are
reserved: open `search.md` using `note open search.md`. Exact lookup does not
slugify legacy names. Creation through the shortcut requires `y` or `yes`; other
answers cancel. Without a terminal, no match returns 1 with a `note new PATH` hint.

Existing `new` destinations are never truncated or retemplated. In a terminal,
the notice asks for empty Enter to open; other input repeats, EOF/Ctrl-C cancels.
Headless use returns 4 unless `--open-existing` authorizes opening. `--no-open`
also returns 4 for existing notes. Editor failures retain newly written notes.

Search/recent automatically print when stdin/stdout are not terminal streams.
`--print`/`--json` bypass fzf. Pathless pickers require a terminal. Empty results
launch no picker/editor. Data uses stdout; diagnostics/status/prompts use stderr.
JSON never contains subprocess output. Human output escapes controls visibly.
Release builds may emit cached update hints on stderr after successful
interactive operations. Help, version, completion, scripts, and `--json`/`--print`
are quiet. See [installation](installation.md) for network/cache behavior and
`NOTE_NO_UPDATE_CHECK`.
Use JSON for unambiguous filenames, including names containing line breaks.

Printed find/search/recent output is one relative path per line; grep output is
`path:line:snippet`. JSON is an array (empty is `[]`) with `path`, `match_kinds`
(array of `path`, `content`, both, or empty for listings), `line` (one-based or
null), and `snippet` (string or null). Recent adds integer `mtime_ns`.

```json
[{"path":"docker/networking.md","match_kinds":["content"],"line":12,"snippet":"Docker bridge network"}]
```

Find is sorted by path; grep by path/line; search places path matches first, then
content-only matches, each group sorted by path. The picker starts with an empty
query so content-only hits remain visible. Discovery uses rg's normal ignore and
hidden-file policy, skips symlinks/nested repositories and non-UTF-8 filenames,
and retains `.md` extensions case-insensitively. No whitelist glob overrides
ignore rules. Unicode case folding in path matching and ripgrep content matching
can differ for some characters; ASCII and German examples are tested.

| Exit | Meaning |
| --- | --- |
| 0 | Success |
| 1 | No matches, empty collection, or missing requested note |
| 2 | Invalid arguments/options/query/path syntax or unavailable terminal for selection |
| 3 | Config/filesystem/dependency/editor/process/ordinary Git failure |
| 4 | Existing new destination without authorized opening |
| 5 | Unresolved Git conflicts or active conflicting operation |
| 130 | Canceled picker/prompt/operation |
