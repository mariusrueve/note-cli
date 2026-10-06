// Package update installs verified GitHub release binaries. It never touches
// knowledge data, configuration, or executables owned by another installer.
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const Repository = "mariusrueve/note-cli"
const releaseBase = "https://github.com/" + Repository + "/releases/download/"
const maxArchive = 64 << 20
const maxBinary = 96 << 20

// Client's transport can be replaced in tests; production URLs are fixed.
type Client struct{ HTTP *http.Client }

func ParseVersion(s string) ([3]uint64, error) {
	var v [3]uint64
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, fmt.Errorf("expected a stable version such as 0.1.0")
	}
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') || strings.Trim(part, "0123456789") != "" {
			return v, fmt.Errorf("invalid stable version %q", s)
		}
		n, e := strconv.ParseUint(part, 10, 64)
		if e != nil {
			return v, fmt.Errorf("invalid stable version %q", s)
		}
		v[i] = n
	}
	return v, nil
}

func Newer(latest, current string) bool {
	a, e := ParseVersion(latest)
	b, f := ParseVersion(current)
	if e != nil || f != nil {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func ArchiveName(version, goos, goarch string) (string, error) {
	if _, e := ParseVersion(version); e != nil {
		return "", e
	}
	if (goos != "darwin" && goos != "linux") || (goarch != "arm64" && goarch != "amd64") {
		return "", fmt.Errorf("release installation is unsupported on %s/%s", goos, goarch)
	}
	return "note_" + version + "_" + goos + "_" + goarch + ".tar.gz", nil
}

func (c Client) get(ctx context.Context, address string, limit int64) ([]byte, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "note-release-updater")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many release redirects")
			}
			if req.URL.Scheme != "https" {
				return fmt.Errorf("release redirect must use HTTPS")
			}
			return nil
		}}
	}
	resp, e := client.Do(req)
	if e != nil {
		return nil, fmt.Errorf("download release: %w", e)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("no publicly accessible release found; private repositories require authenticated manual downloads")
		}
		return nil, fmt.Errorf("release server returned HTTP %d", resp.StatusCode)
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if e != nil {
		return nil, fmt.Errorf("read release: %w", e)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("release download exceeds size limit")
	}
	return data, nil
}

func (c Client) Latest(ctx context.Context) (string, error) {
	b, e := c.get(ctx, "https://api.github.com/repos/"+Repository+"/releases/latest", 1<<20)
	if e != nil {
		return "", e
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if e = json.Unmarshal(b, &release); e != nil {
		return "", fmt.Errorf("read release metadata: %w", e)
	}
	if release.Draft || release.Prerelease || !strings.HasPrefix(release.Tag, "v") {
		return "", fmt.Errorf("latest release is not a published stable version")
	}
	version := strings.TrimPrefix(release.Tag, "v")
	name, e := ArchiveName(version, runtime.GOOS, runtime.GOARCH)
	if e != nil {
		return "", e
	}
	assets := map[string]int{}
	for _, asset := range release.Assets {
		assets[asset.Name]++
	}
	if assets[name] != 1 || assets["checksums.txt"] != 1 {
		return "", fmt.Errorf("release %s is missing its archive or checksums", version)
	}
	return version, nil
}

func checksum(data []byte, name string) (string, error) {
	var result string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != name {
			continue
		}
		hash, e := hex.DecodeString(fields[0])
		if e != nil || len(hash) != sha256.Size || result != "" {
			return "", fmt.Errorf("invalid or duplicate archive checksum")
		}
		result = strings.ToLower(fields[0])
	}
	if result == "" {
		return "", fmt.Errorf("release has no checksum for %s", name)
	}
	return result, nil
}

func (c Client) Binary(ctx context.Context, version string) ([]byte, error) {
	name, e := ArchiveName(version, runtime.GOOS, runtime.GOARCH)
	if e != nil {
		return nil, e
	}
	base := releaseBase + "v" + url.PathEscape(version) + "/"
	sums, e := c.get(ctx, base+"checksums.txt", 64<<10)
	if e != nil {
		return nil, e
	}
	want, e := checksum(sums, name)
	if e != nil {
		return nil, e
	}
	archive, e := c.get(ctx, base+name, maxArchive)
	if e != nil {
		return nil, e
	}
	if fmt.Sprintf("%x", sha256.Sum256(archive)) != want {
		return nil, fmt.Errorf("release archive checksum mismatch; installation was not changed")
	}
	return extract(archive)
}

func extract(data []byte) ([]byte, error) {
	gz, e := gzip.NewReader(bytes.NewReader(data))
	if e != nil {
		return nil, fmt.Errorf("read release archive: %w", e)
	}
	defer gz.Close()
	// Bound decompression, including padding and data after tar's end marker.
	bounded := &io.LimitedReader{R: gz, N: maxBinary + (8 << 20) + 1}
	tr := tar.NewReader(bounded)
	seen := map[string]bool{}
	var binary []byte
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("read release archive: %w", e)
		}
		limit := int64(4 << 20)
		if h.Name == "note" {
			limit = maxBinary
		}
		if (h.Name != "note" && h.Name != "README.md" && h.Name != "LICENSE") || seen[h.Name] || h.Typeflag != tar.TypeReg || h.Size <= 0 || h.Size > limit {
			return nil, fmt.Errorf("unexpected release archive member %q", h.Name)
		}
		seen[h.Name] = true
		b, e := io.ReadAll(tr)
		if e != nil {
			return nil, e
		}
		if h.Name == "note" {
			binary = b
		}
	}
	if _, e = io.Copy(io.Discard, bounded); e != nil {
		return nil, fmt.Errorf("read release archive trailer: %w", e)
	}
	if bounded.N == 0 {
		return nil, fmt.Errorf("release archive exceeds decompression limit")
	}
	if len(seen) != 3 || len(binary) == 0 {
		return nil, fmt.Errorf("release archive is incomplete")
	}
	return binary, nil
}

func CachePath(home func() (string, error)) (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if !filepath.IsAbs(base) {
		if home == nil {
			home = os.UserHomeDir
		}
		h, e := home()
		if e != nil {
			return "", e
		}
		base = filepath.Join(h, ".cache")
	}
	return filepath.Join(base, "note", "update.json"), nil
}

// Hint makes at most one attempt per day. Callers bound the context and ignore
// errors: update availability must never change the result of a note operation.
func (c Client) Hint(ctx context.Context, path, current string, now time.Time) string {
	if _, e := ParseVersion(current); e != nil {
		return ""
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return ""
	}
	unlock, e := lock(filepath.Dir(path))
	if e != nil {
		return ""
	}
	defer unlock()
	var state struct {
		Repository string    `json:"repository"`
		Checked    time.Time `json:"checked"`
		Latest     string    `json:"latest"`
	}
	if info, e := os.Lstat(path); e == nil && info.Mode().IsRegular() && info.Size() <= 4096 {
		b, _ := os.ReadFile(path)
		_ = json.Unmarshal(b, &state)
	}
	age := now.Sub(state.Checked)
	if state.Repository != Repository || age < 0 || age >= 24*time.Hour {
		if state.Repository != Repository {
			state.Latest = ""
		}
		state.Repository, state.Checked = Repository, now
		// Persist the attempt before contacting GitHub. If caching is unavailable,
		// skip automatic network access rather than retrying on every invocation.
		b, e := json.Marshal(state)
		if e != nil || writeCache(path, b) != nil {
			return ""
		}
		if latest, e := c.Latest(ctx); e == nil {
			state.Latest = latest
			if b, e := json.Marshal(state); e == nil {
				_ = writeCache(path, b)
			}
		}
	}
	if Newer(state.Latest, current) {
		return state.Latest
	}
	return ""
}

func writeCache(path string, data []byte) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".update-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	_, e = f.Write(data)
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Rename(f.Name(), path)
}
