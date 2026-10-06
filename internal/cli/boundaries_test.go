package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mariusrueve/note-cli/internal/config"
	"github.com/mariusrueve/note-cli/internal/errs"
	"github.com/mariusrueve/note-cli/internal/search"
	"github.com/spf13/cobra"
)

func recorded(t *testing.T, f fixture) []string {
	t.Helper()
	b, e := os.ReadFile(f.record)
	if e != nil {
		t.Fatal(e)
	}
	var args []string
	if e = json.Unmarshal(b, &args); e != nil {
		t.Fatal(e)
	}
	return args
}
func TestEditorPositionsAndMissingExecutable(t *testing.T) {
	f := setup(t)
	p := filepath.Join(f.root, "hit.md")
	os.WriteFile(p, []byte("later hit"), 0600)
	hx := filepath.Join(filepath.Dir(f.editor), "hx")
	b, _ := os.ReadFile(f.editor)
	os.WriteFile(hx, b, 0755)
	a := &app{Options: Options{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}}
	line := 42
	c := config.Config{Root: f.root, Editor: []string{hx, "-test.run=TestProcessHelper", "--"}}
	if e := a.edit(context.Background(), c, search.Result{Path: "hit.md", Line: &line}); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(recorded(t, f), []string{p + ":42"}) {
		t.Fatal(recorded(t, f))
	}
	os.WriteFile(filepath.Join(f.root, "colon:hit.md"), nil, 0600)
	if e := a.edit(context.Background(), c, search.Result{Path: "colon:hit.md", Line: &line}); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(recorded(t, f), []string{filepath.Join(f.root, "colon:hit.md")}) {
		t.Fatal(recorded(t, f))
	}
	c.Editor = []string{"/missing-editor"}
	if e := a.create(context.Background(), c, "not-created", "default", "", false, false, false); errs.Code(e) != 3 {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(f.root, "not-created.md")); !os.IsNotExist(e) {
		t.Fatal(e)
	}
	if e := a.create(context.Background(), c, "no-tools", "default", "", false, true, false); e != nil {
		t.Fatal(e)
	}
}
func TestCompletionFallbacks(t *testing.T) {
	f := setup(t)
	os.MkdirAll(filepath.Join(f.root, "ordinary", "empty"), 0755)
	os.Mkdir(filepath.Join(f.root, ".hidden"), 0755)
	os.MkdirAll(filepath.Join(f.root, "nested", ".git"), 0755)
	os.Symlink(filepath.Join(f.root, "ordinary"), filepath.Join(f.root, "link"))
	os.WriteFile(filepath.Join(f.root, "ordinary.md"), nil, 0600)
	os.WriteFile(filepath.Join(f.root, "ordinary\n:0.md"), nil, 0600)
	os.Mkdir(filepath.Join(f.root, "ordinary\tunsafe"), 0755)
	withMissingRG := fmtConfig(f.root, f.editor) + "[tools]\nrg=[\"/missing-rg\"]\n"
	os.WriteFile(f.cfg, []byte(withMissingRG), 0600)
	a := &app{configPath: f.cfg}
	cmd := New(Options{})
	cmd.SetContext(context.Background())
	v, d := a.completePaths(cmd, nil, "")
	if !reflect.DeepEqual(v, []string{"ordinary/"}) || d != cobra.ShellCompDirectiveNoFileComp|cobra.ShellCompDirectiveNoSpace {
		t.Fatal(v, d)
	}
	v, d = a.completePaths(cmd, nil, "ordinary/")
	if !reflect.DeepEqual(v, []string{"ordinary/empty/"}) {
		t.Fatal(v, d)
	}
	v, _ = a.completePaths(cmd, nil, ".hidden/")
	if len(v) > 0 {
		t.Fatal(v)
	}
	v, _ = a.completeTemplates(cmd, nil, "")
	if len(v) != 4 {
		t.Fatal(v)
	}
	a.configPath = "/missing-config"
	v, _ = a.completePaths(cmd, nil, "")
	if len(v) > 0 {
		t.Fatal(v)
	}
	v, _ = a.completeTemplates(cmd, nil, "")
	if len(v) != 4 {
		t.Fatal(v)
	}
	os.WriteFile(f.cfg, []byte(fmtConfig(f.root, f.editor)), 0600)
	a.configPath = f.cfg
	v, d = a.completePaths(cmd, nil, "ord")
	if !reflect.DeepEqual(v, []string{"ordinary.md", "ordinary/"}) || d != 6 {
		t.Fatal(v, d)
	}
}
func TestExplicitIgnoredOpenAndHeadlessPicker(t *testing.T) {
	f := setup(t)
	os.WriteFile(filepath.Join(f.root, ".ignore"), []byte("ignored.md\n"), 0600)
	os.WriteFile(filepath.Join(f.root, "ignored.md"), []byte("preserved"), 0600)
	code, _, _ := execute(t, f, false, "", "open", "ignored")
	if code != 0 {
		t.Fatal(code)
	}
	if !reflect.DeepEqual(recorded(t, f), []string{filepath.Join(f.root, "ignored.md")}) {
		t.Fatal(recorded(t, f))
	}
	code, out, _ := execute(t, f, false, "", "find", "ignored", "--json")
	if code != 1 || out != "[]\n" {
		t.Fatal(code, out)
	}
	code, _, _ = execute(t, f, false, "")
	if code != 2 {
		t.Fatal(code)
	}
	code, _, _ = execute(t, f, false, "", "new", "invalid", "--template=")
	if code != 2 {
		t.Fatal(code)
	}
}
