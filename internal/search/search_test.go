package search

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mariusrueve/note-cli/internal/config"
)

func fixture(t *testing.T) config.Config {
	t.Helper()
	if _, e := exec.LookPath("rg"); e != nil {
		t.Fatal("integration tests require ripgrep:", e)
	}
	root := t.TempDir()
	c := config.Config{Root: root, Tools: config.Tools{RG: []string{"rg"}}}
	cmd := exec.Command("git", "init", root)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	for p, s := range map[string]string{"needle-path.md": "ordinary", "needle-both.md": "needle\nneedle twice needle\n", "content.md": "first\nneedle\nÜberblick [a.*] $(touch marker)\n", "neither.md": "none", "UPPER.MD": "NEEDLE", "ignored.md": "needle", ".hidden.md": "needle", "nested/repo.md": "needle", "ignore-file.md": "needle", "rg-ignore.md": "needle"} {
		p = filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte(s), 0600)
	}
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.md\n"), 0600)
	os.WriteFile(filepath.Join(root, ".ignore"), []byte("ignore-file.md\n"), 0600)
	os.WriteFile(filepath.Join(root, ".rgignore"), []byte("rg-ignore.md\n"), 0600)
	os.Mkdir(filepath.Join(root, "nested", ".git"), 0755)
	os.Symlink(filepath.Join(root, "content.md"), filepath.Join(root, "linked.md"))
	return c
}
func paths(r []Result) []string {
	out := []string{}
	for _, i := range r {
		out = append(out, i.Path)
	}
	return out
}
func TestDistinctScopesAndDiscovery(t *testing.T) {
	c := fixture(t)
	all, e := Discover(context.Background(), c, io.Discard)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(paths(all), []string{"UPPER.MD", "content.md", "needle-both.md", "needle-path.md", "neither.md"}) {
		t.Fatal(paths(all))
	}
	p := Find(all, "needle", false)
	content, e := Grep(context.Background(), c, all, "needle", false)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(paths(p), []string{"needle-both.md", "needle-path.md"}) {
		t.Fatal(paths(p))
	}
	if !reflect.DeepEqual(paths(content), []string{"UPPER.MD", "content.md", "needle-both.md", "needle-both.md"}) {
		t.Fatal(paths(content))
	}
	u := Union(p, content)
	if !reflect.DeepEqual(paths(u), []string{"needle-both.md", "needle-path.md", "UPPER.MD", "content.md"}) {
		t.Fatal(paths(u))
	}
	if *u[0].Line != 1 || !reflect.DeepEqual(u[0].MatchKinds, []string{"path", "content"}) {
		t.Fatal(u[0])
	}
	for _, q := range []string{"[a.*]", "$(touch marker)", "überblick"} {
		r, e := Grep(context.Background(), c, all, q, false)
		if e != nil || len(r) != 1 || *r[0].Line != 3 {
			t.Fatal(q, r, e)
		}
	}
	r, e := Grep(context.Background(), c, all, "überblick", true)
	if e != nil || len(r) != 0 {
		t.Fatal(r, e)
	}
	if _, e = os.Stat(filepath.Join(c.Root, "marker")); !os.IsNotExist(e) {
		t.Fatal("query executed")
	}
	if r, e = Grep(context.Background(), c, nil, "needle", false); e != nil || len(r) != 0 {
		t.Fatal(r, e)
	}
	var out bytes.Buffer
	if e = Print(&out, "search", u, true); e != nil {
		t.Fatal(e)
	}
	var decoded []map[string]any
	if e = json.Unmarshal(out.Bytes(), &decoded); e != nil {
		t.Fatal(e)
	}
	for _, o := range decoded {
		if len(o) != 4 {
			t.Fatal(o)
		}
	}
}
func TestRecentAndControlOutput(t *testing.T) {
	c := fixture(t)
	all, e := Discover(context.Background(), c, io.Discard)
	if e != nil {
		t.Fatal(e)
	}
	stamp := time.Unix(1000, 42)
	for _, r := range all {
		os.Chtimes(filepath.Join(c.Root, r.Path), stamp, stamp)
	}
	newest := stamp.Add(time.Nanosecond)
	os.Chtimes(filepath.Join(c.Root, "content.md"), newest, newest)
	r := Recent(c.Root, all, 2, io.Discard)
	if !reflect.DeepEqual(paths(r), []string{"content.md", "UPPER.MD"}) {
		t.Fatal(paths(r))
	}
	if *r[0].MtimeNS != newest.UnixNano() {
		t.Fatal(*r[0].MtimeNS)
	}
	var b bytes.Buffer
	Print(&b, "recent", r, true)
	var fields []map[string]any
	json.Unmarshal(b.Bytes(), &fields)
	if len(fields[0]) != 5 {
		t.Fatal(fields)
	}
	if Visible("a\n\t\x1b[31m\r") != `a\n\t\u{001b}[31m\r` {
		t.Fatal(Visible("a\n\t\x1b[31m\r"))
	}
}
