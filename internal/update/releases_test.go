package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func clientFor(t *testing.T, files map[string][]byte, calls *int) Client {
	t.Helper()
	return Client{HTTP: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		*calls++
		if r.URL.Scheme != "https" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected release request: %s", r.URL)
		}
		b, ok := files[r.URL.Path]
		status := 200
		if !ok {
			status = 404
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(b)), Header: make(http.Header)}, nil
	})}}
}

func metadata(version string) []byte {
	name, _ := ArchiveName(version, runtime.GOOS, runtime.GOARCH)
	b, _ := json.Marshal(map[string]any{"tag_name": "v" + version, "assets": []map[string]string{{"name": name}, {"name": "checksums.txt"}}})
	return b
}

type member struct {
	name     string
	typeflag byte
	data     []byte
}

func archiveFor(t *testing.T, members []member) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	for _, m := range members {
		h := &tar.Header{Name: m.name, Typeflag: m.typeflag, Mode: 0755, Size: int64(len(m.data))}
		if m.typeflag == tar.TypeSymlink {
			h.Size, h.Linkname = 0, "/outside"
		}
		if e := tw.WriteHeader(h); e != nil {
			t.Fatal(e)
		}
		if h.Size != 0 {
			if _, e := tw.Write(m.data); e != nil {
				t.Fatal(e)
			}
		}
	}
	if e := tw.Close(); e != nil {
		t.Fatal(e)
	}
	if e := gz.Close(); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}

func releaseFiles(t *testing.T, version string, binary []byte) map[string][]byte {
	t.Helper()
	name, _ := ArchiveName(version, runtime.GOOS, runtime.GOARCH)
	archive := archiveFor(t, []member{{"note", tar.TypeReg, binary}, {"README.md", tar.TypeReg, []byte("readme")}, {"LICENSE", tar.TypeReg, []byte("license")}})
	base := "/" + Repository + "/releases/download/v" + version + "/"
	return map[string][]byte{
		"/repos/" + Repository + "/releases/latest": metadata(version),
		base + name:            archive,
		base + "checksums.txt": []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), name)),
	}
}

func TestVersions(t *testing.T) {
	for _, version := range []string{"0.1.0", "10.20.30", "1.0.0"} {
		if _, e := ParseVersion(version); e != nil {
			t.Fatal(version, e)
		}
	}
	for _, version := range []string{"", "dev", "snapshot", "v1.0.0", "01.0.0", "1.0", "1.0.0-beta", "1.0.0\n", "+1.0.0", "18446744073709551616.0.0", "1.0.0/else"} {
		if _, e := ParseVersion(version); e == nil {
			t.Fatal("accepted invalid version", version)
		}
	}
	for _, tt := range []struct {
		latest, current string
		want            bool
	}{{"0.10.0", "0.9.9", true}, {"1.0.0", "0.99.99", true}, {"0.2.0", "0.2.0", false}, {"0.1.0", "0.2.0", false}, {"1.0.0", "dev", false}} {
		if Newer(tt.latest, tt.current) != tt.want {
			t.Fatal(tt)
		}
	}
	if _, e := ArchiveName("1.0.0", "windows", "amd64"); e == nil {
		t.Fatal("accepted unsupported platform")
	}
}

func TestLatestValidation(t *testing.T) {
	path := "/repos/" + Repository + "/releases/latest"
	for _, tt := range []struct {
		name string
		data []byte
		good bool
	}{
		{"stable", metadata("0.2.0"), true},
		{"private or missing", nil, false},
		{"malformed", []byte("{"), false},
		{"draft", []byte(`{"tag_name":"v0.2.0","draft":true}`), false},
		{"prerelease", []byte(`{"tag_name":"v0.2.0","prerelease":true}`), false},
		{"bad tag", []byte(`{"tag_name":"../../bad"}`), false},
		{"incomplete", []byte(`{"tag_name":"v0.2.0","assets":[]}`), false},
		{"oversized", bytes.Repeat([]byte("x"), (1<<20)+1), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string][]byte{}
			if tt.data != nil {
				files[path] = tt.data
			}
			calls := 0
			v, e := clientFor(t, files, &calls).Latest(context.Background())
			if (e == nil) != tt.good || calls != 1 || (tt.good && v != "0.2.0") {
				t.Fatal(v, e, calls)
			}
		})
	}
}

func TestArchiveValidation(t *testing.T) {
	good := []member{{"note", tar.TypeReg, []byte("binary")}, {"README.md", tar.TypeReg, []byte("readme")}, {"LICENSE", tar.TypeReg, []byte("license")}}
	for _, tt := range []struct {
		name    string
		members []member
		good    bool
	}{
		{"valid", good, true},
		{"missing", good[:2], false},
		{"duplicate", append(append([]member{}, good...), good[0]), false},
		{"traversal", append(append([]member{}, good...), member{"../outside", tar.TypeReg, []byte("bad")}), false},
		{"symlink", []member{{"note", tar.TypeSymlink, nil}}, false},
		{"empty", []member{{"note", tar.TypeReg, nil}}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, e := extract(archiveFor(t, tt.members))
			if (e == nil) != tt.good || (tt.good && string(b) != "binary") {
				t.Fatal(string(b), e)
			}
		})
	}
	b := archiveFor(t, good)
	b[len(b)-8] ^= 1 // gzip CRC, with an otherwise complete tar archive.
	if _, e := extract(b); e == nil {
		t.Fatal("accepted corrupt gzip trailer")
	}
}

func TestBinaryChecksums(t *testing.T) {
	files := releaseFiles(t, "0.2.0", []byte("binary"))
	name, _ := ArchiveName("0.2.0", runtime.GOOS, runtime.GOARCH)
	base := "/" + Repository + "/releases/download/v0.2.0/"
	for _, mutation := range []string{"valid", "wrong hash", "duplicate hash", "missing hash", "bad hash", "missing archive"} {
		t.Run(mutation, func(t *testing.T) {
			f := map[string][]byte{}
			for k, v := range files {
				f[k] = bytes.Clone(v)
			}
			sums := base + "checksums.txt"
			switch mutation {
			case "wrong hash":
				f[sums] = []byte(strings.Repeat("0", 64) + "  " + name + "\n")
			case "duplicate hash":
				f[sums] = append(f[sums], f[sums]...)
			case "missing hash":
				f[sums] = []byte("other file\n")
			case "bad hash":
				f[sums] = []byte("bad  " + name + "\n")
			case "missing archive":
				delete(f, base+name)
			}
			calls := 0
			b, e := clientFor(t, f, &calls).Binary(context.Background(), "0.2.0")
			if (e == nil) != (mutation == "valid") || (e == nil && string(b) != "binary") {
				t.Fatal(string(b), e)
			}
		})
	}
}

func TestHintCacheAndOffline(t *testing.T) {
	calls := 0
	files := map[string][]byte{"/repos/" + Repository + "/releases/latest": metadata("0.2.0")}
	c := clientFor(t, files, &calls)
	path := filepath.Join(t.TempDir(), "cache", "update.json")
	now := time.Now()
	if v := c.Hint(context.Background(), path, "0.1.0", now); v != "0.2.0" || calls != 1 {
		t.Fatal(v, calls)
	}
	delete(files, "/repos/"+Repository+"/releases/latest")
	if v := c.Hint(context.Background(), path, "0.1.0", now.Add(time.Hour)); v != "0.2.0" || calls != 1 {
		t.Fatal(v, calls)
	}
	if v := c.Hint(context.Background(), path, "0.2.0", now.Add(time.Hour)); v != "" || calls != 1 {
		t.Fatal(v, calls)
	}
	_ = c.Hint(context.Background(), path, "0.1.0", now.Add(25*time.Hour))
	if calls != 2 {
		t.Fatal(calls)
	}
	_ = c.Hint(context.Background(), path, "0.1.0", now.Add(26*time.Hour))
	if calls != 2 {
		t.Fatal("offline checks were not cached", calls)
	}
	_ = c.Hint(context.Background(), path, "dev", now.Add(48*time.Hour))
	if calls != 2 {
		t.Fatal("development build checked releases")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
}

func TestCachePaths(t *testing.T) {
	h := t.TempDir()
	for _, base := range []string{"", "relative", filepath.Join(h, "xdg")} {
		t.Setenv("XDG_CACHE_HOME", base)
		path, e := CachePath(func() (string, error) { return h, nil })
		want := filepath.Join(h, ".cache", "note", "update.json")
		if filepath.IsAbs(base) {
			want = filepath.Join(base, "note", "update.json")
		}
		if e != nil || path != want {
			t.Fatal(path, e)
		}
	}
}

func TestUnavailableCacheSkipsNetwork(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "file")
	if e := os.WriteFile(blocked, []byte("preserve"), 0600); e != nil {
		t.Fatal(e)
	}
	calls := 0
	c := clientFor(t, map[string][]byte{}, &calls)
	if v := c.Hint(context.Background(), filepath.Join(blocked, "update.json"), "0.1.0", time.Now()); v != "" || calls != 0 {
		t.Fatal(v, calls)
	}
	unlock, e := lock(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer unlock()
	if v := c.Hint(context.Background(), filepath.Join(dir, "update.json"), "0.1.0", time.Now()); v != "" || calls != 0 {
		t.Fatal(v, calls)
	}
}

func TestCanceledRequest(t *testing.T) {
	c := Client{HTTP: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, e := c.Latest(ctx); e == nil || time.Since(start) > time.Second {
		t.Fatal(e)
	}
}
