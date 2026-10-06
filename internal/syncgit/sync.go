// Package syncgit records local work before integration and leaves recovery to Git.
package syncgit

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mariusrueve/note-cli/internal/config"
	"github.com/mariusrueve/note-cli/internal/errs"
	"github.com/mariusrueve/note-cli/internal/process"
	"github.com/mariusrueve/note-cli/internal/search"
	"golang.org/x/sys/unix"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type Options struct {
	In      io.Reader
	Err     io.Writer
	Now     func() time.Time
	Message *string
}
type repo struct {
	c   config.Config
	o   Options
	ctx context.Context
}
type State struct{ Branch, Remote, Ref, Upstream, Admin string }

func (r repo) capture(args ...string) ([]byte, error) {
	cmd := process.Background(r.ctx, r.c.Tools.Git, args...)
	cmd.Dir = r.c.Root
	cmd.Stdin = r.o.In
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	b, e := cmd.Output()
	if r.ctx.Err() != nil {
		return b, r.ctx.Err()
	}
	if e != nil {
		return b, fmt.Errorf("git %s: %w: %s", args[0], e, strings.TrimSpace(diagnostic.String()))
	}
	return b, nil
}
func (r repo) text(args ...string) (string, error) {
	b, e := r.capture(args...)
	return strings.TrimSuffix(string(b), "\n"), e
}
func (r repo) run(args ...string) error {
	cmd := process.Background(r.ctx, r.c.Tools.Git, args...)
	cmd.Dir = r.c.Root
	cmd.Stdin = r.o.In
	cmd.Stdout = r.o.Err
	cmd.Stderr = r.o.Err
	e := cmd.Run()
	if r.ctx.Err() != nil {
		return r.ctx.Err()
	}
	if e != nil {
		return fmt.Errorf("git %s: %w", args[0], e)
	}
	return nil
}
func (r repo) predicate(args ...string) (bool, error) {
	cmd := process.Background(r.ctx, r.c.Tools.Git, args...)
	cmd.Dir = r.c.Root
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	e := cmd.Run()
	if r.ctx.Err() != nil {
		return false, r.ctx.Err()
	}
	if e == nil {
		return true, nil
	}
	if process.ExitCode(e) == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git %s: %w: %s", args[0], e, diagnostic.String())
}
func (r repo) active() error {
	for _, name := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply", "sequencer"} {
		p, e := r.text("rev-parse", "--git-path", name)
		if e != nil {
			return e
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(r.c.Root, p)
		}
		if _, e = os.Lstat(p); e == nil {
			return errs.New(5, "Git operation in progress (%s); inspect git status and continue or abort it first", name)
		} else if !os.IsNotExist(e) {
			return e
		}
	}
	b, e := r.capture("ls-files", "--unmerged", "-z")
	if e != nil {
		return e
	}
	if len(b) > 0 {
		return errs.New(5, "unmerged index; resolve conflicts with Git before note sync")
	}
	return nil
}
func (r repo) preflight() (State, error) {
	var s State
	top, e := r.text("rev-parse", "--show-toplevel")
	if e != nil {
		return s, fmt.Errorf("sync needs a dedicated Git working tree: %w", e)
	}
	top, e = filepath.EvalSymlinks(top)
	if e != nil {
		return s, e
	}
	if top != r.c.Root {
		return s, fmt.Errorf("knowledge root must be the Git working-tree top level, not a subdirectory")
	}
	if e = r.active(); e != nil {
		return s, e
	}
	if parent, e := r.text("rev-parse", "--show-superproject-working-tree"); e != nil {
		return s, e
	} else if parent != "" {
		return s, fmt.Errorf("a submodule root is unsupported for note sync")
	}
	s.Branch, e = r.text("symbolic-ref", "--quiet", "--short", "HEAD")
	if e != nil {
		return s, fmt.Errorf("sync requires a local branch, not detached HEAD; switch to a branch")
	}
	if _, e = r.text("rev-parse", "--verify", "HEAD^{commit}"); e != nil {
		return s, fmt.Errorf("unborn repository; create an initial commit and configure its upstream")
	}
	for _, key := range []string{"core.sparseCheckout", "core.sparseCheckoutCone"} {
		v, e := r.text("config", "--bool", "--get", key)
		if e != nil && process.ExitCode(e) != 1 {
			return s, e
		}
		if v == "true" {
			return s, fmt.Errorf("sparse checkout is unsupported; use a complete dedicated checkout")
		}
	}
	stages, e := r.capture("ls-files", "--stage", "-z")
	if e != nil {
		return s, e
	}
	for _, entry := range bytes.Split(stages, []byte{0}) {
		if bytes.HasPrefix(entry, []byte("160000 ")) {
			return s, fmt.Errorf("submodules are unsupported for note sync")
		}
	}
	e = filepath.WalkDir(r.c.Root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if p == r.c.Root {
			return nil
		}
		if d.Name() == ".git" {
			if filepath.Dir(p) == r.c.Root {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return fmt.Errorf("nested repository is unsupported: %s", search.Visible(filepath.Dir(p)))
		}
		return nil
	})
	if e != nil {
		return s, e
	}
	b, e := r.capture("config", "--get-all", "branch."+s.Branch+".remote")
	if e != nil {
		return s, fmt.Errorf("missing upstream; configure with git push --set-upstream REMOTE BRANCH")
	}
	remotes := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	b, e = r.capture("config", "--get-all", "branch."+s.Branch+".merge")
	if e != nil {
		return s, fmt.Errorf("missing upstream branch; configure git branch --set-upstream-to")
	}
	refs := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(remotes) != 1 || len(refs) != 1 {
		return s, fmt.Errorf("sync requires exactly one upstream remote and branch")
	}
	s.Remote, s.Ref = remotes[0], refs[0]
	if s.Remote == "" || strings.HasPrefix(s.Remote, "-") || strings.ContainsAny(s.Remote, "\x00\r\n") || !strings.HasPrefix(s.Ref, "refs/heads/") {
		return s, fmt.Errorf("invalid upstream remote/branch")
	}
	if _, e = r.capture("check-ref-format", s.Ref); e != nil {
		return s, e
	}
	s.Upstream, e = r.text("for-each-ref", "--format=%(upstream)", "refs/heads/"+s.Branch)
	if e != nil {
		return s, e
	}
	if s.Upstream == "" {
		return s, fmt.Errorf("upstream is not resolvable; configure a fetched remote branch")
	}
	s.Admin, e = r.text("rev-parse", "--absolute-git-dir")
	return s, e
}

// Check is read-only and never contacts a remote or acquires a lock.
func Check(ctx context.Context, c config.Config) (State, error) {
	return (repo{c: c, ctx: ctx, o: Options{Err: io.Discard}}).preflight()
}
func lock(admin string) (*os.File, error) {
	f, e := os.OpenFile(filepath.Join(admin, "note-sync.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); e != nil {
		f.Close()
		return nil, fmt.Errorf("another note sync holds the repository lock: %w", e)
	}
	return f, nil
}
func canonical(p string) string { return cases.Fold().String(norm.NFD.String(p)) }
func Collides(local, remote string) bool {
	a, b := canonical(local), canonical(remote)
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}
func (r repo) collision(tip string) error {
	// --others without exclude rules includes ordinary AND ignored untracked files.
	b, e := r.capture("ls-files", "--others", "-z")
	if e != nil {
		return e
	}
	locals := bytes.Split(b, []byte{0})
	b, e = r.capture("ls-tree", "-r", "--name-only", "-z", tip)
	if e != nil {
		return e
	}
	tracked := bytes.Split(b, []byte{0})
	for _, a := range locals {
		if len(a) == 0 {
			continue
		}
		for _, b := range tracked {
			if len(b) > 0 && Collides(string(a), string(b)) {
				return fmt.Errorf("local untracked/ignored path %s collides with fetched %s; review both paths before integration", search.Visible(string(a)), search.Visible(string(b)))
			}
		}
	}
	return nil
}
func Run(ctx context.Context, c config.Config, o Options) error {
	if o.Err == nil {
		o.Err = io.Discard
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Message != nil && strings.TrimSpace(*o.Message) == "" {
		return errs.New(2, "--message cannot be empty")
	}
	r := repo{c, o, ctx}
	s, e := r.preflight()
	if e != nil {
		return e
	}
	f, e := lock(s.Admin)
	if e != nil {
		return e
	}
	defer f.Close()
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
	s, e = r.preflight()
	if e != nil {
		return e
	}
	status, e := r.capture("status", "--porcelain=v1", "-z", "--untracked-files=all")
	if e != nil {
		return e
	}
	if len(status) > 0 {
		for _, identity := range []string{"GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT"} {
			if _, e = r.capture("var", identity); e != nil {
				return fmt.Errorf("a local commit needs a Git identity; configure user.name/user.email before staging: %w", e)
			}
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	fmt.Fprintln(o.Err, "Sync: staging saved repository changes.")
	if e = r.run("add", "--all", "--", "."); e != nil {
		return e
	}
	clean, e := r.predicate("diff", "--cached", "--quiet", "--exit-code")
	if e != nil {
		return e
	}
	if !clean {
		before, e := r.text("rev-parse", "HEAD")
		if e != nil {
			return e
		}
		message := strings.ReplaceAll(c.Sync.CommitMessage, "{timestamp}", o.Now().UTC().Format(time.RFC3339))
		if o.Message != nil {
			message = *o.Message
		}
		if e = r.run("commit", "--message", message); e != nil {
			// A post-commit hook can fail/be interrupted after Git recorded HEAD.
			// Read it with a separate bounded context even when sync was canceled.
			inspect, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			reader := r
			reader.ctx = inspect
			if tip, inspectErr := reader.text("rev-parse", "HEAD"); inspectErr == nil && tip != before {
				fmt.Fprintf(o.Err, "Sync: local commit %s is present; inspect git status.\n", tip)
			}
			cancel()
			return fmt.Errorf("commit failed; index/working files are preserved; no fetch occurred: %w", e)
		}
		tip, e := r.text("rev-parse", "HEAD")
		if e != nil {
			return e
		}
		fmt.Fprintf(o.Err, "Sync: local commit %s recorded.\n", tip)
	} else {
		fmt.Fprintln(o.Err, "Sync: no automatic commit needed.")
	}
	status, e = r.capture("status", "--porcelain=v1", "-z", "--untracked-files=all")
	if e != nil {
		return e
	}
	if len(status) > 0 {
		return fmt.Errorf("working tree changed during staging/commit (possibly a hook); review and retry; no integration occurred")
	}
	previous, e := r.text("rev-parse", "--verify", s.Upstream+"^{commit}")
	if e != nil {
		return fmt.Errorf("previous upstream is unavailable; fetch/configure the upstream manually before syncing: %w", e)
	}
	fmt.Fprintf(o.Err, "Sync: fetching %s %s.\n", s.Remote, s.Ref)
	// Fetch to FETCH_HEAD only. Do not update the tracking ref until rewrite review
	// passes, so retrying cannot forget the previous upstream tip.
	if e = r.run("fetch", "--no-tags", "--no-recurse-submodules", "--refmap=", "--", s.Remote, s.Ref); e != nil {
		return fmt.Errorf("fetch failed; completed local commits remain; retry note sync: %w", e)
	}
	tip, e := r.text("rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if e != nil {
		return e
	}
	advanced, e := r.predicate("merge-base", "--is-ancestor", previous, tip)
	if e != nil {
		return e
	}
	if !advanced {
		return fmt.Errorf("upstream history was rewritten; local commits remain; inspect fetched history manually before retrying")
	}
	if e = r.collision(tip); e != nil {
		return e
	}
	if e = r.active(); e != nil {
		return e
	}
	status, e = r.capture("status", "--porcelain=v1", "-z", "--untracked-files=all")
	if e != nil {
		return e
	}
	if len(status) > 0 {
		return fmt.Errorf("working tree changed before rebase; review and retry")
	}
	fmt.Fprintf(o.Err, "Sync: rebasing onto fetched commit %s.\n", tip)
	if e = r.run("rebase", "--no-autostash", tip); e != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if active := r.active(); errs.Code(active) == 5 {
			return errs.New(5, "rebase stopped: %v; inspect git status, resolve/stage then git rebase --continue, or git rebase --abort; no push occurred", e)
		}
		return fmt.Errorf("rebase failed; local commits remain; inspect git status: %w", e)
	}
	status, e = r.capture("status", "--porcelain=v1", "-z", "--untracked-files=all")
	if e != nil {
		return e
	}
	if len(status) > 0 {
		return fmt.Errorf("working tree changed during rebase; review before retrying; no push occurred")
	}
	if e = r.run("update-ref", s.Upstream, tip, previous); e != nil {
		return fmt.Errorf("upstream tracking changed concurrently; inspect Git refs before retrying: %w", e)
	}
	fmt.Fprintf(o.Err, "Sync: pushing HEAD to %s %s.\n", s.Remote, s.Ref)
	if e = r.run("push", "--", s.Remote, "HEAD:"+s.Ref); e != nil {
		return fmt.Errorf("push failed; local commits remain; retry note sync after reviewing Git diagnostics: %w", e)
	}
	fmt.Fprintln(o.Err, "Sync complete.")
	return nil
}
