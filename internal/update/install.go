package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type receipt struct {
	Repository string `json:"repository"`
	Version    string `json:"version"`
	SHA256     string `json:"sha256"`
}

func receiptPath(binary string) string {
	return filepath.Join(filepath.Dir(binary), ".note-install.json")
}

func regularBytes(path string, limit int64) ([]byte, error) {
	i, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !i.Mode().IsRegular() || i.Size() > limit {
		return nil, fmt.Errorf("expected a regular, bounded file: %s", path)
	}
	return os.ReadFile(path)
}

func managed(binary string) (receipt, error) {
	var r receipt
	b, e := regularBytes(receiptPath(binary), 4096)
	if e != nil {
		return r, fmt.Errorf("installation is not managed by the release installer; use its original installer or package manager")
	}
	if e = json.Unmarshal(b, &r); e != nil || r.Repository != Repository {
		return r, fmt.Errorf("invalid release installation receipt")
	}
	if _, e = ParseVersion(r.Version); e != nil {
		return r, fmt.Errorf("invalid release installation version")
	}
	b, e = regularBytes(binary, maxBinary)
	if e != nil {
		return r, e
	}
	if fmt.Sprintf("%x", sha256.Sum256(b)) != r.SHA256 {
		return r, fmt.Errorf("installed executable was changed by another installer; use its original installer or package manager")
	}
	return r, nil
}

func lock(dir string) (func(), error) {
	fd, e := unix.Open(filepath.Join(dir, ".note-update.lock"), unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if e != nil {
		return nil, fmt.Errorf("open installation lock: %w", e)
	}
	var info unix.Stat_t
	if e = unix.Fstat(fd, &info); e != nil || info.Mode&unix.S_IFMT != unix.S_IFREG {
		unix.Close(fd)
		return nil, fmt.Errorf("installation lock must be a regular file")
	}
	if e = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); e != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("another note installation or update is running: %w", e)
	}
	return func() { _ = unix.Flock(fd, unix.LOCK_UN); _ = unix.Close(fd) }, nil
}

func stage(dir, pattern string, data []byte, mode os.FileMode) (string, error) {
	f, e := os.CreateTemp(dir, pattern)
	if e != nil {
		return "", e
	}
	name := f.Name()
	defer func() {
		if e != nil {
			os.Remove(name)
		}
	}()
	if _, e = f.Write(data); e == nil {
		e = f.Chmod(mode)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	return name, e
}

// replace stages both files on the destination filesystem. A canceled or failed
// download/validation leaves the old executable untouched. Rename is atomic.
func replace(ctx context.Context, path, version string, data []byte, old *receipt, validate bool) error {
	dir := filepath.Dir(path)
	temp, e := stage(dir, ".note-binary-*", data, 0755)
	if e != nil {
		return e
	}
	defer os.Remove(temp)
	if validate {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(checkCtx, temp, "--version")
		cmd.WaitDelay = time.Second
		var output bytes.Buffer
		cmd.Stdout = &boundedOutput{Writer: &output, Remaining: 4096}
		if e = cmd.Run(); e != nil {
			if checkCtx.Err() != nil {
				return checkCtx.Err()
			}
			return fmt.Errorf("validate downloaded executable: %w", e)
		}
		line := strings.TrimSpace(output.String())
		if !strings.HasPrefix(line, "note version "+version+" (") || !strings.HasSuffix(line, ")") || strings.ContainsAny(line, "\r\n") {
			return fmt.Errorf("downloaded executable has unexpected version")
		}
	}
	r := receipt{Repository, version, fmt.Sprintf("%x", sha256.Sum256(data))}
	metadata, e := json.Marshal(r)
	if e != nil {
		return e
	}
	marker, e := stage(dir, ".note-receipt-*", append(metadata, '\n'), 0600)
	if e != nil {
		return e
	}
	defer os.Remove(marker)
	if old != nil {
		current, e := managed(path)
		if e != nil {
			return e
		}
		if current != *old {
			return fmt.Errorf("installation changed while updating; retry")
		}
	} else if _, e = os.Lstat(path); !os.IsNotExist(e) {
		return fmt.Errorf("destination appeared while installing; refusing to overwrite it")
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	if e = os.Rename(temp, path); e != nil {
		return fmt.Errorf("replace executable: %w", e)
	}
	if e = os.Rename(marker, receiptPath(path)); e != nil {
		return fmt.Errorf("executable installed, but installation receipt could not be saved: %w", e)
	}
	return nil
}

type boundedOutput struct {
	io.Writer
	Remaining int
}

func (w *boundedOutput) Write(b []byte) (int, error) {
	if len(b) > w.Remaining {
		return 0, fmt.Errorf("version output exceeds size limit")
	}
	w.Remaining -= len(b)
	return w.Writer.Write(b)
}

// Install is called by the checksum-verified archive executable. Existing
// destinations must have a matching receipt; it never adopts another installer.
func Install(ctx context.Context, source, directory, version string) (string, error) {
	if _, e := ParseVersion(version); e != nil {
		return "", e
	}
	dir, e := filepath.Abs(directory)
	if e != nil {
		return "", e
	}
	if e = os.MkdirAll(dir, 0755); e != nil {
		return "", e
	}
	unlock, e := lock(dir)
	if e != nil {
		return "", e
	}
	defer unlock()
	path := filepath.Join(dir, "note")
	var old *receipt
	if _, e = os.Lstat(path); e == nil {
		r, e := managed(path)
		if e != nil {
			return "", e
		}
		if Newer(r.Version, version) {
			return "", fmt.Errorf("refusing to downgrade %s to %s", r.Version, version)
		}
		old = &r
	} else if !os.IsNotExist(e) {
		return "", e
	}
	if _, e := os.Lstat(receiptPath(path)); e == nil && old == nil {
		return "", fmt.Errorf("orphaned installation receipt; remove it after checking the destination")
	} else if e != nil && !os.IsNotExist(e) {
		return "", e
	}
	b, e := regularBytes(source, maxBinary)
	if e != nil {
		return "", e
	}
	if e = replace(ctx, path, version, b, old, false); e != nil {
		return "", e
	}
	return path, nil
}

func (c Client) SelfUpdate(ctx context.Context, executable, current, latest string) error {
	if !Newer(latest, current) {
		return fmt.Errorf("target version must be newer than the installed version")
	}
	path, e := filepath.Abs(executable)
	if e != nil {
		return e
	}
	unlock, e := lock(filepath.Dir(path))
	if e != nil {
		return e
	}
	defer unlock()
	r, e := managed(path)
	if e != nil {
		return e
	}
	if r.Version != current {
		return fmt.Errorf("running executable no longer matches the installed version; retry")
	}
	b, e := c.Binary(ctx, latest)
	if e != nil {
		return e
	}
	return replace(ctx, path, latest, b, &r, true)
}
