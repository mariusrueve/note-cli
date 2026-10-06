package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArchive(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "note")
	os.WriteFile(binary, []byte("binary"), 0755)
	cwd, _ := os.Getwd()
	os.Chdir("../..")
	defer os.Chdir(cwd)
	p := filepath.Join(dir, "release.tar.gz")
	if e := archive(p, binary, []byte("Bundled license fixture\n")); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(p)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	gz, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	want := []string{"note", "README.md", "LICENSE"}
	for _, name := range want {
		h, e := tr.Next()
		if e != nil || h.Name != name {
			t.Fatal(h, e)
		}
		b, e := io.ReadAll(tr)
		if e != nil || len(b) == 0 {
			t.Fatal(e)
		}
		if name == "note" && (string(b) != "binary" || h.Mode != 0755) {
			t.Fatal(h, string(b))
		}
		if name == "LICENSE" && !bytes.Contains(b, []byte("Bundled license fixture")) {
			t.Fatal("archive omitted dependency notices")
		}
	}
	if _, e = tr.Next(); e != io.EOF {
		t.Fatal("unexpected archive entry", e)
	}
	first, _ := os.ReadFile(p)
	if e = os.Chtimes(binary, time.Now(), time.Now()); e != nil {
		t.Fatal(e)
	}
	p2 := filepath.Join(dir, "again.tar.gz")
	if e = archive(p2, binary, []byte("Bundled license fixture\n")); e != nil {
		t.Fatal(e)
	}
	second, _ := os.ReadFile(p2)
	if !bytes.Equal(first, second) {
		t.Fatal("archive depends on source timestamps")
	}
}

func TestVersionedReleaseRequiresCleanCheckout(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(root, "missing-config"))
	t.Setenv("GIT_CONFIG_COUNT", "0")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatal(string(out), e)
		}
	}
	git("init", "-q", "-b", "main")
	if e := os.WriteFile("tracked", []byte("original"), 0600); e != nil {
		t.Fatal(e)
	}
	git("add", "tracked")
	git("-c", "user.name=Release Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	output := t.TempDir()
	for _, version := range []string{"../outside", "-bad", "version with space"} {
		if e := run(version, output); e == nil || !strings.Contains(e.Error(), "invalid version") {
			t.Fatal(version, e)
		}
	}
	for _, file := range []string{"untracked", "tracked"} {
		if e := os.WriteFile(file, []byte("changes"), 0600); e != nil {
			t.Fatal(e)
		}
		if e := run("0.1.0", output); e == nil || !strings.Contains(e.Error(), "clean Git checkout") {
			t.Fatal(e)
		}
		entries, e := os.ReadDir(output)
		if e != nil || len(entries) != 0 {
			t.Fatal("release wrote output before source validation", entries, e)
		}
		if file == "untracked" {
			if e := os.Remove(file); e != nil {
				t.Fatal(e)
			}
		}
	}
}
