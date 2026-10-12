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
	"time"

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
func TestResearchTemplateDate(t *testing.T) {
	f := setup(t)
	var out, diagnostic bytes.Buffer
	c := New(Options{Out: &out, Err: &diagnostic, Interactive: func() bool { return false }, Now: func() time.Time { return time.Date(2026, 1, 2, 23, 30, 0, 0, time.Local) }})
	c.SetArgs([]string{"--config", f.cfg, "new", "lab/binding-assay", "--template", "experiment", "--no-open"})
	if e := c.ExecuteContext(context.Background()); e != nil {
		t.Fatal(e, diagnostic.String())
	}
	b, _ := os.ReadFile(filepath.Join(f.root, "lab", "binding-assay.md"))
	if !strings.HasPrefix(string(b), "# Binding assay\n\nDate: 2026-01-02\n\n## Question\n") {
		t.Fatalf("contents %q", b)
	}
	code, list, _ := execute(t, f, false, "", "templates")
	for _, name := range []string{"experiment", "idea", "meeting", "paper"} {
		if code != 0 || !strings.Contains(list, name) {
			t.Fatal(code, name, list)
		}
	}
}
func TestScopedViews(t *testing.T) {
	f := setup(t)
	for _, p := range []string{"phd/lab/assay.md", "phd/ideas.md", "docker/assay.md", "phdx/assay.md"} {
		os.MkdirAll(filepath.Join(f.root, filepath.Dir(p)), 0755)
		os.WriteFile(filepath.Join(f.root, p), []byte("binding assay\n"), 0600)
	}
	os.WriteFile(filepath.Join(f.root, "phd", "ideas.md"), []byte("open questions\n"), 0600)
	for _, kind := range []string{"find", "grep", "search"} {
		code, out, _ := execute(t, f, false, "", kind, "assay", "--in", "phd/", "--json")
		var results []struct{ Path string }
		json.Unmarshal([]byte(out), &results)
		if code != 0 || len(results) != 1 || results[0].Path != "phd/lab/assay.md" {
			t.Fatal(kind, code, out)
		}
	}
	code, out, _ := execute(t, f, false, "", "recent", "--in", "phd", "--print")
	if code != 0 || strings.Count(out, "\n") != 2 || strings.Contains(out, "docker") || strings.Contains(out, "phdx") {
		t.Fatal(code, out)
	}
	code, out, _ = execute(t, f, false, "", "find", "assay", "--in", "phd/lab", "--json", "--case-sensitive")
	if code != 0 || !strings.Contains(out, `"phd/lab/assay.md"`) {
		t.Fatal(code, out)
	}
	for args, want := range map[string]int{"find assay --in missing": 1, "find assay --in docker/assay.md": 1, "find assay --in ../x": 2, "recent --in /abs": 2, "open phd/ideas --in phd": 2, "find ideas --in docker --print": 1} {
		if code, _, _ := execute(t, f, false, "", strings.Fields(args)...); code != want {
			t.Fatal(args, code, want)
		}
	}
	// The shortcut considers only scoped notes; a root-level exact path outside the scope is not opened.
	code, _, _ = execute(t, f, false, "", "--in", "phd", "assay")
	if code != 0 {
		t.Fatal(code)
	}
	argvData, _ := os.ReadFile(f.record)
	if !strings.Contains(string(argvData), filepath.Join("phd", "lab", "assay.md")) {
		t.Fatal(string(argvData))
	}
	if code, _, _ = execute(t, f, false, "", "--in", "phd", "docker/assay"); code != 1 {
		t.Fatal(code)
	}
	code, _, notice := execute(t, f, true, "y\n", "--in", "phd", "new-question")
	if code != 0 {
		t.Fatal(code, notice)
	}
	if _, e := os.Stat(filepath.Join(f.root, "phd", "new-question.md")); e != nil {
		t.Fatal(e)
	}
	code, out, _ = execute(t, f, false, "", "__complete", "find", "assay", "--in", "ph")
	if code != 0 || !strings.Contains(out, "phd/") || strings.Contains(out, ".md") {
		t.Fatal(code, out)
	}
}
func TestInboxDefault(t *testing.T) {
	f := setup(t)
	clock := time.Date(2026, 3, 4, 9, 5, 6, 0, time.Local)
	run := func(input string, args ...string) (int, string) {
		var out, diagnostic bytes.Buffer
		c := New(Options{In: strings.NewReader(input), Out: &out, Err: &diagnostic, Interactive: func() bool { return false }, Now: func() time.Time { return clock }})
		c.SetArgs(append([]string{"--config", f.cfg, "new"}, args...))
		e := c.ExecuteContext(context.Background())
		return errs.Code(e), diagnostic.String()
	}
	read := func(rel string) string {
		b, e := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(rel)))
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	if code, d := run("", "--no-open"); code != 0 || !strings.Contains(d, "Created inbox/2026-03-04-090506.md") {
		t.Fatal(code, d)
	}
	if got := read("inbox/2026-03-04-090506.md"); got != "# 2026-03-04 09:05:06\n\n" {
		t.Fatalf("%q", got)
	}
	// The same second never overwrites; it follows the existing-note rules.
	if code, _ := run("", "--no-open"); code != 4 {
		t.Fatal(code)
	}
	if code, _ := run("", "--title", "TCP/IP Basics", "--template", "idea", "--no-open"); code != 0 {
		t.Fatal(code)
	}
	if got := read("inbox/tcp-ip-basics.md"); !strings.HasPrefix(got, "# TCP/IP Basics\n\nDate: 2026-03-04\n") {
		t.Fatalf("%q", got)
	}
	clock = clock.Add(time.Second)
	if code, _ := run("", "--title", "日本語", "--no-open"); code != 0 {
		t.Fatal(code)
	}
	if got := read("inbox/2026-03-04-090507.md"); got != "# 日本語\n\n" {
		t.Fatalf("%q", got)
	}
	// The editor opens new inbox notes like any other new note.
	if code, d := run("", "--title", "Quick thought"); code != 0 {
		t.Fatal(code, d)
	}
	argvData, _ := os.ReadFile(f.record)
	if !strings.Contains(string(argvData), filepath.Join("inbox", "quick-thought.md")) {
		t.Fatal(string(argvData))
	}
	os.WriteFile(f.cfg, []byte(fmtConfig(f.root, f.editor)+"inbox_dir = \"phd/capture/\"\n"), 0600)
	if code, _ := run("", "--title", "Idea", "--no-open"); code != 0 {
		t.Fatal(code)
	}
	read("phd/capture/idea.md")
	os.Symlink(t.TempDir(), filepath.Join(f.root, "linked"))
	os.WriteFile(f.cfg, []byte(fmtConfig(f.root, f.editor)+"inbox_dir = \"linked\"\n"), 0600)
	if code, _ := run("", "--no-open"); code == 0 {
		t.Fatal("created through a symlinked inbox")
	}
}
func TestBacklinksAndDeadlinks(t *testing.T) {
	f := setup(t)
	for p, s := range map[string]string{"chem/ecfp.md": "# ECFP\n", "index.md": "[ECFP](chem/ecfp.md)\n[gone](missing.md)\n", "phd/reading.md": "x\n[fp](../chem/ecfp.md)\n"} {
		os.MkdirAll(filepath.Join(f.root, filepath.Dir(p)), 0755)
		os.WriteFile(filepath.Join(f.root, p), []byte(s), 0600)
	}
	code, out, _ := execute(t, f, false, "", "backlinks", "chem/ecfp")
	if code != 0 || out != "index.md:1:[ECFP](chem/ecfp.md)\nphd/reading.md:2:[fp](../chem/ecfp.md)\n" {
		t.Fatalf("%d %q", code, out)
	}
	code, out, _ = execute(t, f, false, "", "backlinks", "chem/ecfp.md", "--in", "phd", "--json")
	if code != 0 || out != `[{"path":"phd/reading.md","match_kinds":["link"],"line":2,"snippet":"[fp](../chem/ecfp.md)"}]`+"\n" {
		t.Fatalf("%d %q", code, out)
	}
	for args, want := range map[string]int{"backlinks index": 1, "backlinks missing": 1, "backlinks ../x": 2, "backlinks a --print --json": 2, "deadlinks --in missing": 1} {
		if code, _, _ := execute(t, f, false, "", strings.Fields(args)...); code != want {
			t.Fatal(args, code, want)
		}
	}
	code, out, _ = execute(t, f, false, "", "deadlinks")
	if code != 1 || out != "index.md:2:missing.md (missing)\n" {
		t.Fatalf("%d %q", code, out)
	}
	code, out, _ = execute(t, f, false, "", "deadlinks", "--json")
	if code != 1 || !strings.Contains(out, `"snippet":"missing.md","problem":"missing"`) {
		t.Fatalf("%d %q", code, out)
	}
	code, out, notice := execute(t, f, false, "", "deadlinks", "--in", "phd")
	if code != 0 || out != "" || !strings.Contains(notice, "No broken links.") {
		t.Fatalf("%d %q %q", code, out, notice)
	}
	code, out, _ = execute(t, f, false, "", "deadlinks", "--in", "phd", "--json")
	if code != 0 || out != "[]\n" {
		t.Fatalf("%d %q", code, out)
	}
}
func TestCitations(t *testing.T) {
	f := setup(t)
	bibFile := filepath.Join(t.TempDir(), "My Library.bib")
	source := "@article{rogersExtendedConnectivity2010,\n  title = {Extended-{{Connectivity Fingerprints}}},\n  author = {Rogers, David and Hahn, Mathew},\n  date = {2010-05-24},\n  doi = {10.1021/ci100050t},\n  url = {https://example.org/ecfp}\n}\n@online{noAscii, title = {日本語}}\n"
	os.WriteFile(bibFile, []byte(source), 0400)
	// Without a configured bibliography, citing is a configuration error.
	if code, _, _ := execute(t, f, false, "", "cite", "rogersExtendedConnectivity2010"); code != 3 {
		t.Fatal(code)
	}
	b, _ := json.Marshal(bibFile)
	os.WriteFile(f.cfg, []byte(fmtConfig(f.root, f.editor)+"bibliography = "+string(b)+"\n"), 0600)
	code, _, notice := execute(t, f, false, "", "cite", "rogersExtendedConnectivity2010", "--no-open")
	if code != 0 || !strings.Contains(notice, "Created literature/rogersextendedconnectivity2010.md") {
		t.Fatal(code, notice)
	}
	p := filepath.Join(f.root, "literature", "rogersextendedconnectivity2010.md")
	got, _ := os.ReadFile(p)
	want := "# Extended-Connectivity Fingerprints\n\n## Citation\n\n- Key: rogersExtendedConnectivity2010\n- Authors: David Rogers, Mathew Hahn\n- Year: 2010\n- DOI: 10.1021/ci100050t\n- URL: https://example.org/ecfp\n\n## Summary\n"
	if !strings.HasPrefix(string(got), want) {
		t.Fatalf("%q", got)
	}
	// An existing literature note opens without a prompt and is never rewritten.
	os.WriteFile(p, []byte("my notes\n"), 0600)
	if code, _, _ = execute(t, f, false, "", "cite", "rogersExtendedConnectivity2010"); code != 0 {
		t.Fatal(code)
	}
	if got, _ = os.ReadFile(p); string(got) != "my notes\n" {
		t.Fatalf("%q", got)
	}
	argvData, _ := os.ReadFile(f.record)
	if !strings.Contains(string(argvData), p) {
		t.Fatal(string(argvData))
	}
	if code, _, _ = execute(t, f, false, "", "cite", "rogersExtendedConnectivity2010", "--no-open"); code != 4 {
		t.Fatal(code)
	}
	// note new --cite fills any template at any path; --title still wins.
	code, _, _ = execute(t, f, false, "", "new", "phd/reading/ecfp", "--cite", "rogersExtendedConnectivity2010", "--template", "default", "--title", "ECFP", "--no-open")
	if got, _ = os.ReadFile(filepath.Join(f.root, "phd", "reading", "ecfp.md")); code != 0 || string(got) != "# ECFP\n\n" {
		t.Fatalf("%d %q", code, got)
	}
	// Without a citation, citation placeholders render empty.
	execute(t, f, false, "", "new", "blank-paper", "--template", "paper", "--no-open")
	if got, _ = os.ReadFile(filepath.Join(f.root, "blank-paper.md")); !strings.Contains(string(got), "- Key: \n- Authors: \n") {
		t.Fatalf("%q", got)
	}
	for args, want := range map[string]int{"cite missingKey": 1, "cite": 2, "cite bad\tkey": 2, "new --cite missingKey --no-open": 1, "cite rogersExtendedConnectivity2010 --template=../x": 2} {
		if code, _, _ := execute(t, f, false, "", strings.Split(args, " ")...); code != want {
			t.Fatal(args, code, want)
		}
	}
	// Unicode titles are kept in the heading; the basename comes from the key.
	execute(t, f, false, "", "cite", "noAscii", "--no-open")
	if got, _ = os.ReadFile(filepath.Join(f.root, "literature", "noascii.md")); !strings.HasPrefix(string(got), "# 日本語\n") {
		t.Fatalf("%q", got)
	}
	code, out, _ := execute(t, f, false, "", "__complete", "cite", "rogers")
	if code != 0 || !strings.Contains(out, "rogersExtendedConnectivity2010") || strings.Contains(out, "noAscii") {
		t.Fatal(code, out)
	}
	if after, _ := os.ReadFile(bibFile); string(after) != source {
		t.Fatal("bibliography modified")
	}
}
