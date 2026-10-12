package picker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mariusrueve/note-cli/internal/config"
	"github.com/mariusrueve/note-cli/internal/errs"
	"github.com/mariusrueve/note-cli/internal/search"
)

func TestRoundTripAndCleanup(t *testing.T) {
	if _, e := exec.LookPath("fzf"); e != nil {
		t.Fatal("integration tests require fzf:", e)
	}
	root := t.TempDir()
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("FZF_DEFAULT_OPTS", "--print-query --multi --bind=enter:abort")
	t.Setenv("FZF_DEFAULT_OPTS_FILE", "/no/such/file")
	t.Setenv("FZF_DEFAULT_COMMAND", "touch marker")
	p := "unique '$(touch marker)`x`\ncolon:Ü.md"
	os.WriteFile(filepath.Join(root, p), []byte("source"), 0600)
	c := config.Config{Root: root, Tools: config.Tools{FZF: []string{"fzf", "--filter=unique"}}, Finder: config.Finder{Height: "90%", PreviewWindow: "right:60%:wrap"}}
	r, e := Pick(context.Background(), c, []search.Result{search.Listing(p)}, "find", "/installed/note", io.Discard)
	if e != nil || r.Path != p {
		t.Fatal(r, e)
	}
	entries, _ := os.ReadDir(tmp)
	if len(entries) != 0 {
		t.Fatal("manifest leaked", entries)
	}
	if os.Getenv("FZF_DEFAULT_OPTS") == "" {
		t.Fatal("parent environment changed")
	}
	c.Tools.FZF = []string{"fzf", "--filter=absent"}
	_, e = Pick(context.Background(), c, []search.Result{search.Listing(p)}, "find", "/installed/note", io.Discard)
	if errs.Code(e) != 130 {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(root, "marker")); !os.IsNotExist(e) {
		t.Fatal("display text executed")
	}
}
func TestInstalledBinaryAndPreview(t *testing.T) {
	checkout, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin with 'quote")
	os.Mkdir(binDir, 0755)
	binary := filepath.Join(binDir, "note")
	build := exec.Command("go", "build", "-mod=readonly", "-ldflags=-X main.version=test -X main.commit=fixture", "-o", binary, "./cmd/note")
	build.Dir = checkout
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	root := filepath.Join(dir, "knowledge")
	os.Mkdir(root, 0755)
	root, _ = filepath.EvalSymlinks(root)
	cfg := filepath.Join(dir, "config.toml")
	b, _ := json.Marshal(root)
	os.WriteFile(cfg, []byte("root="+string(b)+"\n"), 0600)
	run := func(args ...string) (string, error) {
		cmd := exec.Command(binary, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH=/missing-tools")
		out, e := cmd.Output()
		return string(out), e
	}
	for _, args := range [][]string{{"--help"}, {"--version"}, {"completion", "bash"}, {"--config", cfg, "templates"}, {"--config", cfg, "new", "outside", "--no-open"}} {
		if out, e := run(args...); e != nil {
			t.Fatal(args, out, e)
		}
	}
	contents, _ := os.ReadFile(filepath.Join(root, "outside.md"))
	if string(contents) != "# Outside\n\n" {
		t.Fatalf("embedded template: %q", contents)
	}
	p := "quote' $(touch marker).md"
	os.WriteFile(filepath.Join(root, p), []byte(strings.Repeat("line\n", 150)+"hit\x1b[31m\n"), 0600)
	line := 151
	r := search.Listing(p)
	r.Line = &line
	manifest := filepath.Join(dir, "manifest 'quote.json")
	mf, _ := os.Create(manifest)
	json.NewEncoder(mf).Encode(Manifest{1, root, []search.Result{r}})
	mf.Close()
	cmd := exec.Command("/bin/sh", "-c", Quote(binary)+" __preview --manifest "+Quote(manifest)+" --id 1")
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatal(string(out), e)
	}
	if !bytes.Contains(out, []byte(`hit\u{001b}[31m`)) || !bytes.Contains(out, []byte(">   151")) {
		t.Fatal(string(out))
	}
	var preview bytes.Buffer
	if e = Preview(manifest, 2, &preview); e == nil {
		t.Fatal("invalid ID")
	}
	os.Remove(filepath.Join(root, p))
	if e = Preview(manifest, 1, &preview); e == nil {
		t.Fatal("missing file")
	}
	os.Symlink(filepath.Join(root, "outside.md"), filepath.Join(root, p))
	if e = Preview(manifest, 1, &preview); e == nil {
		t.Fatal("symlink preview")
	}
	if _, e = os.Stat(filepath.Join(dir, "marker")); !os.IsNotExist(e) {
		t.Fatal("preview path executed")
	}
}

func TestPreviewBudgetAndValidation(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	path := filepath.Join(root, "large.md")
	os.WriteFile(path, []byte(strings.Repeat("line\n", 400000)), 0600)
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	line := 400000
	r := search.Listing("large.md")
	r.Line = &line
	save := func(m Manifest) {
		f, e := os.Create(manifest)
		if e != nil {
			t.Fatal(e)
		}
		json.NewEncoder(f).Encode(m)
		f.Close()
	}
	save(Manifest{1, root, []search.Result{r}})
	var b bytes.Buffer
	if e := Preview(manifest, 1, &b); e != nil {
		t.Fatal(e)
	}
	if b.Len() > 2000 || !strings.Contains(b.String(), "truncated") || !strings.Contains(b.String(), "beyond preview read budget") {
		t.Fatal("preview not bounded", b.Len())
	}
	for _, m := range []Manifest{{2, root, []search.Result{r}}, {1, "relative", []search.Result{r}}, {1, root, []search.Result{search.Listing("../outside.md")}}, {1, root, []search.Result{search.Listing("large.txt")}}} {
		save(m)
		if e := Preview(manifest, 1, io.Discard); e == nil {
			t.Fatal("unsafe manifest accepted", m)
		}
	}
	os.WriteFile(path, []byte{'x', 0xff, '\x1b'}, 0600)
	save(Manifest{1, root, []search.Result{search.Listing("large.md")}})
	b.Reset()
	if e := Preview(manifest, 1, &b); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(b.String(), "invalid UTF-8") || strings.ContainsRune(b.String(), '\x1b') {
		t.Fatal(b.String())
	}
	raw, _ := os.ReadFile(path)
	if !bytes.Equal(raw, []byte{'x', 0xff, '\x1b'}) {
		t.Fatal("preview rewrote invalid text")
	}
}
func TestChooseWithPreviewAndPreviewText(t *testing.T) {
	if _, e := exec.LookPath("fzf"); e != nil {
		t.Fatal("integration tests require fzf:", e)
	}
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	c := config.Config{Tools: config.Tools{FZF: []string{"fzf", "--filter=second"}}, Finder: config.Finder{Height: "90%", PreviewWindow: "right:60%:wrap"}}
	n, e := ChooseWithPreview(context.Background(), c, []string{"first", "second"}, []string{"a", "b"}, "/installed/note", io.Discard)
	if e != nil || n != 1 {
		t.Fatal(n, e)
	}
	if entries, _ := os.ReadDir(tmp); len(entries) != 0 {
		t.Fatal("manifest leaked", entries)
	}
	if _, e = ChooseWithPreview(context.Background(), c, []string{"x"}, nil, "", io.Discard); errs.Code(e) != 1 {
		t.Fatal(e)
	}
	manifest := filepath.Join(t.TempDir(), "m.json")
	os.WriteFile(manifest, []byte(`{"schema_version":1,"texts":["Title\nctrl\u001b[31m","two"]}`), 0600)
	var b bytes.Buffer
	if e = PreviewText(manifest, 1, &b); e != nil || b.String() != "Title\nctrl\\u{001b}[31m\n" {
		t.Fatalf("%q %v", b.String(), e)
	}
	for _, body := range []string{`{"schema_version":2,"texts":["a"]}`, `{"schema_version":1,"texts":["a"],"x":1}`, `{"schema_version":1,"texts":["a"]} {}`, `{"schema_version":1,"texts":[]}`} {
		os.WriteFile(manifest, []byte(body), 0600)
		if e = PreviewText(manifest, 1, io.Discard); e == nil {
			t.Fatal("accepted", body)
		}
	}
	if e = PreviewText(manifest+".missing", 1, io.Discard); e == nil {
		t.Fatal("missing manifest")
	}
}
