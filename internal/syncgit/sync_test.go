package syncgit

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mariusrueve/note-cli/internal/config"
	"github.com/mariusrueve/note-cli/internal/errs"
)

type matrix struct{ a, b, remote string }

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v\n%s", args, e, b)
	}
	return strings.TrimSuffix(string(b), "\n")
}
func write(t *testing.T, root, path, text string) {
	t.Helper()
	p := filepath.Join(root, path)
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(text), 0600); e != nil {
		t.Fatal(e)
	}
}
func read(t *testing.T, root, path string) string {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(root, path))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func setup(t *testing.T) matrix {
	t.Helper()
	base := t.TempDir()
	base, _ = filepath.EvalSymlinks(base)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(base, "empty-config"))
	t.Setenv("GIT_CONFIG_COUNT", "0")
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_COMMITTER_EMAIL", "")
	// Empty identity environment variables override local config, so remove them
	// and restore through Cleanup rather than changing the user's HOME.
	for _, key := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		os.Unsetenv(key)
	}
	m := matrix{filepath.Join(base, "a"), filepath.Join(base, "b"), filepath.Join(base, "remote.git")}
	git(t, base, "init", "--bare", "--initial-branch=main", m.remote)
	git(t, base, "clone", m.remote, m.a)
	identity(t, m.a)
	write(t, m.a, "shared.md", "base\n")
	write(t, m.a, ".gitignore", "*.local\nignored\nlocaldir/\n")
	git(t, m.a, "add", "--all")
	git(t, m.a, "commit", "-m", "initial")
	git(t, m.a, "push", "-u", "origin", "main")
	git(t, base, "clone", m.remote, m.b)
	identity(t, m.b)
	return m
}
func identity(t *testing.T, dir string) {
	git(t, dir, "config", "user.name", "Note Integration Test")
	git(t, dir, "config", "user.email", "note-test@example.invalid")
	git(t, dir, "config", "commit.gpgsign", "false")
}
func cfg(root string) config.Config {
	return config.Config{Root: root, Tools: config.Tools{Git: []string{"git"}}, Sync: config.Sync{CommitMessage: "notes: sync {timestamp}"}}
}
func sync(t *testing.T, root string, want int) string {
	t.Helper()
	var out bytes.Buffer
	e := Run(context.Background(), cfg(root), Options{Err: &out, Now: func() time.Time { return time.Date(2026, 10, 6, 10, 11, 12, 0, time.UTC) }})
	if errs.Code(e) != want {
		t.Fatalf("sync exit %d, want %d: %v\n%s", errs.Code(e), want, e, out.String())
	}
	if e != nil {
		out.WriteString(e.Error())
	}
	return out.String()
}
func remoteCommit(t *testing.T, m matrix, path, text string) {
	write(t, m.a, path, text)
	git(t, m.a, "add", "--all")
	git(t, m.a, "commit", "-m", "remote edit")
	git(t, m.a, "push")
}
func TestGitMatrix(t *testing.T) {
	t.Run("unchanged receiving and unpublished commits", func(t *testing.T) {
		m := setup(t)
		head := git(t, m.b, "rev-parse", "HEAD")
		sync(t, m.b, 0)
		if git(t, m.b, "rev-parse", "HEAD") != head {
			t.Fatal("empty commit")
		}
		remoteCommit(t, m, "remote.md", "remote\n")
		git(t, m.b, "config", "--unset", "user.name")
		git(t, m.b, "config", "--unset", "user.email")
		sync(t, m.b, 0)
		if read(t, m.b, "remote.md") != "remote\n" {
			t.Fatal("not received")
		}
		if git(t, m.b, "rev-list", "--count", "HEAD") != "2" {
			t.Fatal("extra commit")
		}
		identity(t, m.b)
		write(t, m.b, "published.md", "local\n")
		git(t, m.b, "add", "--all")
		git(t, m.b, "commit", "-m", "manual")
		sync(t, m.b, 0)
		if git(t, m.remote, "rev-parse", "main") != git(t, m.b, "rev-parse", "HEAD") {
			t.Fatal("not pushed")
		}
	})
	t.Run("full scope and stash preservation", func(t *testing.T) {
		m := setup(t)
		write(t, m.b, "shared.md", "stash contents\n")
		git(t, m.b, "stash", "push", "-m", "personal")
		stash := git(t, m.b, "stash", "list")
		patch := git(t, m.b, "stash", "show", "-p")
		write(t, m.b, "staged.md", "stage\n")
		git(t, m.b, "add", "staged.md")
		write(t, m.b, "staged.md", "unstaged\n")
		write(t, m.b, "attachment.png", "binary\x00bytes")
		os.Remove(filepath.Join(m.b, "shared.md"))
		write(t, m.b, "ignored.local", "private")
		sync(t, m.b, 0)
		if git(t, m.b, "rev-list", "--count", "HEAD") != "2" {
			t.Fatal("commit count")
		}
		if git(t, m.b, "show", "HEAD:staged.md") != "unstaged" {
			t.Fatal("unstaged not committed")
		}
		if git(t, m.b, "show", "HEAD:attachment.png") != "binary\x00bytes" {
			t.Fatal("attachment")
		}
		if strings.Contains(git(t, m.b, "ls-tree", "-r", "--name-only", "HEAD"), "shared.md") {
			t.Fatal("deletion")
		}
		if git(t, m.b, "stash", "list") != stash || git(t, m.b, "stash", "show", "-p") != patch {
			t.Fatal("stash changed")
		}
		if git(t, m.b, "log", "-1", "--format=%s") != "notes: sync 2026-10-06T10:11:12Z" {
			t.Fatal("timestamp")
		}
	})
	t.Run("independent changes", func(t *testing.T) {
		m := setup(t)
		remoteCommit(t, m, "remote.md", "remote\n")
		write(t, m.b, "local.md", "local\n")
		sync(t, m.b, 0)
		if read(t, m.b, "remote.md") != "remote\n" || read(t, m.b, "local.md") != "local\n" {
			t.Fatal("lost files")
		}
		if git(t, m.remote, "rev-list", "--count", "main") != "3" {
			t.Fatal("history")
		}
	})
	for _, recovery := range []string{"abort", "continue"} {
		t.Run("conflict "+recovery, func(t *testing.T) {
			m := setup(t)
			write(t, m.b, "stash.md", "personal\n")
			git(t, m.b, "stash", "push", "--include-untracked", "-m", "personal")
			stash := git(t, m.b, "stash", "list")
			remoteCommit(t, m, "shared.md", "remote\n")
			remoteHead := git(t, m.remote, "rev-parse", "main")
			write(t, m.b, "shared.md", "local\n")
			sync(t, m.b, 5)
			localHead := git(t, m.b, "rev-parse", "ORIG_HEAD")
			if git(t, m.b, "show", localHead+":shared.md") != "local" {
				t.Fatal("local data lost")
			}
			if git(t, m.remote, "rev-parse", "main") != remoteHead {
				t.Fatal("pushed conflict")
			}
			sync(t, m.b, 5)
			if recovery == "abort" {
				git(t, m.b, "rebase", "--abort")
				if git(t, m.b, "rev-parse", "HEAD") != localHead || read(t, m.b, "shared.md") != "local\n" {
					t.Fatal("abort lost automatic commit")
				}
				sync(t, m.b, 5)
			}
			write(t, m.b, "shared.md", "resolved\n")
			git(t, m.b, "add", "shared.md")
			t.Setenv("GIT_EDITOR", "true")
			git(t, m.b, "rebase", "--continue")
			sync(t, m.b, 0)
			if read(t, m.b, "shared.md") != "resolved\n" {
				t.Fatal("recovery")
			}
			if git(t, m.b, "stash", "list") != stash {
				t.Fatal("stash changed")
			}
		})
	}
	t.Run("same new path conflict", func(t *testing.T) {
		m := setup(t)
		remoteCommit(t, m, "same.md", "remote\n")
		write(t, m.b, "same.md", "local\n")
		sync(t, m.b, 5)
		git(t, m.b, "rebase", "--abort")
		if read(t, m.b, "same.md") != "local\n" {
			t.Fatal("add/add lost local")
		}
	})
	for _, pair := range []struct{ name, local, remote string }{{"exact", "ignored", "ignored"}, {"local file remote directory", "ignored", "ignored/remote.md"}, {"local directory remote file", "localdir/private", "localdir"}, {"case", "ignored.local", "IGNORED.LOCAL"}, {"normalization", "localdir/é", "LOCALDIR/é"}} {
		t.Run("collision "+pair.name, func(t *testing.T) {
			m := setup(t)
			write(t, m.b, pair.local, "private bytes\n")
			write(t, m.a, pair.remote, "remote bytes\n")
			git(t, m.a, "add", "-f", "--", pair.remote)
			git(t, m.a, "commit", "-m", "track ignored file")
			git(t, m.a, "push")
			head := git(t, m.b, "rev-parse", "HEAD")
			sync(t, m.b, 3)
			if read(t, m.b, pair.local) != "private bytes\n" {
				t.Fatal("local bytes lost")
			}
			if git(t, m.b, "rev-parse", "HEAD") != head {
				t.Fatal("integrated collision")
			}
			if _, e := Check(context.Background(), cfg(m.b)); e != nil {
				t.Fatal("entered Git operation", e)
			}
		})
	}
	t.Run("fetch failure and retry", func(t *testing.T) {
		m := setup(t)
		git(t, m.b, "remote", "set-url", "origin", filepath.Join(m.b, "missing-remote"))
		write(t, m.b, "local.md", "saved\n")
		sync(t, m.b, 3)
		head := git(t, m.b, "rev-parse", "HEAD")
		if git(t, m.b, "show", "HEAD:local.md") != "saved" {
			t.Fatal("local commit missing")
		}
		git(t, m.b, "remote", "set-url", "origin", m.remote)
		sync(t, m.b, 0)
		if git(t, m.b, "rev-parse", "HEAD") != head {
			t.Fatal("retry created extra commit")
		}
	})
	t.Run("missing identity before staging", func(t *testing.T) {
		m := setup(t)
		// Some CI hosts have a hostname that lets Git infer a valid identity.
		// Explicitly require configured identity to exercise the failure path.
		git(t, m.b, "config", "user.useConfigOnly", "true")
		git(t, m.b, "config", "--unset", "user.name")
		git(t, m.b, "config", "--unset", "user.email")
		write(t, m.b, "local.md", "saved\n")
		before := git(t, m.b, "ls-files", "--stage")
		sync(t, m.b, 3)
		if git(t, m.b, "ls-files", "--stage") != before {
			t.Fatal("staged before identity validation")
		}
	})
	t.Run("hook failure and hook extra changes", func(t *testing.T) {
		m := setup(t)
		write(t, m.b, "local.md", "saved\n")
		hook := filepath.Join(m.b, ".git", "hooks", "pre-commit")
		os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0755)
		sync(t, m.b, 3)
		if git(t, m.b, "rev-list", "--count", "HEAD") != "1" || !strings.Contains(git(t, m.b, "diff", "--cached", "--name-only"), "local.md") {
			t.Fatal("hook failure preservation")
		}
		os.Remove(hook)
		hook = filepath.Join(m.b, ".git", "hooks", "post-commit")
		os.WriteFile(hook, []byte("#!/bin/sh\nprintf 'extra\\n' > hook.md\n"), 0755)
		sync(t, m.b, 3)
		if git(t, m.b, "rev-list", "--count", "HEAD") != "2" || git(t, m.remote, "rev-list", "--count", "main") != "1" {
			t.Fatal("hook changes integrated")
		}
	})
	t.Run("push rejection and retry", func(t *testing.T) {
		m := setup(t)
		hook := filepath.Join(m.remote, "hooks", "pre-receive")
		os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0755)
		write(t, m.b, "local.md", "local\n")
		sync(t, m.b, 3)
		head := git(t, m.b, "rev-parse", "HEAD")
		os.Remove(hook)
		sync(t, m.b, 0)
		if git(t, m.remote, "rev-parse", "main") != head {
			t.Fatal("retry push")
		}
	})
	t.Run("upstream rewritten stays refused", func(t *testing.T) {
		m := setup(t)
		remoteCommit(t, m, "first.md", "first\n")
		sync(t, m.b, 0)
		git(t, m.a, "reset", "--hard", "HEAD~1")
		write(t, m.a, "rewrite.md", "rewrite\n")
		git(t, m.a, "add", "--all")
		git(t, m.a, "commit", "-m", "rewrite")
		git(t, m.a, "push", "--force")
		head := git(t, m.b, "rev-parse", "HEAD")
		sync(t, m.b, 3)
		sync(t, m.b, 3)
		if git(t, m.b, "rev-parse", "HEAD") != head {
			t.Fatal("rewritten integrated")
		}
	})
}
func TestPreflightAndLock(t *testing.T) {
	for _, kind := range []string{"missing upstream", "subdirectory", "detached", "unborn", "sparse", "nested", "submodule", "merge state", "unmerged index", "worktree"} {
		t.Run(kind, func(t *testing.T) {
			m := setup(t)
			root := m.b
			want := 3
			switch kind {
			case "missing upstream":
				git(t, m.b, "branch", "--unset-upstream")
			case "subdirectory":
				root = filepath.Join(m.b, "sub")
				os.Mkdir(root, 0755)
			case "detached":
				git(t, m.b, "checkout", "--detach")
			case "unborn":
				root = filepath.Join(filepath.Dir(m.b), "unborn")
				os.Mkdir(root, 0755)
				git(t, root, "init")
			case "sparse":
				git(t, m.b, "config", "core.sparseCheckout", "true")
			case "nested":
				os.MkdirAll(filepath.Join(m.b, "nested", ".git"), 0755)
			case "submodule":
				oid := git(t, m.b, "rev-parse", "HEAD")
				git(t, m.b, "update-index", "--add", "--cacheinfo", "160000,"+oid+",submodule")
			case "merge state":
				write(t, m.b, ".git/MERGE_HEAD", git(t, m.b, "rev-parse", "HEAD"))
				want = 5
			case "unmerged index":
				remoteCommit(t, m, "shared.md", "remote\n")
				write(t, m.b, "shared.md", "local\n")
				git(t, m.b, "add", "--all")
				git(t, m.b, "commit", "-m", "local")
				cmd := exec.Command("git", "pull", "--no-rebase")
				cmd.Dir = m.b
				cmd.Run()
				os.Remove(filepath.Join(m.b, ".git", "MERGE_HEAD"))
				want = 5
			case "worktree":
				root = filepath.Join(filepath.Dir(m.b), "worktree")
				git(t, m.b, "worktree", "add", "-b", "worktree", root)
				git(t, root, "branch", "--set-upstream-to=origin/main")
				want = 0
			}
			before := git(t, root, "ls-files", "--stage")
			sync(t, root, want)
			if want != 0 && git(t, root, "ls-files", "--stage") != before {
				t.Fatal("preflight mutated index")
			}
		})
	}
	m := setup(t)
	s, e := Check(context.Background(), cfg(m.b))
	if e != nil {
		t.Fatal(e)
	}
	f, e := lock(s.Admin)
	if e != nil {
		t.Fatal(e)
	}
	sync(t, m.b, 3)
	f.Close()
	sync(t, m.b, 0)
	if _, e = os.Stat(filepath.Join(s.Admin, "note-sync.lock")); e != nil {
		t.Fatal("lock inode removed")
	}
}
func TestCancellationAndLiteralMessage(t *testing.T) {
	m := setup(t)
	write(t, m.b, "local.md", "saved\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := Run(ctx, cfg(m.b), Options{Err: io.Discard})
	if errs.Code(e) != 130 {
		t.Fatal(e)
	}
	message := "literal {timestamp} $(no) `no`"
	e = Run(context.Background(), cfg(m.b), Options{Err: io.Discard, Message: &message})
	if e != nil {
		t.Fatal(e)
	}
	if git(t, m.b, "log", "-1", "--format=%s") != message {
		t.Fatal("message interpreted")
	}
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
func waitFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, e := os.Stat(path); e == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}
func TestInstalledSignalsAndConcurrentSyncs(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "note")
	build := exec.Command("go", "build", "-mod=readonly", "-o", binary, "./cmd/note")
	build.Dir = "../.."
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	for _, phase := range []string{"after commit", "during rebase"} {
		t.Run(phase, func(t *testing.T) {
			m := setup(t)
			marker := filepath.Join(filepath.Dir(m.b), "phase.marker")
			conf := filepath.Join(filepath.Dir(m.b), "config.toml")
			text := "root=" + strconv.Quote(m.b) + "\n"
			if phase == "after commit" {
				write(t, m.b, "local.md", "preserved\n")
				hook := filepath.Join(m.b, ".git", "hooks", "post-commit")
				os.WriteFile(hook, []byte("#!/bin/sh\nprintf ready > "+shellQuote(marker)+"\nsleep 30\n"), 0755)
			} else {
				remoteCommit(t, m, "shared.md", "remote\n")
				write(t, m.b, "shared.md", "local\n")
				gitPath, _ := exec.LookPath("git")
				wrapper := filepath.Join(filepath.Dir(m.b), "git-wrapper")
				script := "#!/bin/sh\nif [ \"$1\" = rebase ]; then\n" + shellQuote(gitPath) + " \"$@\"\nstatus=$?\nprintf ready > " + shellQuote(marker) + "\nsleep 30\nexit $status\nfi\nexec " + shellQuote(gitPath) + " \"$@\"\n"
				os.WriteFile(wrapper, []byte(script), 0755)
				text += "[tools]\ngit=[" + strconv.Quote(wrapper) + "]\n"
			}
			os.WriteFile(conf, []byte(text), 0600)
			cmd := exec.Command(binary, "--config", conf, "sync")
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			if e := cmd.Start(); e != nil {
				t.Fatal(e)
			}
			defer func() {
				if cmd.ProcessState == nil {
					cmd.Process.Kill()
					cmd.Wait()
				}
			}()
			waitFile(t, marker)
			if phase == "after commit" {
				second := exec.Command(binary, "--config", conf, "sync")
				b, e := second.CombinedOutput()
				if e == nil || second.ProcessState.ExitCode() != 3 || !bytes.Contains(b, []byte("lock")) {
					t.Fatalf("second sync: %s %v", b, e)
				}
			}
			if e := cmd.Process.Signal(syscall.SIGINT); e != nil {
				t.Fatal(e)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case <-done:
			case <-time.After(8 * time.Second):
				cmd.Process.Kill()
				t.Fatal("cancellation did not finish")
			}
			if cmd.ProcessState.ExitCode() != 130 {
				t.Fatalf("interrupt exit %d: %s", cmd.ProcessState.ExitCode(), output.String())
			}
			if phase == "after commit" {
				os.Remove(filepath.Join(m.b, ".git", "hooks", "post-commit"))
				if !strings.Contains(output.String(), git(t, m.b, "rev-parse", "HEAD")) {
					t.Fatal("interrupted commit ID was not reported", output.String())
				}
				if git(t, m.b, "show", "HEAD:local.md") != "preserved" {
					t.Fatal("interrupted commit lost")
				}
				sync(t, m.b, 0)
			} else {
				if _, e := Check(context.Background(), cfg(m.b)); errs.Code(e) != 5 {
					t.Fatal("interrupted rebase state missing", e)
				}
				git(t, m.b, "rebase", "--abort")
				if read(t, m.b, "shared.md") != "local\n" {
					t.Fatal("interrupted local contents lost")
				}
				sync(t, m.b, 5)
				write(t, m.b, "shared.md", "resolved\n")
				git(t, m.b, "add", "shared.md")
				t.Setenv("GIT_EDITOR", "true")
				git(t, m.b, "rebase", "--continue")
				sync(t, m.b, 0)
			}
		})
	}
	t.Run("headless receiving server", func(t *testing.T) {
		m := setup(t)
		remoteCommit(t, m, "server.md", "received\n")
		git(t, m.b, "config", "--unset", "user.name")
		git(t, m.b, "config", "--unset", "user.email")
		gitPath, _ := exec.LookPath("git")
		conf := filepath.Join(filepath.Dir(m.b), "server.toml")
		text := "root=" + strconv.Quote(m.b) + "\neditor=\"/missing-editor\"\n[tools]\ngit=[" + strconv.Quote(gitPath) + "]\nrg=[\"/missing-rg\"]\nfzf=[\"/missing-fzf\"]\n"
		os.WriteFile(conf, []byte(text), 0600)
		cmd := exec.Command(binary, "--config", conf, "sync")
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "PATH=") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, "PATH=/missing-tools")
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatal(string(b), e)
		}
		if read(t, m.b, "server.md") != "received\n" || git(t, m.b, "rev-list", "--count", "HEAD") != "2" {
			t.Fatal("server update")
		}
	})
}

func TestPushRace(t *testing.T) {
	m := setup(t)
	write(t, m.a, "race.md", "remote race\n")
	git(t, m.a, "add", "--all")
	git(t, m.a, "commit", "-m", "race")
	remoteTip := git(t, m.a, "rev-parse", "HEAD")
	git(t, m.remote, "fetch", m.a, "HEAD:refs/heads/race")
	gitPath, _ := exec.LookPath("git")
	hook := filepath.Join(m.b, ".git", "hooks", "pre-push")
	script := "#!/bin/sh\n" + shellQuote(gitPath) + " --git-dir=" + shellQuote(m.remote) + " update-ref refs/heads/main " + remoteTip + "\n"
	os.WriteFile(hook, []byte(script), 0755)
	write(t, m.b, "local.md", "local\n")
	sync(t, m.b, 3)
	os.Remove(hook)
	sync(t, m.b, 0)
	if read(t, m.b, "race.md") != "remote race\n" || read(t, m.b, "local.md") != "local\n" {
		t.Fatal("race retry lost data")
	}
	if git(t, m.remote, "rev-list", "--count", "main") != "3" {
		t.Fatal("race history")
	}
}
