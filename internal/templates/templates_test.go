package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplates(t *testing.T) {
	dir := t.TempDir()
	values := map[string]string{"title": "ECFP 日本語 {{slug}}", "slug": "ecfp", "path": "chem/ecfp.md", "date": "2026-10-12", "citekey": "rogers2010", "authors": "David Rogers", "year": "2010", "doi": "10.1/x", "url": "https://example.org"}
	list, e := List(dir)
	if e != nil || len(list) != 8 {
		t.Fatal(list, e)
	}
	for _, i := range list {
		src, e := Load(dir, i.Name)
		if e != nil {
			t.Fatal(e)
		}
		got, e := Render(src, values)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(got, "---") {
			t.Fatal("frontmatter")
		}
		if i.Name == "paper" && !strings.Contains(got, "- Key: rogers2010\n- Authors: David Rogers\n- Year: 2010\n- DOI: 10.1/x\n") {
			t.Fatalf("paper: %q", got)
		}
		if i.Name == "empty" && got != "" {
			t.Fatalf("empty template: %q", got)
		}
		if dated := i.Name == "experiment" || i.Name == "meeting" || i.Name == "idea"; dated != strings.Contains(got, "Date: 2026-10-12\n") {
			t.Fatalf("%s date: %q", i.Name, got)
		}
	}
	src := `\{{title}} $PATH $E = mc^2$ {{title}} {{slug}} {{path}} {{date}}`
	got, e := Render(src, values)
	if e != nil {
		t.Fatal(e)
	}
	want := `{{title}} $PATH $E = mc^2$ ECFP 日本語 {{slug}} ecfp chem/ecfp.md 2026-10-12`
	if got != want {
		t.Fatalf("%q != %q", got, want)
	}
	for _, s := range []string{"{{bad}}", "{{ title }}", "{{Date}}", "{{title", "}}", "{{}}"} {
		if _, e = Render(s, values); e == nil {
			t.Fatal("accepted malformed", s)
		}
	}
	os.WriteFile(filepath.Join(dir, "default.md"), []byte("custom {{title}}"), 0600)
	list, e = List(dir)
	if e != nil {
		t.Fatal(e)
	}
	for _, i := range list {
		if i.Name == "default" && !i.Overrides {
			t.Fatal(i)
		}
	}
	os.Symlink(filepath.Join(dir, "default.md"), filepath.Join(dir, "link.md"))
	if _, e = Load(dir, "link"); e == nil {
		t.Fatal("template symlink")
	}
	for _, n := range []string{"../default", "/default", "no-such"} {
		if _, e = Load(dir, n); e == nil {
			t.Fatal(n)
		}
	}
}
