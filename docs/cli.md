# CLI contract

Global options: `--config PATH`, `--help`, `--version`. Put global flags before
the subcommand in scripts. Help, version, and completion generation need no config
or tools. Quote queries containing spaces: each query is one argument. Empty
queries, NUL, and line breaks are invalid. Searches are literal substrings;
additional filtering in fzf is fuzzy.

| Command | Behavior/options |
| --- | --- |
| `note` / `note open` | Pick from discovered Markdown notes |
| `note QUERY` | Exact existing path (optional `.md`), exact discoverable basename/stem, then relative-path substring; one opens, multiple pick, none offers creation |
| `note open PATH` | Literal existing root-relative filename, optional `.md`; supports ignored notes and preserves legacy spelling |
| `note new PATH` | Normalize basename, create exclusively, open; `--template NAME`, `--title TITLE`, `--no-open`, `--open-existing` |
| `note find QUERY` | Paths only; `--case-sensitive`, `--print`, `--json` |
| `note grep QUERY` | Contents only, one result per matching line; same options |
| `note search QUERY` | Path/content union, one result per note with first content line; same options |
| `note recent` | Newest mtime first, path order for ties; `--limit N` (20, positive), `--print`, `--json` |
| `note templates` | Effective templates, sources, embedded overrides, default |
| `note init` | Create config/root; `--root PATH` (`~/Knowledge`), `--editor EXECUTABLE` (`hx`) |
| `note doctor` | Read-only effective config/tools/root/Git readiness/completion guidance |
| `note completion SHELL` | Print Bash, Zsh, or Fish script |
| `note sync` | Commit-first sync; `--message TEXT` literal nonempty override |

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
