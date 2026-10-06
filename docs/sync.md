# Git sync

The knowledge root must be the top level of a dedicated Git working tree. Sync
includes all nonignored files: notes, attachments, staged/unstaged changes, new
files, and deletions. Worktrees with a `.git` administration file are supported.
Subdirectory roots, sparse checkouts, submodules, and nested repositories are
rejected before staging. Discovery's Markdown collection does not restrict sync.

Initialize and configure Git yourself; `note init` does not do this:

```sh
cd ~/Knowledge
git init -b main
git config user.name "Your Name"
git config user.email "you@example.org"
git add --all
git commit -m "Initial notes"
git remote add origin YOUR_REMOTE_URL
git push --set-upstream origin main
note sync
```

Save editor buffers and avoid concurrent Git/editor changes during sync. Unsaved
buffers are invisible to the command. The advisory lock coordinates `note sync`
processes only. It lives in the actual worktree administration directory; its
inode stays stable and closing the descriptor/process exit releases the lock.

The workflow is:

1. Validate the dedicated repository, branch, exactly one upstream, repository
   shape, active Git operations/unmerged index, and identity if a commit is needed.
2. Lock, recheck, then `git add --all -- .`.
3. Commit staged changes if nonempty, respecting hooks/signing/config. Default
   message is `notes: sync {timestamp}` in UTC RFC 3339. `--message` is literal.
4. Capture the previous upstream tip and fetch exactly the upstream branch once
   into FETCH_HEAD, without changing its tracking ref yet.
5. Refuse rewritten upstream history and exact/ancestor collisions between local
   untracked/ignored files and fetched tracked paths. Comparisons conservatively
   fold case and normalize Unicode, even on Linux. Keep all original bytes.
6. Rebase onto the captured OID with `--no-autostash`, update the tracking ref after
   checks/integration, then push `HEAD:refs/heads/UPSTREAM_BRANCH` to the explicit
   remote. Push also occurs when there were existing unpublished commits and no
   new automatic commit. Success requires a successful push.

Personal stash entries are never created/applied/dropped. There is no automatic
resolution, reset, clean, force push, or recovery journal. After hooks and before
integration/push, newly dirty state stops the operation for review. Hook failures
preserve working files/index; fetch/push failures preserve completed commits.
Diagnostics report completed phases and any newly created commit ID.

A collision refusal (exit 3) leaves local ignored files and fetched objects intact.
Review both versions manually before moving/reconciling your local file. Empty
directories have no file contents to guard. An upstream rewrite keeps the old
tracking tip, so retries continue refusing it. Inspect FETCH_HEAD and the previous
upstream; an intentional manual fetch/update acknowledges the history change.

For conflicts (exit 5):

```sh
cd ~/Knowledge
git status
# Resolve conflicted files, then:
git add -- path/to/resolved.md
git rebase --continue
note sync
# Or abandon integration while retaining the local automatic commit:
git rebase --abort
```

Do not rerun sync during an unresolved rebase/merge/cherry-pick/revert. Aborting
returns to the pre-rebase state including the automatic commit. After a temporary
fetch/authentication/push failure, retrying sync reuses existing commits, then
fetches/rebases/pushes; it does not make another commit without new changes.
Ctrl-C returns 130, releases the lock, and preserves completed commits and Git's
remaining operation state. Inspect `git status` before retrying after interruption.

A clean headless server needs Git, config, and the root, without hx/fzf/rg or a TTY.
It receives commits without creating an empty commit or needing a local author
identity. Configure ordinary SSH keys/Git credentials for unattended use. The CLI
does not provide credentials or a scheduler.
