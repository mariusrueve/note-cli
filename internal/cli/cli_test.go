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

	"github.com/mariusrueve/note-cli/internal/errs"
)

// A Go test process is the editor helper; its arguments remain ordinary argv.
func TestProcessHelper(t *testing.T) {
	if os.Getenv("NOTE_EDITOR_HELPER") == "" {
		return
	}
	i := 0
	for i < len(os.Args) && os.Args[i] != "--" {
		i++
	}
	data, _ := json.Marshal(os.Args[i+1:])
	if e := os.WriteFile(os.Getenv("NOTE_EDITOR_RECORD"), data, 0600); e != nil {
		os.Exit(7)
	}
	if os.Getenv("NOTE_EDITOR_FAIL") != "" {
		os.Exit(7)
	}
	os.Exit(0)
}

type fixture struct{ root, cfg, record, editor string }

func setup(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "kb")
	os.Mkdir(root, 0755)
	root, _ = filepath.EvalSymlinks(root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	record := filepath.Join(dir, "args.json")
	exe, _ := os.Executable()
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	editor := filepath.Join(dir, "editor")
	if e = os.WriteFile(editor, b, 0755); e != nil {
		t.Fatal(e)
	}
	t.Setenv("NOTE_EDITOR_HELPER", "1")
	t.Setenv("NOTE_EDITOR_RECORD", record)
	cfg := filepath.Join(dir, "config.toml")
	data := fmtConfig(root, editor)
	if e = os.WriteFile(cfg, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
	return fixture{root, cfg, record, editor}
}
func fmtConfig(root, editor string) string {
	b, _ := json.Marshal(root)
	e, _ := json.Marshal(editor)
	return "root=" + string(b) + "\neditor=[" + string(e) + ",\"-test.run=TestProcessHelper\",\"--\"]\n"
}
func execute(t *testing.T, f fixture, interactive bool, input string, args ...string) (int, string, string) {
	t.Helper()
	var out, diagnostic bytes.Buffer
	c := New(Options{In: strings.NewReader(input), Out: &out, Err: &diagnostic, Interactive: func() bool { return interactive }})
	c.SetArgs(append([]string{"--config", f.cfg}, args...))
	e := c.ExecuteContext(context.Background())
	if e != nil {
		diagnostic.WriteString(e.Error())
	}
	return errs.Code(e), out.String(), diagnostic.String()
}
func TestRoutingWithoutConfig(t *testing.T) {
	f := fixture{cfg: filepath.Join(t.TempDir(), "missing")}
	for _, args := range [][]string{{"--help"}, {"--version"}, {"completion", "bash"}, {"completion", "zsh"}, {"completion", "fish"}, {"new", "--help"}} {
		code, out, _ := execute(t, f, false, "", args...)
		if code != 0 || out == "" {
			t.Fatal(args, code, out)
		}
	}
	for _, args := range [][]string{{"a", "b"}, {"--wrong"}, {"find", ""}, {"grep", "one\ntwo"}, {"recent", "--limit=0"}, {"completion", "powershell"}, {"new", "a", "--no-open", "--open-existing"}} {
		code, _, _ := execute(t, f, false, "", args...)
		if code != 2 {
			t.Fatal(args, code)
		}
	}
}
func TestCreationAndExisting(t *testing.T) {
	f := setup(t)
	code, out, _ := execute(t, f, false, "", "new", "Docker/Network_Config.md", "--no-open")
	if code != 0 || out != "" {
		t.Fatal(code, out)
	}
	p := filepath.Join(f.root, "Docker", "network-config.md")
	before, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	if string(before) != "# Network Config\n\n" {
		t.Fatalf("contents %q", before)
	}
	for _, args := range [][]string{{"new", "Docker/Network_Config.md"}, {"new", "Docker/Network_Config.md", "--no-open"}, {"new", "Docker/Network_Config.md", "--template=missing"}} {
		code, _, _ := execute(t, f, false, "", args...)
		if code != 4 {
			t.Fatal(args, code)
		}
	}
	code, _, notice := execute(t, f, true, "wrong\n\n", "new", "Docker/Network_Config.md", "--template=missing", "--title=changed")
	if code != 0 || !strings.Contains(notice, "already exists") {
		t.Fatal(code, notice)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("existing note modified")
	}
	argvData, _ := os.ReadFile(f.record)
	var argv []string
	json.Unmarshal(argvData, &argv)
	if !reflect.DeepEqual(argv, []string{p}) {
		t.Fatal(argv)
	}
	code, _, _ = execute(t, f, true, "", "new", "Docker/Network_Config.md")
	if code != 130 {
		t.Fatal(code)
	}
	code, _, _ = execute(t, f, false, "", "new", "Docker/Network_Config.md", "--open-existing")
	if code != 0 {
		t.Fatal(code)
	}
	code, _, _ = execute(t, f, false, "", "new", "empty", "--template=empty", "--no-open")
	if code != 0 {
		t.Fatal(code)
	}
	b, _ := os.ReadFile(filepath.Join(f.root, "empty.md"))
	if len(b) != 0 {
		t.Fatal(string(b))
	}
	code, _, _ = execute(t, f, false, "", "new", "bad-template", "--template=missing", "--no-open")
	if code != 3 {
		t.Fatal(code)
	}
	if _, e = os.Stat(filepath.Join(f.root, "bad-template.md")); !os.IsNotExist(e) {
		t.Fatal(e)
	}
	code, _, _ = execute(t, f, false, "", "new", "invalid-title", "--title=\xff", "--no-open")
	if code != 2 {
		t.Fatal(code)
	}
	if _, e = os.Stat(filepath.Join(f.root, "invalid-title.md")); !os.IsNotExist(e) {
		t.Fatal(e)
	}
	t.Setenv("NOTE_EDITOR_FAIL", "1")
	code, _, _ = execute(t, f, false, "", "new", "kept", "--title=日本語")
	if code != 3 {
		t.Fatal(code)
	}
	b, e = os.ReadFile(filepath.Join(f.root, "kept.md"))
	if e != nil || string(b) != "# 日本語\n\n" {
		t.Fatal(string(b), e)
	}
}
func TestLookupCompletionAndJSON(t *testing.T) {
	f := setup(t)
	os.Mkdir(filepath.Join(f.root, "folder"), 0755)
	os.WriteFile(filepath.Join(f.root, "Legacy '$(no)`x`.md"), []byte("hello"), 0600)
	code, _, _ := execute(t, f, false, "", "Legacy '$(no)`x`")
	if code != 0 {
		t.Fatal("root query routing", code)
	}
	code, out, _ := execute(t, f, false, "", "find", "absent", "--json")
	if code != 1 || out != "[]\n" {
		t.Fatal(code, out)
	}
	code, out, _ = execute(t, f, false, "", "__complete", "new", "fol")
	if code != 0 || !strings.Contains(out, "folder/") {
		t.Fatal(code, out)
	}
	code, _, _ = execute(t, f, true, "no\n", "new note lookup")
	if code != 130 {
		t.Fatal(code)
	}
	if _, e := os.Stat(filepath.Join(f.root, "new-note-lookup.md")); !os.IsNotExist(e) {
		t.Fatal(e)
	}
	code, _, notice := execute(t, f, true, "yes\n", "new note lookup")
	if code != 0 {
		t.Fatal(code, notice)
	}
	code, _, _ = execute(t, f, false, "", "search", "hello", "--json", "--print")
	if code != 2 {
		t.Fatal(code)
	}
	code, _, _ = execute(t, f, false, "", "open", "../escape")
	if code != 2 {
		t.Fatal(code)
	}
}
