# Troubleshooting

Run `note doctor` to inspect effective config, root, tools, Git readiness, and
completion guidance. It never contacts a remote or starts an editor.

- Missing config: run `note init`, or pass `--config` before the command. Relative
  TOML paths are relative to that file. Unknown keys and unsupported schema versions
  are errors. Only init creates missing roots.
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
on stderr. An empty JSON result is `[]` and exit 1.
