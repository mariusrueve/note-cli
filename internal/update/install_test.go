package update

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func installFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "bin with spaces")
	source := filepath.Join(t.TempDir(), "source")
	if e := os.WriteFile(source, []byte("original"), 0755); e != nil {
		t.Fatal(e)
	}
	path, e := Install(context.Background(), source, dir, "0.1.0")
	if e != nil {
		t.Fatal(e)
	}
	return path, source
}

func assertOriginal(t *testing.T, path string) {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil || string(b) != "original" {
		t.Fatal("original binary was changed", string(b), e)
	}
	entries, e := os.ReadDir(filepath.Dir(path))
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".note-binary-") || strings.HasPrefix(entry.Name(), ".note-receipt-") {
			t.Fatal("staged file was leaked", entry.Name())
		}
	}
}

func TestManagedInstall(t *testing.T) {
	path, source := installFixture(t)
	r, e := managed(path)
	if e != nil || r.Version != "0.1.0" {
		t.Fatal(r, e)
	}
	if _, e = Install(context.Background(), source, filepath.Dir(path), "0.1.0"); e != nil {
		t.Fatal(e)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0755 {
		t.Fatal(info.Mode())
	}
	if e := os.WriteFile(source, []byte("new binary"), 0755); e != nil {
		t.Fatal(e)
	}
	if _, e = Install(context.Background(), source, filepath.Dir(path), "0.2.0"); e != nil {
		t.Fatal(e)
	}
	if _, e = Install(context.Background(), source, filepath.Dir(path), "0.1.0"); e == nil {
		t.Fatal("allowed downgrade")
	}
	r, e = managed(path)
	if e != nil || r.Version != "0.2.0" {
		t.Fatal(r, e)
	}
}

func TestInstallGuards(t *testing.T) {
	for _, kind := range []string{"unmanaged", "changed", "receipt symlink", "binary symlink", "canceled", "locked"} {
		t.Run(kind, func(t *testing.T) {
			path, source := installFixture(t)
			ctx := context.Background()
			switch kind {
			case "unmanaged":
				os.Remove(receiptPath(path))
			case "changed":
				os.WriteFile(path, []byte("another installer"), 0755)
			case "receipt symlink":
				os.Rename(receiptPath(path), receiptPath(path)+".real")
				os.Symlink(receiptPath(path)+".real", receiptPath(path))
			case "binary symlink":
				os.Rename(path, path+".real")
				os.Symlink(path+".real", path)
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "locked":
				unlock, e := lock(filepath.Dir(path))
				if e != nil {
					t.Fatal(e)
				}
				defer unlock()
			}
			before, _ := os.ReadFile(path)
			if _, e := Install(ctx, source, filepath.Dir(path), "0.2.0"); e == nil {
				t.Fatal("guard failed", kind)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("changed protected executable")
			}
		})
	}
}

func TestSelfUpdate(t *testing.T) {
	for _, kind := range []string{"success", "checksum", "wrong version", "bad executable", "canceled", "changed receipt", "download missing"} {
		t.Run(kind, func(t *testing.T) {
			path, _ := installFixture(t)
			binary := []byte("#!/bin/sh\nprintf 'note version 0.2.0 (fixture)\\n'\n")
			if kind == "wrong version" {
				binary = []byte("#!/bin/sh\nprintf 'note version 0.3.0 (fixture)\\n'\n")
			}
			if kind == "bad executable" {
				binary = []byte("invalid executable")
			}
			files := releaseFiles(t, "0.2.0", binary)
			name, _ := ArchiveName("0.2.0", runtime.GOOS, runtime.GOARCH)
			base := "/" + Repository + "/releases/download/v0.2.0/"
			if kind == "checksum" {
				files[base+"checksums.txt"] = []byte(strings.Repeat("0", 64) + "  " + name + "\n")
			}
			if kind == "download missing" {
				delete(files, base+name)
			}
			if kind == "changed receipt" {
				os.WriteFile(receiptPath(path), []byte("{}"), 0600)
			}
			ctx := context.Background()
			if kind == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			calls := 0
			e := clientFor(t, files, &calls).SelfUpdate(ctx, path, "0.1.0", "0.2.0")
			if (e == nil) != (kind == "success") {
				t.Fatal(kind, e)
			}
			if e != nil {
				assertOriginal(t, path)
				return
			}
			r, e := managed(path)
			if e != nil || r.Version != "0.2.0" {
				t.Fatal(r, e)
			}
			b, _ := os.ReadFile(path)
			if !bytes.Equal(b, binary) {
				t.Fatal("wrong installed binary")
			}
		})
	}
}
