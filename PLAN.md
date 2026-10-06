# PLAN.md — `note`

Status: v1 implemented and verified locally on macOS ARM64; native CI and Linux/Intel macOS checks remain pending. Updated: 2026-10-06. See docs/verification.md for executed checks and remaining platform work.

This document is a self-contained starting point for a new repository. A coding agent should be able to implement, test, package, and document the tool without the original conversation. Checkboxes must remain unchecked until the work and its verification are complete.

## 1. Product and agreed requirements

`note` is a small Go CLI built with Cobra for a personal Markdown knowledge base. It connects ordinary files and folders to an editor, ripgrep, fzf, and Git. The knowledge base remains fully usable with other editors and Markdown tools.

### Agreed product behavior

- The executable is named `note`. Use Go and Cobra.
- Support macOS and Linux, with one configured knowledge root per configuration.
- Users choose their folder structure. Projects are ordinary folders, without a separate model or specialized commands.
- Create new Markdown basenames in ASCII kebab-case. Never rename existing files automatically. Preserve Unicode in note contents and titles.
- `note new PATH` creates a missing note and opens it. If the file exists, display a notice and let Enter open it.
- Support templates without automatically adding metadata or YAML frontmatter.
- `note` without arguments opens fzf, with a preview on the right.
- `note find` searches filenames and relative paths only; `note grep` searches contents only; `note search` searches both.
- Search commands use literal substring matching. Additional interactive filtering inside fzf uses fuzzy matching.
- `note recent` selects notes ordered by file modification time.
- `note QUERY` offers convenient lookup, selection, and explicitly confirmed creation when nothing matches.
- Configure the editor, defaulting to `hx`.
- Respect XDG configuration paths on both operating systems.
- `note sync` stages changes, creates an automatic commit if needed, pulls with rebase, and pushes. A clean second computer or server uses the same command to receive updates.
- Sync the entire dedicated knowledge repository, including nonignored attachments and deletions.
- Provide a self-contained source repository, documentation, tests, straightforward local installation, and an easy modification/update workflow. Use the MIT license.
- No AI functionality in v1.

Recording local changes before pulling lets ordinary Git commits and rebase state handle recovery without a custom stash journal.

### Technical defaults

These choices guide implementation. Internal structure may change when that makes the code simpler, provided the public contracts remain intact.

| Topic | v1 decision |
| --- | --- |
| Toolchain | Minimum Go 1.26; use supported patch releases and also test Go 1.27 |
| Module | One Go module; executable entry point in `cmd/note`; module: `github.com/mariusrueve/note-cli` |
| CLI | Cobra, with explicit argument validation and a fresh command tree per test |
| Configuration | TOML, initialized by `note init`; no Viper |
| Dependencies | Cobra, a small TOML decoder, and only necessary `golang.org/x` packages |
| Discovery | ripgrep, respecting ignore rules; no persistent search index |
| Links | Ordinary relative Markdown links; v1 does not analyze or rewrite them |
| Templates | Fixed `{{title}}`, `{{slug}}`, and `{{path}}` placeholders; no executable template language |
| Git integration | Commit before pull; explicit rebase and upstream push; preserve failures for manual recovery |
| Delivery | Local build/install first; portable binary releases after the working core and sync |
| Extensibility | Clear responsibilities and documented contracts; no plugin framework |

### Outside v1

No LLMs, API keys, embeddings, semantic search, or `note ask`. No databases, daemons, cloud services, telemetry, GUI, simultaneous multiple roots, or specialized project/meeting/journal commands. No automatic commit on editor save: Git changes only through `note sync`. Link checking, backlinks, rename with link updates, and Markdown formatting are later work.

Do not add hidden management data to notes. Configuration and user templates live outside the knowledge root by default. Repository customization should be easy without building an extension framework in advance.

## 2. Typical workflow

```bash
# Create the root and local configuration once.
note init --root ~/Knowledge

# Create a note from the default template and open hx.
note new docker/networking

# Keep the filename ecfp.md while using a specific title.
note new cheminformatics/ecfp --template concept --title "ECFP"

# Open an existing note directly, or select from all notes.
note open docker/networking
note

# Convenience lookup; projects remain ordinary folders.
note docker
note new projects/poseedit/architecture

# Choose the search scope explicitly.
note find docker
note grep "extended connectivity"
note search fingerprint

# Select recently modified notes, or sync the repository.
note recent
note sync

# After shell completion is installed, type a directory prefix
# and press Tab: note new chemin  ->  cheminformatics/
```

Example knowledge root:

```text
~/Knowledge/
├── .git/
├── .gitignore
├── docker/
│   └── networking.md
├── cheminformatics/
│   ├── ecfp.md
│   └── molecular-fingerprints.md
├── attachments/
│   └── fingerprint-example.png
└── projects/
    └── poseedit/
        └── architecture.md
```

Markdown discovery and search cover notes. Git sync covers the dedicated repository, including attachments.

## 3. CLI contract

### 3.1 Commands and options

Document global options before the subcommand, for example `note --config /path/config.toml find docker`. Cobra may also accept persistent flags in its other supported positions. Help, version, and completion-script generation must work without a configured root or external tools.

| Invocation | Behavior |
| --- | --- |
| `note` | Select from all discoverable Markdown files in fzf; Enter opens the selection |
| `note QUERY` | Resolve a convenient lookup as specified in section 3.4 |
| `note open [PATH]` | Open an existing path; without a path, use the same picker as `note` |
| `note new PATH` | Normalize the new basename, render a template, create exclusively, and open |
| `note find QUERY` | Filter relative paths only and select a matching note |
| `note grep QUERY` | Search contents only and select a matching line |
| `note search QUERY` | Combine path and content matches; show each note once |
| `note recent` | Select notes ordered by modification time, newest first |
| `note templates` | List available template names, sources, overrides, and the default |
| `note completion SHELL` | Write a completion script for `bash`, `zsh`, or `fish` to stdout |
| `note init` | Create the root and initial configuration; do not initialize Git |
| `note doctor` | Report configuration, root, tool, completion, and Git prerequisites |
| `note sync` | Run the Git workflow in section 9 |

Options:

- Global: `--config PATH`, `--help`, `--version`.
- `init`: `--root PATH` (default `~/Knowledge`) and `--editor EXECUTABLE` (default `hx`). Refuse to overwrite an existing configuration. An existing root may be reused without changing its files.
- `new`: `--template NAME`, `--title TITLE`, `--no-open`, `--open-existing`. The last two are mutually exclusive.
- `find`, `grep`, `search`: `--case-sensitive`, `--print`, `--json`.
- `recent`: `--limit N` (default 20; positive integer), `--print`, `--json`.
- `sync`: `--message TEXT` to override the automatic commit message with literal text.

`--print` and `--json` are mutually exclusive. Each query is one argument; quote queries containing spaces. Empty queries and queries containing line breaks are usage errors. Regex search is outside v1.

### 3.2 Terminal behavior, output, and exit codes

Search commands and `recent` use fzf when an interactive terminal is available. Without one, or when stdout is redirected, they print results automatically. `--print` and `--json` bypass fzf even in a terminal. `note` and `open` without a path require an interactive terminal and otherwise report how to use the output commands.

Printed `find` and `search` results contain one root-relative path per line. `grep --print` uses `path:line:snippet`. `recent --print` prints paths in recency order. Escape control characters visibly in human-readable output; programs that need unambiguous paths must use JSON.

JSON is an array of objects with `path`, `match_kinds`, `line`, and `snippet`. Paths are relative to the root. Lines are one-based or `null`; snippets are strings or `null`. `match_kinds` contains `path`, `content`, both, or an empty array for listings without a search match. `recent` also includes integer `mtime_ns`. Empty results must serialize as `[]`, never `null`.

```json
[{"path":"docker/networking.md","match_kinds":["content"],"line":12,"snippet":"Docker bridge network"}]
```

Write result data to stdout and diagnostics, status, warnings, and prompts to stderr. Subprocess output must not contaminate JSON. Empty result sets launch neither fzf nor the editor.

| Exit code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | No matches, empty collection, or missing requested note; JSON output is `[]` |
| `2` | Invalid arguments, options, query, or path syntax |
| `3` | Configuration, filesystem, dependency, editor, subprocess, or ordinary Git failure |
| `4` | `new` found an existing file and opening was not explicitly authorized |
| `5` | Unresolved Git conflicts or a conflicting Git operation already in progress |
| `130` | User canceled a picker, prompt, or running operation |

Treat ripgrep exit 1 as no matches, not an execution failure. Map editor failures to 3 while retaining any newly created note. Canceled selection and unconfirmed prompts leave files unchanged. A cancellation after creation or a Git mutation follows the preservation rules for that operation.

### 3.3 Creating and opening notes

Creation sequence:

1. Load and validate configuration and arguments; derive and validate the destination.
2. If it exists, use the existing-file behavior below. Do not require or render a template for this case.
3. Otherwise, resolve and validate the template. Validate the editor executable before creating anything unless `--no-open` is set.
4. Create missing parent directories, then create the file exclusively.
5. Write the rendered UTF-8 contents and check both write and close errors.
6. Open the configured editor unless `--no-open` is set.

Use `os.OpenFile` with `O_WRONLY | O_CREATE | O_EXCL` and mode `0666`, subject to the user's umask. A concurrent creation returning `fs.ErrExist` enters the same existing-file handler. Never truncate an existing file. On a failed write, remove only the incomplete file created by this invocation; an editor failure keeps a successfully written note.

Existing-file prompt:

```text
Note docker/networking.md already exists.
Press Enter to open it, or Ctrl-C to cancel:
```

Only an empty Enter response opens the file; other input repeats the prompt. EOF or Ctrl-C cancels with 130. Without an interactive terminal, return 4 unless `--open-existing` explicitly requests opening. `--no-open` returns 4 for an existing file; it does not claim a new file was created. Directories, symlinks, and unsafe paths are errors. Neither `--title` nor `--template` changes an existing note.

`open PATH` accepts a literal root-relative existing filename, with an optional `.md` extension. It does not create files or normalize legacy filenames. Explicit opening can access an ignored Markdown note, subject to path safety checks. Pass the resolved absolute path as editor data, never as a shell command.

### 3.4 Convenience lookup: `note QUERY`

Resolve in this order:

1. Registered CLI commands take precedence.
2. An exact relative existing path, optionally omitting `.md`, opens directly.
3. If the query contains no slash, check for an exact basename or stem across the discoverable notes; this equality follows the configured case setting.
4. Otherwise, use literal relative-path substring matching with the normal case setting.

One match opens it; multiple matches open fzf. For no match, offer creation with `Create a new note? [y/N]`. Only `y` or `yes` proceeds through the ordinary `new` workflow. Declining or canceling creates nothing. Without a terminal, return 1 and suggest `note new PATH`.

Command names are reserved, including `help`, `completion`, and internal command names. Use `note open search.md` to open a note named after a command. Exact path lookup must not silently slugify a legacy name.

Implement the root as `note [QUERY]` with an explicit maximum-one-argument validator and `RunE`; zero arguments invokes the picker. Test this routing early: Cobra's default argument behavior must not reject the intended query as an unknown command. Build a fresh command tree through a factory, load config lazily, and avoid rewriting `os.Args`. Set `SilenceErrors` and `SilenceUsage`, classify errors consistently, and call `os.Exit` only after deferred cleanup has run. See [Cobra's command API](https://pkg.go.dev/github.com/spf13/cobra).

## 4. Paths, filenames, and discovery

### 4.1 Root and containment

Expand a leading `~` in configured filesystem paths and resolve the root to an absolute canonical directory. The configured root itself may be a symlink; resolve it once. Do not traverse symlinked notes or symlinked directories below that root.

Note arguments are relative to the root. Reject absolute paths, `..`, `.` segments, empty segments, NUL, and control characters in new paths. Do not perform environment-variable or shell expansion. Check containment and symlink components again immediately before opening or creating a file. Do not traverse `.git`, nested repositories, or submodules during note discovery.

Existing Markdown names may contain spaces, Unicode, quotes, or punctuation. Preserve their identity and never infer filesystem paths from a formatted display string. These checks protect normal CLI use; do not claim protection against a hostile process changing the filesystem concurrently.

### 4.2 New basename normalization

Normalize only the final basename. Existing directory names remain unchanged and can be selected through completion.

1. Remove an optional `.md` suffix, case-insensitively.
2. Transliterate German umlauts and sharp s: `ä → ae`, `ö → oe`, `ü → ue`, `ß → ss`, including uppercase forms.
3. Apply Unicode NFKD normalization, remove combining marks, and lowercase.
4. Replace each run outside ASCII `a-z` and `0-9` with one hyphen.
5. Trim leading/trailing hyphens; reject an empty result; append `.md`.

| Input | New path |
| --- | --- |
| `Molecular Fingerprints` | `molecular-fingerprints.md` |
| `docker/Network_Config.md` | `docker/network-config.md` |
| `knowledge/Überblick` | `knowledge/ueberblick.md` |
| `cheminformatics/ECFP` | `cheminformatics/ecfp.md` |

Use `golang.org/x/text/unicode/norm`, with a small explicit German mapping. Slugging must be deterministic and idempotent. For titles whose text produces no usable ASCII slug, request an explicit ASCII basename; `--title` preserves the desired Unicode heading.

Report the actual normalized path when a collision occurs and use the existing-file behavior. Check case-insensitive basename collisions even on Linux, so creating `network.md` does not introduce a second note beside `Network.md`. A single equivalent existing name opens that actual file after confirmation; ambiguous legacy collisions fail clearly. Do not rename either file.

### 4.3 Discoverable notes

Run `rg --no-config --files --null` inside the root, then retain case-insensitive `.md` suffixes. Honor ripgrep's normal `.gitignore`, `.ignore`, `.rgignore`, hidden-file, and symlink behavior. Do not add a whitelist glob that accidentally overrides an ignore rule. Exclude nested Git repositories and submodules explicitly.

Sort by relative path unless another command specifies an order. `recent` uses modification time in nanoseconds descending, then path ascending for ties. If a file disappears during scanning, skip it with a concise warning and revalidate before opening.

Write new notes as UTF-8. Do not rewrite invalid existing text to repair encoding. Preview may report an encoding problem, and search snippets may use replacement characters. Skip non-UTF-8 filenames with a warning rather than producing a lossy path identity in JSON. The search collection is distinct from the full Git sync scope. See [ripgrep's guide](https://github.com/BurntSushi/ripgrep/blob/master/GUIDE.md).

## 5. Configuration and XDG paths

### 5.1 Locations and resolution

Use the same XDG layout on macOS and Linux:

| Item | Location |
| --- | --- |
| Config | `$XDG_CONFIG_HOME/note/config.toml`, falling back to `~/.config/note/config.toml` |
| User templates | `$XDG_CONFIG_HOME/note/templates/`, falling back to `~/.config/note/templates/` |
| Knowledge root | Explicit `root` setting; `note init` defaults to `~/Knowledge` |

Ignore empty or relative XDG environment values and use the fallback; report invalid values in ordinary diagnostics, while keeping completion quiet. Follow the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/).

`--config PATH` selects only the config file. The default template directory remains XDG-based unless `templates_dir` is set. Resolve relative path values inside TOML relative to the config file's directory. Expand leading `~`, but not `$VARIABLE` expressions. Temporary picker files use the operating system's temporary directory. v1 needs no persistent cache or custom sync recovery journal.

### 5.2 TOML schema

```toml
schema_version = 1
root = "~/Knowledge"
editor = "hx"
default_template = "default"

# Optional override; otherwise use the XDG template directory.
# templates_dir = "./templates"

[tools]
fzf = ["fzf"]
rg = ["rg"]
git = ["git"]

[finder]
height = "90%"
preview_window = "right:60%:wrap"

[search]
case_sensitive = false

[sync]
commit_message = "notes: sync {timestamp}"
```

`root` is required; other settings have the defaults shown. Accept the editor as either one executable string or a nonempty argv list:

```toml
editor = ["code", "--wait", "--reuse-window"]
```

A string is one executable, not shell syntax. Reject strings such as `"code --wait"` with guidance to use a list. Tool settings are nonempty argv lists with the executable first. Reject unknown fields, unsupported schema versions, invalid types, and empty executable names. Preserve the distinction between a missing setting and an explicit `false`.

Precedence is defaults → TOML → applicable CLI flags. Do not silently override the configured editor with `EDITOR` or `VISUAL`. Keep configuration validation separate from checking whether a particular command's executables are installed.

Use `github.com/pelletier/go-toml/v2` with unknown-field rejection and explicit semantic validation. Decode the editor union into a validated command specification. Avoid a general configuration framework. See [go-toml's decoder API](https://pkg.go.dev/github.com/pelletier/go-toml/v2).

### 5.3 Initialization and diagnostics

Help, version, `init`, and completion-script generation do not require an existing config. Other commands report a missing config with an actionable `note init` hint. Only `init` creates the root; ordinary commands do not silently recreate a missing knowledge base.

`init` writes a readable, commented config without overwriting one. Create the root and config directories only as needed. It does not install tools, edit shell startup files, initialize Git, or change existing notes.

`doctor` checks the effective configuration, root, configured executables, and Git readiness, and explains how to enable completion. Report missing prerequisites without launching editors or accessing remotes. Basic creation with `--no-open` needs no editor, fzf, ripgrep, or Git. Sync needs Git only, apart from configuration and root validation. No command downloads dependencies automatically.

## 6. Templates

### 6.1 Sources and selection

Embed `default`, `concept`, `reference`, and `empty` Markdown templates in the binary with `go:embed`. An installed binary must work outside its source checkout. Use a resource directory containing `*.md`, including a genuinely empty `empty.md` file. See [Go's embed package](https://pkg.go.dev/embed).

User templates are flat files named `NAME.md` in the template directory. A user template with the same name overrides the embedded one. Select the explicit `--template` value first, then `default_template`, then the built-in default when the config key is absent. An explicitly selected missing or invalid template is an error, never a silent fallback.

Template identifiers may contain letters, digits, hyphens, and underscores. Reject path separators, traversal, absolute paths, and symlinked template files. `note templates` identifies the effective source, any overridden built-in, and the configured default.

### 6.2 Placeholder contract

Support exactly these fixed substitutions:

| Placeholder | Value |
| --- | --- |
| `{{title}}` | Explicit `--title`, or a title derived from the original input basename |
| `{{slug}}` | Normalized basename without `.md` |
| `{{path}}` | Normalized root-relative path including `.md` |

Use a small single-pass renderer, not `text/template` evaluation. No functions, loops, includes, shell commands, environment access, or dynamic dates. Inserted values are not interpreted again. Unknown or malformed unescaped placeholder syntax is an error before file creation.

Use `\{{` in a template to emit a literal `{{` opener; remove that one escape during rendering. Ordinary dollar signs, shell variables such as `$PATH`, and Markdown mathematics such as `$E = mc^2$` remain unchanged. Placeholder names have no whitespace variants: use `{{title}}`, not `{{ title }}`.

Derive a default title from the original basename after removing the extension: turn hyphens/underscores into spaces, collapse whitespace, and uppercase the first letter while preserving the rest. Preserve explicit titles and acronyms such as `ECFP`. `--title` changes the heading only, never the filename.

Default template:

```markdown
# {{title}}

```

The concept template contains `# {{title}}` followed by `## Idea`, `## Explanation`, `## Example`, and `## Sources`. The reference template uses `## Overview`, `## Details`, and `## Sources`. The empty template renders zero bytes. None adds frontmatter or other metadata. User-authored content is preserved as template text; editing a template affects only future creations.

## 7. Search, fzf preview, and editor execution

### 7.1 Search semantics and ordering

All searches operate on the same discoverable Markdown collection from section 4.3. A filename match includes the whole relative path and therefore its folder names; it does not inspect the Markdown heading.

- `find`: literal substring matching against relative paths only. Use Unicode case folding by default, for example `golang.org/x/text/cases.Fold()`. `--case-sensitive` uses direct substring matching.
- `grep`: literal content search through ripgrep, with one result per matching line even if that line contains the query several times. Use `--no-config`, `--json`, `--fixed-strings`, and `--line-number`; add `--ignore-case` unless case-sensitive mode is requested. Pass the query with `--regexp` and file arguments after `--`. Limit searches to the discovered collection and batch file arguments to stay below platform argument limits.
- `search`: union of path and content matches, deduplicated by relative path. Preserve whether each note matched `path`, `content`, or both. Use the first content match for its line/snippet when there is one.

Sort `find` by relative path. Sort `grep` by relative path, then line number. For `search`, show path matches first in path order, followed by content-only matches in path order. `recent` uses section 4.3's ordering, applying its limit before display.

Parse ripgrep JSON rather than splitting `path:line:text`. Handle its text/bytes variants without corrupting path identity. Report execution errors separately from no matches. Do not start ripgrep with no selected files, where it might search stdin instead.

The path matcher and ripgrep both support Unicode-aware case-insensitive matching, but their Unicode folding rules can differ for some characters. Document this boundary; do not promise identical case behavior for every Unicode character. Test normal ASCII and German text explicitly.

Run the requested search once before opening fzf. Start fzf with an empty query so that content-only matches remain visible; preloading the original query would wrongly filter them by filename again. Users may then fuzzy-filter the displayed results. No live reload, regex mode, or persistent index in v1.

### 7.2 Picker data and preview

Use one common result model carrying relative path, absolute path, optional line/snippet, and match provenance. Select one result at a time. Display paths for listings, line/snippet for `grep`, and a compact match indication for `search`. Keep the preview on the right, defaulting to `right:60%:wrap`. Disable fzf reordering for `recent` with `--no-sort`.

Do not put filenames or note contents into a shell command. Use this protocol:

1. Create a private temporary JSON manifest mapping positive numeric IDs to validated result records and the canonical root. Include a manifest schema version.
2. Send fzf NUL-delimited `ID<TAB>DISPLAY` records with `--read0` and `--print0`. Escape control characters in display text. Hide the ID using field-display options, and restrict fuzzy matching to display fields.
3. Obtain the current binary's absolute path using `os.Executable`. Use a hidden `__preview` command receiving a fixed manifest path and numeric ID, for example `note __preview --manifest FIXED_PATH --id {1}`.
4. POSIX-quote the binary and manifest paths when constructing the preview command; set the fzf child's `SHELL=/bin/sh`. Only the numeric record ID is substituted. No path, title, query, or snippet is interpolated into executable shell syntax.
5. Parse the returned ID, look up the corresponding result, and revalidate its path. Never reconstruct a path from the displayed label.

Create the manifest with mode `0600`, and delete it on normal completion, failure, and cancellation. The preview helper must work without loading the user's config or rediscovering notes. Validate manifest schema, root, ID, containment, and symlinks before reading a file. Treat a disappeared note as a preview error rather than opening a different path.

Preview Markdown source with line numbers, centering on a content match when available. Bound reads and output, for example a 1 MiB read budget and a window of lines around a hit for large files. Sanitize terminal control sequences. Do not execute links, render HTML, or require an external preview tool. Optional color support may be added only after the internal text preview works.

Clear `FZF_DEFAULT_OPTS`, `FZF_DEFAULT_OPTS_FILE`, and `FZF_DEFAULT_COMMAND` in the fzf child's environment so user defaults cannot break the selection/preview protocol. Leave the parent environment unchanged. Verify the exact field and NUL options against the pinned/tested fzf version. See [fzf's manual](https://github.com/junegunn/fzf/blob/master/man/man1/fzf.1).

### 7.3 Editor and process boundary

Execute the configured editor synchronously with argv and inherited terminal streams. Append the resolved absolute note path as one argument. A generic editor receives only the path. For `hx`, a content match may use `absolute-path.md:LINE`; if a filename contains a colon, fall back to opening without a line position. Document wait flags for GUI editors, such as the config-list example for VS Code. See [Helix's usage documentation](https://docs.helix-editor.com/usage.html).

Use `os/exec` directly for editor, ripgrep, fzf, and Git. Propagate contexts and signals deliberately, preserve the original error for diagnostics, and return the public exit code. The fixed fzf preview wrapper is the only necessary shell command boundary. Test quoting there separately. See [Go's os/exec package](https://pkg.go.dev/os/exec).

Never require Go, a module cache, or the source repository at runtime. Creating with `--no-open` must not launch an editor. A failed editor leaves the created note available and reports how to open it again.

## 8. Shell completion

Use Cobra's native completion generation for Bash, Zsh, and Fish. `note completion SHELL` writes the script to stdout; it does not install it or edit shell files. Provide tested installation instructions for all three shells in `docs/completion.md`.

Dynamic completion behavior:

| Context | Candidates |
| --- | --- |
| Root invocation | Registered commands and root-relative note/directory paths |
| `new PATH` | Existing root directories, including empty ones, with trailing `/`; existing Markdown names in the current prefix may also be offered |
| `open PATH` | Directories and existing Markdown notes |
| `--template` | Effective embedded and user template identifiers |
| `--config` | Ordinary local config-file path completion |

Resolve candidates relative to the knowledge root, regardless of the current working directory. For a nested prefix, inspect the matching directory level. Preserve its existing spelling rather than normalizing directory names.

Directory completion may use `os.ReadDir` and include empty or ignored ordinary folders, while excluding hidden folders, `.git`, nested repositories, and symlinks. File suggestions follow discovery's ignore rules. If ripgrep is missing, command, directory, and template suggestions still work; omit file candidates quietly. Missing/invalid config yields no knowledge-path suggestions, without prompts or error chatter. Embedded template names remain available where possible.

Implement `ValidArgsFunction` and relevant flag completion functions. Use Cobra's no-file-completion directive for managed note paths so shells do not substitute unrelated current-directory files. Apply no-space behavior appropriately to directory continuations, and test mixed file/directory suggestions rather than assuming identical behavior across shells.

Completion is read-only: no editor, picker, content search, Git mutation, network request, or root creation. Cobra's internal completion protocol owns stdout; diagnostics must not corrupt it. Script generation itself needs neither config nor external tools. See [Cobra's completion guide](https://github.com/spf13/cobra/blob/main/site/content/completions/_index.md).

## 9. Git sync semantics

### 9.1 Scope and prerequisites

The knowledge root must be the top level of a dedicated Git working tree. Syncing a subdirectory of a larger source repository is not supported. Ordinary Git worktrees are supported, including a `.git` file pointing to their administration directory. Sparse checkouts, submodules, and nested repositories are outside v1 sync support; reject them clearly rather than staging an unexpected partial repository.

Stage all changes in this repository: Markdown, nonignored attachments, other nonignored files, tracked files even if they now match an ignore rule, and deletions. Never use a Markdown-only pathspec. Ignored untracked files remain local. User config/templates outside the root are not included.

Users configure their Git identity, remote, credentials, and upstream using Git. `note init` does not do this. A cloned knowledge repository with an upstream is the simplest setup. For a new repository, document ordinary Git initialization, a first commit, adding the chosen remote, and setting the upstream on the first push. Do not invent a remote URL or branch name.

Before mutation:

- Verify the root, Git working-tree state, supported repository shape, and current local branch.
- Resolve exactly one upstream remote and branch. Reject a missing upstream, detached HEAD, or an unborn repository with a setup hint.
- Reject an unmerged index or active merge/rebase/cherry-pick/revert with exit 5.
- Acquire an exclusive nonblocking sync lock in the working tree's actual Git administration directory. Resolve it through Git rather than assuming `root/.git` is a directory. On macOS/Linux, an advisory `flock` held by an open file descriptor is suitable; a second invocation fails clearly, and process exit releases the lock. Keep the lock file's inode stable rather than unlinking it on release.
- Check that an effective Git identity is available if a local commit will be necessary. A completely clean update must not require a local author identity just to receive commits.

Resolve the root/upstream again if needed after obtaining the lock. The lock coordinates `note sync` invocations; it does not control editors or unrelated Git processes. Ask users to save buffers and avoid concurrent Git operations during sync. Check for newly dirty state after hooks and before integration, and stop instead of absorbing additional edits silently.

### 9.2 Workflow

The user-facing order is **add → commit if needed → pull with rebase → push**:

1. **Stage:** run `git add --all -- .` at the repository root.
2. **Commit if needed:** inspect the staged diff, distinguishing no change from errors. If nonempty, commit using the configured automatic message. Do not create an empty commit. Respect normal hooks, signing, and Git configuration; do not bypass them.
3. **Pull with rebase:** fetch the exact upstream and rebase local commits onto the fetched commit, with automatic stashing explicitly disabled. Integrate only after the collision checks below.
4. **Push:** push the current branch to its explicit upstream remote branch. Do this even if no automatic commit was created, because the repository may already contain unpublished commits. Do not depend on `push.default`.

The default message is `notes: sync {timestamp}`, where `{timestamp}` is a UTC RFC 3339 timestamp. Expand this one config placeholder only. `--message TEXT` is a literal override, not a template; reject an empty override. Capture the clock through a small test seam.

If a commit hook fails, do not pull or push. Keep the index and working files available for inspection. If a hook succeeds but leaves additional changes, stop before pull and explain that the user should review them and retry. Report completed phases and the local commit ID so an interrupted operation remains understandable.

### 9.3 Safe pull implementation

Implement the logical pull as a controlled **fetch + rebase**, rather than an opaque `git pull` invocation. This remains the specified pull-with-rebase behavior and permits checking local ignored files before integration.

- Capture the previous upstream tip, fetch exactly the configured upstream branch once, and record the resulting commit OID. Do not integrate a moving ref or fetch again after checking it.
- Compare untracked local paths, including ignored files, with the tracked tree of that fetched commit. Refuse any exact or file/directory ancestor collision before rebase. A remote change that begins tracking `attachments/local.png` must not overwrite a different ignored local copy.
- Make collision comparisons conservative for case-insensitive and Unicode-normalizing filesystems. Case-only or canonically equivalent spellings must not bypass the guard, even when tests run on Linux. This comparison does not rename files.
- Use NUL-delimited Git output for these path checks, for example `git ls-files` and `git ls-tree`; do not parse quoted human output. Include both ignored and ordinary untracked paths.
- If the remote upstream history was rewritten rather than advanced normally, stop with a manual-review message. v1 does not attempt to interpret force-pushed history automatically.
- Rebase onto the captured OID with `--no-autostash`. Never create, apply, or drop a stash. Existing personal stash entries are untouched.

A collision refusal is exit 3: the local files and any completed local commit remain, and integration/push do not run. Explain which paths require the user's review. Do not move, delete, or overwrite an ignored file to make the pull succeed. If a directory contains only empty folders, no file data is at risk; do not treat that as an attachment to preserve.

Use the exact upstream remote and full remote branch ref for push, conceptually `git push REMOTE HEAD:refs/heads/UPSTREAM_BRANCH`. Treat remote names/refs as validated data and avoid shell construction. Never force-push. See [Git fetch](https://git-scm.com/docs/git-fetch), [Git rebase](https://git-scm.com/docs/git-rebase), [Git ls-files](https://git-scm.com/docs/git-ls-files), and [Git ls-tree](https://git-scm.com/docs/git-ls-tree).

### 9.4 Failures and recovery

| Failure | Required behavior |
| --- | --- |
| Preflight fails | No staging, commit, integration, or push |
| Add or commit fails | Preserve working files/index; no pull or push |
| Fetch fails | Keep the completed local commit; no rebase or push |
| Untracked/ignored path collision | Keep both local state and fetched Git objects; stop before integration |
| Rebase conflicts | Exit 5; retain Git's rebase state; do not push or commit a resolution automatically |
| Push rejected or unavailable | Keep local commits; report that retry may be needed |
| Ctrl-C | Return 130; release the sync lock; preserve completed commits and any Git operation state |

After a rebase conflict, users inspect `git status`, resolve files, stage the resolutions, and use `git rebase --continue`, or use `git rebase --abort`. Aborting returns to the state before rebase, including the local automatic commit created before pull. A subsequent `note sync` refuses to run while unresolved Git state remains.

After a push race, fetch failure, or temporary authentication/network failure, retrying `note sync` should be safe. It skips a new automatic commit when there are no new changes, then integrates and pushes the existing commits. Explain authentication errors without adding another credential mechanism.

No automatic conflict resolution, `ours`/`theirs` selection, reset, clean, force push, stash manipulation, or custom recovery journal. Never report sync success unless push succeeded. Preserve the original Git diagnostic and give the user the completed phase and next action. Crash recovery uses Git's commits and operation state, not a second state machine maintained by `note`.

### 9.5 Server use

`note sync` does not depend on editor, fzf, ripgrep, or a TTY. A clean server receives remote updates without creating an empty commit. Interactive Git authentication may still be unsuitable for unattended use; document SSH keys or other ordinary Git credential setup rather than implementing credentials inside `note`.

Sync includes saved filesystem changes only. It cannot read unsaved editor buffers. Schedule or deploy it through normal shell tooling if desired; scheduling is not a v1 feature.

## 10. Architecture and repository structure

Keep the architecture small and easy to navigate. The responsibilities below are mandatory; the precise package/file split is advisory. An agent may combine closely related modules or split a growing module when that improves clarity. Do not create an interface or package merely to match a diagram.

| Responsibility | Contract |
| --- | --- |
| CLI | Cobra routing, flags, argument validation, terminal/output policy, and public exit codes |
| Configuration | XDG resolution, TOML decoding, defaults, validation, and executable specifications |
| Paths and notes | Safe path resolution, basename normalization, exclusive creation, and existing-file handling |
| Templates | Embedded/user resolution and fixed-placeholder rendering |
| Discovery and search | Shared visible collection, literal matching, content results, ordering, and deduplication |
| Picker and preview | fzf record IDs, temporary manifest, safe preview helper, selection, and cleanup |
| Editor/processes | Direct argv execution, streams, context/signal behavior, and editor line positioning |
| Sync | Dedicated-repository preflight, lock, commit-first integration, push, and failure reporting |

Suggested layout:

```text
note/
├── go.mod
├── go.sum
├── cmd/note/main.go
├── internal/
│   ├── cli/
│   ├── config/
│   ├── notes/             # Paths and creation; may also own templates.
│   ├── templates/defaults/
│   ├── search/            # May include discovery.
│   ├── picker/            # May include preview.
│   └── syncgit/
├── tests/integration/     # Only where cross-component tests need it.
├── testdata/
├── docs/
├── examples/config.toml
├── AGENTS.md
├── CONTRIBUTING.md
├── README.md
├── PLAN.md
├── CHANGELOG.md
└── LICENSE
```

Place focused `_test.go` files beside their code. Keep `main` small: construct dependencies, execute, clean up, and choose the exit code. No filesystem work or process execution in package initialization. Use explicit structs for configuration, command specifications, search results, and operation errors. JSON tags are part of the output contract; avoid accidentally serializing Go field names or nil arrays.

Inject only boundaries tests need, such as a process runner, clock, home-directory resolver, and terminal availability. Prefer ordinary functions and small structs over a dependency-injection container, generalized workflow engine, or mock-only architecture. Keep root-relative and absolute paths distinguishable in the data model.

`AGENTS.md` must summarize the public invariants, build/test commands, and documentation responsibilities. It should permit routine technical choices without reopening product questions. Tests must use temporary knowledge roots and Git repositories; never run mutating tests against personal data. Do not publish a release or modify unrelated repositories as an implementation side effect.

Build a usable core first. Completion and docs for the core should work before sync complexity and release automation are added. Architectural decisions and accepted deviations belong in `docs/architecture.md`; do not make conversation history a dependency.

## 11. Go dependencies, build, installation, and releases

### 11.1 Module setup

Use Go modules with concrete versions in `go.mod` and checksums in `go.sum`. Choose compatible dependency releases when bootstrapping; do not leave `@latest` as an ongoing development instruction. `go.sum` verifies module contents; it is not a separate dependency lockfile.

Expected direct dependencies:

- `github.com/spf13/cobra` for the CLI and shell completion.
- `github.com/pelletier/go-toml/v2` for TOML.
- `golang.org/x/text` for normalization and case folding.
- `golang.org/x/term` if needed for terminal detection.
- `golang.org/x/sys/unix` if needed for the sync lock.

Do not add Viper, a full TUI framework, a Markdown parser, or a Git library without a concrete v1 need. Use the installed Git CLI so normal credentials, configuration, hooks, and user recovery tools keep working.

**Repository/module address:** `github.com/mariusrueve/note-cli`, verified from this repository's configured origin. The actual module/imports use this address. Versioned remote installation instructions wait for a published version.

The complete MIT license in `LICENSE` uses the existing repository metadata: Copyright (c) 2026 Marius Rüve.

### 11.2 Local development and updates

The initial workflow must work from a source checkout using Go and the relevant external tools. Keep build intermediates under `work/` and ignore them in Git.

```bash
# Download and verify the dependency versions selected in go.mod.
go mod download
go mod verify

# Run directly during development.
go run -mod=readonly ./cmd/note --help

# Format, check, and test.
go fmt ./...
go vet -mod=readonly ./...
go test -mod=readonly ./...
go test -mod=readonly -race ./...

# Build a local binary, or install it into the Go binary directory.
mkdir -p work/bin
go build -mod=readonly -trimpath -o work/bin/note ./cmd/note
go install -mod=readonly ./cmd/note
```

Document `GOBIN`, the default Go binary directory, and adding that directory to `PATH`. A local update is: update or edit source, run the relevant checks, then rebuild/reinstall. Editing source does not change an already installed executable. A fork follows the same workflow after updating its module path where appropriate.

Add version and commit variables to the main package, defaulting to `dev` and `unknown`, and pass them into the CLI. `note --version` must work without config. A release build can set them through linker flags:

```bash
CGO_ENABLED=0 go build -mod=readonly -trimpath \
  -ldflags "-s -w -X main.version=0.1.0 -X main.commit=unknown" \
  -o work/bin/note ./cmd/note
```

Release automation replaces `unknown` with the actual commit. Keep the runtime dependencies pure Go so released binaries build with `CGO_ENABLED=0`. Race tests have separate native toolchain requirements.

See [Go module management](https://go.dev/doc/modules/managing-dependencies), [Go installation](https://go.dev/doc/install), [the Go command](https://pkg.go.dev/cmd/go), and [Go's release history and support policy](https://go.dev/doc/devel/release).

### 11.3 Binary delivery after the working core

Support these four release targets: `darwin/arm64`, `darwin/amd64`, `linux/arm64`, and `linux/amd64`. Cross-compilation proves that a target builds; native smoke tests establish that it runs correctly. Record any native platform not covered by regular CI and test it before declaring its release verified.

An installed binary needs no Go runtime, source tree, or module cache. Runtime requirements are command-specific: editor for opening, ripgrep for discovery/content search, fzf for interactive selection, and Git for sync. Embed all default templates and the preview implementation.

Deliver `.tar.gz` archives containing the correctly named binary, README, and MIT license, plus SHA-256 checksums. Document download, checksum verification, copying the binary into `PATH`, replacement for updates, and removal for uninstall. Uninstalling must leave config and knowledge data intact. Remote `go install MODULE/cmd/note@VERSION` instructions wait until the actual module address exists.

GoReleaser v2 is a suitable optional release tool once core behavior and sync pass. Its configuration must build all four targets with `CGO_ENABLED=0`, version metadata, archives, and checksums. A local snapshot must not publish anything. Release publishing is a separate explicit action, not part of ordinary development or tests. See [GoReleaser's documentation](https://goreleaser.com/intro/).

Keep source, tests, templates, example config, this plan, and docs together in the repository. Homebrew packaging, Linux distribution packages, and automatic self-update are later additions, not prerequisites for a usable v1.

## 12. Tests and verification

Use Go's `testing` package and verify public behavior and actual integration risks. Avoid tests that merely repeat private implementation lines or confirm large mocks. Use `t.TempDir()` for notes and repositories, and `t.Setenv()` for isolated XDG/Git variables. Tests changing process environment must not run in parallel. Inject a home-directory resolver for `~` tests rather than repurposing the user's `HOME`.

Create a fresh Cobra command tree per CLI test, set buffered streams, and call `ExecuteContext`. For signal and installed-binary tests, run an actually built `note` executable. Use small process/clock/terminal seams only where necessary. See [Go testing](https://pkg.go.dev/testing).

### 12.1 Unit and CLI coverage

- **Configuration:** defaults, editor argv form, overrides, relative TOML paths, absent/invalid XDG values, wrong types, unknown keys, schema version, and explicit `false`. Help/version/script generation work without config or tools.
- **Paths:** slug examples, idempotence, traversal/absolute-path rejection, symlink escape, Unicode, spaces, quotes, colons, normalization collisions, and legacy case collisions.
- **Creation:** parent directories, pre-creation template validation, no frontmatter, exclusive creation and races, existing-file preservation, Enter/EOF/cancellation, headless existing files, `--no-open`, and `--open-existing`.
- **Templates:** all embedded resources, user overrides, unknown/malformed placeholders, escaped literal openers, unchanged `$PATH` and `$E = mc^2$`, derived titles, explicit Unicode titles, and zero-byte empty output.
- **Routing:** no arguments, query lookup, exact and ambiguous matches, creation offer and refusal, reserved command names, global flags, unknown options, and too many arguments. Help must not start a picker.
- **Outputs:** exact JSON schema, empty arrays, search deduplication/provenance, line indexes, deterministic sorting, exit codes, and separated stdout/stderr.
- **Process boundary:** filenames and queries containing quotes, `$()`, backticks, and leading hyphens remain data. No secondary command runs.
- **Completion:** root-relative prefixes and trailing slashes, empty directories, existing notes, templates, invalid/missing config, quiet fallbacks, directives, and absence of mutation. Script generation needs no dependencies.

### 12.2 Integration coverage

- **ripgrep/search:** fixtures where a term occurs only in a path, only in content, in both, and in neither. Prove the three distinct result sets. Verify literal regex punctuation, case handling, ignore rules, hidden files, and content line numbers.
- **Discovery:** `.gitignore`, `.ignore`, `.rgignore`, symlinks, submodules/nested repositories, and explicit opening of an ignored note.
- **Picker/preview:** ID round-trip with NUL framing, unusual filenames, manifest validation, missing/large files, later-line content hits, cancellation, and cleanup. A deterministic fzf filter can test selection; verify the complete terminal UI separately.
- **Editor:** a Go test-helper executable records argv and exit behavior. Verify generic opening, hx line positions, configured arguments, errors, and preservation of newly created files. Do not introduce another language runtime for test helpers.
- **Recent:** controlled nanosecond modification times, tie ordering, limit, and preserved picker order.
- **Installed binary:** copy the built binary outside the checkout and run help, version, completion, template listing, and `new --no-open`. Exercise its preview helper with installation paths containing spaces and apostrophes. Embedded resources must work without the repository or module cache at runtime.
- **Release stage:** verify all four targets, version stamps, archives, included README/license, and checksums. Use native tests for runtime support rather than treating cross-build success as execution proof.

### 12.3 Git matrix

Use a local bare remote and two temporary clones, the real Git CLI, isolated system/user Git configuration, and an explicit test identity. No internet or personal credentials. Assertions must inspect commits, file contents, index, branch state, remote state, and stash entries, not just command invocation strings.

| Scenario | Required evidence |
| --- | --- |
| Both clones unchanged | Successful sync; no empty commit |
| Remote-only updates | Clean second clone receives them without a new local commit or author identity requirement |
| Existing unpublished local commits | They are pushed even without new edits |
| Staged/unstaged edits, new files, attachment, deletion | One automatic commit contains all nonignored changes before integration |
| Independent local and remote edits | Rebase succeeds and both changes reach the remote |
| Conflicting local and remote edits | Local changes were committed first; rebase stops with 5; no push; recovery preserves those commits |
| Same new path created locally and remotely | Git reports a conflict without silently losing local contents |
| Ignored local file newly tracked remotely | Collision guard stops before rebase; original local bytes remain |
| Local file versus remote directory, and reverse | Ancestor collision guard protects untracked/ignored files |
| Case-only or canonically equivalent local/remote names | Collision guard protects data on case-insensitive/normalizing filesystems |
| Existing personal stashes | Entries and contents remain unchanged throughout success and failure |
| Fetch failure after local commit | Commit remains; no integration/push; later retry succeeds |
| Commit hook failure | No fetch/rebase/push; working files and index remain inspectable |
| Missing identity when a commit is needed | Failure before staging/mutation; clean receiving clone is still supported |
| Hook leaves extra changes | Stop before integration rather than silently committing those changes |
| Push hook rejection or remote race | Local commits remain; retry can integrate and push |
| Upstream rewritten | Stop for review; no automatic interpretation or force push |
| Missing upstream, subdirectory root, detached/unborn HEAD | Stop before mutation with setup guidance |
| Sparse checkout, submodule, nested repository | Clear unsupported-shape error before staging |
| Unmerged index or active Git operation | Exit 5; no automatic resolution or new sync commit |
| Concurrent syncs | Exactly one obtains the lock; the second fails clearly |
| Interrupt after commit or during rebase | Local commit/Git state survives; lock releases; documented recovery works |
| Clean headless server without editor/fzf/ripgrep | Update and push checks work without interactive note tools |

Inject failures with local hooks, invalid local remotes, and controlled child-process interruptions. Exercise both `git rebase --continue` and `git rebase --abort` after a simulated conflict, then retry `note sync`. Recovery is verified only when it has actually been performed.

### 12.4 CI and manual checks

Run CI on macOS and Linux with supported Go 1.26 and 1.27 patch releases. Verify dependency compatibility and available versions at bootstrap. Cover Apple Silicon and x86-64 natively, and cross-build all four target binaries. Document native Intel-macOS and ARM64-Linux release checks if those runners are not available in every CI run.

Required checks are module download/verification, formatting, `go vet`, `go test`, relevant race tests, and binary builds. Use `-mod=readonly` for tests/builds. A format check must fail when `gofmt -l` reports files; merely printing its output is not a failing check. After imports are implemented, `go mod tidy` must not leave an unexplained dependency diff.

Run race tests on supported native runners with CGO enabled and a suitable C compiler. `CGO_ENABLED=0` applies to shipped binaries, not the race detector. Avoid artificial concurrency in the application simply to justify race tests.

Install required integration tools explicitly in CI. Do not silently skip release-critical Git/ripgrep tests. Ordinary checks must not publish releases or touch real knowledge roots/remotes.

Manual smoke checks on macOS and Linux:

- Load generated completion scripts in Bash, Zsh, and Fish and actually complete a root directory.
- Open `note`, confirm the right-side preview, and use Enter to open hx.
- Create a note, then repeat `new` and verify the existing-file notice and Enter confirmation.
- Cancel the picker/prompt and verify unchanged files.
- Run the documented local installation/update flow from a fresh checkout.

At the release stage, inspect a local release snapshot and verify archive contents/checksums. Record remaining limitations honestly; building a plan is not evidence that these implementation checks passed.

## 13. Implementation milestones

Each milestone delivers usable code, appropriate tests, and updated docs. Agents may decide routine internal details while preserving these contracts. A product change requires an explicit proposed alternative; do not quietly expand scope.

### Phase 0 — Foundation and routing proof

- [x] Create the repository, `cmd/note`, module files, pinned dependencies, and local build workflow.
- [x] Add MIT `LICENSE`, initial README, `AGENTS.md`, and a brief architecture map.
- [x] Prove Cobra routing for `note`, `note QUERY`, subcommands, global flags, and completion.
- [x] Establish error/exit-code handling and the process boundary.

Acceptance: help and version run without config; a local build produces a real executable named `note`. The repository address may remain an explicitly documented local placeholder until release setup.

### Phase 1 — Config, templates, creation, and opening

- [x] Implement XDG/TOML, `init`, schema validation, and editor configuration.
- [x] Implement safe paths, ASCII slugging, embedded templates, and user overrides.
- [x] Implement `new`, `open PATH`, existing-file confirmation, and headless options.
- [x] Complete file-preservation, placeholder, and editor-argv tests.

Acceptance: a new note is created without metadata and opened; existing files remain unchanged and open after Enter. Creation without opening works without external tools.

### Phase 2 — Discovery, picker, and preview

- [x] Implement the shared collection and ignore rules.
- [x] Connect `note` and pathless `open` to fzf.
- [x] Implement numeric record IDs, temporary manifests, the installed-binary preview helper, and cleanup.
- [x] Verify cancellation, empty collections, unusual names, and right-side preview.

Acceptance: select and open notes reliably, with filenames and contents always treated as data.

### Phase 3 — Search, recent, and convenience lookup

- [x] Implement the distinct `find`, `grep`, and `search` scopes.
- [x] Implement print/JSON modes, case handling, deterministic order, and headless output.
- [x] Add `recent` and the full convenience lookup, including the confirmed creation offer.
- [x] Verify real ripgrep results, content-only visibility in fzf, and deduplication.

Acceptance: the search fixture proves the distinct result sets; convenience lookup creates only with explicit consent.

### Phase 4 — Completion and core usability

- [x] Implement root-relative path/template completion and script generation for all three shells.
- [x] Document installation and perform completion smoke checks.
- [x] Add `doctor` for implemented configuration/tools and describe the later Git requirements.
- [x] Finish core quickstart and local build/update documentation.

**Core checkpoint:** creation, opening, navigation, search, recent, preview, and completion are usable from an installed binary. Reach this checkpoint before adding Git sync and release automation.

### Phase 5 — Commit-first Git sync

- [x] Implement dedicated-repository preflight, upstream resolution, and the sync lock.
- [x] Implement add, optional commit, guarded fetch/rebase, and explicit upstream push.
- [x] Complete the Git matrix and real manual-recovery exercises.
- [x] Finish sync/server documentation and extend `doctor` with Git readiness checks.

Acceptance: two clones sync safely; local changes are committed before integration; conflicts stop without push; personal stashes and ignored local files remain intact.

### Phase 6 — Delivery and documentation

- [x] Complete the documents in section 14 and verify a fresh user's local installation/update path.
- [ ] Activate macOS/Linux CI and finish installed-binary, native smoke, and cross-build checks.
- [x] Verify portable archives, version metadata, all four targets, and checksums; add release automation only now.
- [x] Record manual UI/completion checks and assess the v1 checklist.
- [x] Before any publication, replace the module placeholder, fill copyright metadata, and verify release instructions.

Acceptance: another user can install, configure, use, modify, and update the tool from the repository documentation. Release publication itself is not required to finish local implementation.

## 14. Documentation and definition of done

Write the plan and repository documentation in English. Prompts, help, diagnostics, config comments, and bundled template headings should also use English in v1. User-authored note titles and text may use any language.

| Document | Required content |
| --- | --- |
| `README.md` | Purpose, limits, prerequisites, local install, quickstart, main commands, update |
| `docs/cli.md` | Commands/options, distinct search scopes, shortcut, terminal behavior, output schema, exit codes |
| `docs/configuration.md` | XDG paths on both systems, defaults, schema, precedence, editor argv |
| `docs/templates.md` | Embedded/user templates, overrides, fixed placeholders/escaping, no automatic metadata |
| `docs/completion.md` | Tested Bash/Zsh/Fish setup, root-relative directory examples, troubleshooting |
| `docs/sync.md` | Dedicated-repository setup, full-repository scope, commit-first workflow, collision guard, server use, recovery |
| `docs/architecture.md` | Actual responsibility boundaries, data flow, intentional simplifications, adding general features |
| `docs/troubleshooting.md` | Config/tools/PATH, editor failure, ignore rules, empty results, Git failure and next actions |
| `docs/releasing.md` | Added at release stage: targets, toolchain, local snapshots, archives, metadata, checksums, publication |
| `CONTRIBUTING.md` | Reproducible Go development, tests, rebuild/reinstall, forks, dependency updates |
| `AGENTS.md` | Short agent work rules, public invariants, safe test data, concrete verification commands |
| `examples/config.toml` | Valid commented schema example, without personal paths |
| `CHANGELOG.md` | Public behavior/config changes by version |
| `LICENSE` | Complete MIT license with actual copyright metadata |

Completion criteria:

- [ ] Every requirement in section 1 is implemented, documented, and appropriately verified.
- [x] No AI dependencies, metadata injection, or specialized project/meeting/journal commands are present.
- [x] One-root configuration, templates, and knowledge data are distinct and portable.
- [x] Existing notes are never overwritten by creation, and slug/path safety tests pass.
- [x] The three search scopes, no-argument picker, and convenience lookup behave as specified.
- [ ] Root-relative completion and right-side preview have been checked on both platforms.
- [x] The complete Git matrix and continue/abort recovery exercises pass.
- [x] Installed binaries work outside the checkout, with embedded templates and preview and no language runtime requirement.
- [x] All four release targets build; archive contents, version metadata, and checksums are verified.
- [x] Local installation, update, and customization are fully documented and reproducible.
- [x] Required checks pass; remaining limits are documented and unfinished work remains unchecked.

## 15. Later extensions

These ideas are a roadmap, not additional v1 tasks:

| Extension | Possible scope |
| --- | --- |
| Backlinks | Read-only `note backlinks PATH`, computed from ordinary relative Markdown links at invocation |
| Dead-link checker | Local Markdown targets and anchors; external URL checks only with explicit opt-in |
| Markdown lint/format | General commands integrating existing tools; formatting mutates only when requested |
| Rename with link updates | Separate specification for relative/reference links, anchors, and safe multi-file changes |
| Git convenience | Read-only status and clearer conflict diagnostics after v1 sync is proven |
| Editor adapters | More reliable line jumps for additional editors without changing search semantics |
| Packaging | Homebrew/distribution packages and easier binary updates |
| `note ask` | Separate decision about meaning, provider, data access, and any AI use; absent from v1 |

Backlinks mean that if `a.md` links to `b.md`, the tool can list `a.md` as an incoming link to `b.md`. This does not require adding metadata to either file. v1 needs neither `[[Wiki Links]]` nor a link index. Links such as `[ECFP](../cheminformatics/ecfp.md)` remain ordinary Markdown.

Extend the existing responsibilities and document changes to CLI/config contracts. Multiple roots, specialized commands, or AI would change the current product boundaries and require a separate decision. Users can make those choices in a fork; v1 does not need a framework to anticipate them.
