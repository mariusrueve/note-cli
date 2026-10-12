# Troubleshooting

Run `note doctor` to inspect effective config, root, tools, inbox and literature
directories, bibliography, Git readiness, and completion guidance. It never
contacts a remote or starts an editor. A configured but unreadable bibliography
fails doctor like a missing tool.

- Missing config: run `note init`, or pass `--config` before the command. Relative
  TOML paths are relative to that file. Unknown keys and unsupported schema versions
  are errors. Only init creates missing roots.
- Unknown key that the docs describe (for example `bibliography`): an older `note`
  is probably first on PATH. Check `which -a note` and `note --version`. Release
  installs live in `~/.local/bin`, source installs in `GOBIN` or `~/go/bin`; keep
  only one, or order PATH deliberately. Run `note self-update` for release installs.
- Missing tools: install the executable configured in its argv list and ensure
  PATH includes it. Opening needs the editor, discovery/search needs rg, picking
  needs fzf, sync needs Git. `new --no-open` needs no external executable.
- Editor failure: the newly written note remains. Fix editor/PATH/wait flags and
  use `note open PATH`. Use a TOML list for GUI editor arguments.
- No results: find searches paths, grep searches contents, search combines them.
  Hidden/ignored files and symlinks are omitted. Inspect `.gitignore`, `.ignore`,
  `.rgignore`; use `rg --no-config --files --null` at the root. Explicit `open`
  can open an ignored Markdown file without changing discovery rules.
- Existing destination: creation preserves contents and returns 4 headlessly;
  use `note open PATH` or `new --open-existing`. Case-equivalent legacy names use
  their actual spelling after confirmation; ambiguous collisions need review.
- Non-ASCII-only slug: provide an ASCII basename and preserve the heading with
  `--title`. Unicode content and existing filenames remain usable.
- Picker canceled: code 130; no selection is opened. Parent fzf defaults are
  ignored in the child to keep the ID/preview protocol reliable. Preview shows
  escaped source, reads at most 1 MiB, and reports missing/invalid text.
- Bibliography: `note cite` needs `bibliography` in the config (exit 3 when unset).
  Read and parse errors name the file and line; Better BibTeX auto-export keeps
  the file current. Keys are exact and case-sensitive, so an unknown key exits 1;
  use Tab completion or `note cite` without a key to pick.
- Broken links after moving or renaming notes by hand: run `note deadlinks` and
  fix the reported `path:line` targets. Prefer `note move OLD NEW --dry-run`,
  then `note move OLD NEW`, which updates links for you. Ignored notes and
  case-only renames are outside its scope. `case differs` works on macOS but fails on
  case-sensitive Linux servers.
- Sync preflight failure: use a dedicated top-level checkout, committed local
  branch, and exactly one upstream. Configure identity before making local
  commits. Sparse/submodule/nested shapes need a different Git workflow.
- Git conflict: inspect `git status`, resolve/stage and `git rebase --continue`,
  or `git rebase --abort`. Sync refuses active operations. See [sync](sync.md).
- Fetch/push/authentication error: completed local commits remain. Fix ordinary
  Git transport/credentials and retry. Never use force push to fix a routine race.
- Local ignored-file collision or rewritten upstream: inspect the reported paths
  or FETCH_HEAD yourself; the CLI stops before integration and preserves data.
- Busy sync lock: wait for the other sync. Process exit releases the advisory
  lock; do not delete the stable lock file while another process may hold it.

For machine output use `--json`, not human path/line splitting. Diagnostics are
on stderr. For search and listing commands an empty JSON result is `[]` and
exit 1. For `deadlinks`, `[]` means no broken links and exits 0; broken links exit 1.
