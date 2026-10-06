package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mariusrueve/note-cli/internal/errs"
	"github.com/mariusrueve/note-cli/internal/update"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (f updateTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func updateFixture(calls *int) *update.Client {
	name, _ := update.ArchiveName("0.2.0", runtime.GOOS, runtime.GOARCH)
	metadata, _ := json.Marshal(map[string]any{"tag_name": "v0.2.0", "assets": []map[string]string{{"name": name}, {"name": "checksums.txt"}}})
	return &update.Client{HTTP: &http.Client{Transport: updateTransport(func(r *http.Request) (*http.Response, error) {
		(*calls)++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(metadata)), Header: make(http.Header)}, nil
	})}}
}

func TestUpdateCommandsWithoutConfig(t *testing.T) {
	for _, tt := range []struct {
		name, distribution, version string
		args                        []string
		code, calls                 int
	}{
		{"source check", "source", "dev", []string{"self-update", "--check"}, 0, 1},
		{"archive check", "archive", "0.1.0", []string{"self-update", "--check"}, 0, 1},
		{"source replacement refused", "source", "dev", []string{"self-update"}, 3, 0},
		{"snapshot replacement refused", "archive", "snapshot", []string{"self-update"}, 3, 0},
		{"current version", "archive", "0.2.0", []string{"self-update"}, 0, 1},
		{"extra argument", "archive", "0.1.0", []string{"self-update", "extra"}, 2, 0},
		{"missing directory", "archive", "0.1.0", []string{"__install"}, 2, 0},
		{"source installation refused", "source", "0.1.0", []string{"__install", "--directory", t.TempDir()}, 3, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, diagnostics bytes.Buffer
			calls := 0
			c := New(Options{Out: &out, Err: &diagnostics, Version: tt.version, Distribution: tt.distribution, UpdateClient: updateFixture(&calls), Interactive: func() bool { return true }})
			c.SetArgs(append([]string{"--config", filepath.Join(t.TempDir(), "missing")}, tt.args...))
			e := c.ExecuteContext(context.Background())
			if errs.Code(e) != tt.code || calls != tt.calls || out.Len() != 0 {
				t.Fatal(e, calls, out.String(), diagnostics.String())
			}
			if tt.code == 0 && !strings.Contains(diagnostics.String(), "0.2.0") {
				t.Fatal(diagnostics.String())
			}
		})
	}
}

func TestAutomaticUpdatePolicy(t *testing.T) {
	f := setup(t)
	if e := os.WriteFile(filepath.Join(f.root, "example.md"), []byte("example"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, tt := range []struct {
		name                          string
		interactive                   bool
		distribution, version, optout string
		args                          []string
		calls                         int
	}{
		{"interactive", true, "archive", "0.1.0", "", []string{"templates"}, 1},
		{"headless", false, "archive", "0.1.0", "", []string{"templates"}, 0},
		{"source", true, "source", "0.1.0", "", []string{"templates"}, 0},
		{"development", true, "archive", "dev", "", []string{"templates"}, 0},
		{"optout", true, "archive", "0.1.0", "1", []string{"templates"}, 0},
		{"json", true, "archive", "0.1.0", "", []string{"find", "example", "--json"}, 0},
		{"print", true, "archive", "0.1.0", "", []string{"find", "example", "--print"}, 0},
		{"completion script", true, "archive", "0.1.0", "", []string{"completion", "zsh"}, 0},
		{"completion callback", true, "archive", "0.1.0", "", []string{"__complete", "new", "ex"}, 0},
		{"help", true, "archive", "0.1.0", "", []string{"--help"}, 0},
		{"version", true, "archive", "0.1.0", "", []string{"--version"}, 0},
		{"failed command", true, "archive", "0.1.0", "", []string{"open", "absent"}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			t.Setenv("NOTE_NO_UPDATE_CHECK", tt.optout)
			var out, diagnostics bytes.Buffer
			calls := 0
			c := New(Options{Out: &out, Err: &diagnostics, In: strings.NewReader(""), Version: tt.version, Distribution: tt.distribution, UpdateClient: updateFixture(&calls), Interactive: func() bool { return tt.interactive }})
			c.SetArgs(append([]string{"--config", f.cfg}, tt.args...))
			_ = c.ExecuteContext(context.Background())
			if calls != tt.calls {
				t.Fatal("unexpected update request", calls)
			}
			hint := strings.Contains(diagnostics.String(), "run note self-update")
			if hint != (tt.calls == 1) {
				t.Fatal(diagnostics.String())
			}
			if strings.Contains(out.String(), "0.2.0 is available") {
				t.Fatal("update hint polluted stdout")
			}
		})
	}
}
