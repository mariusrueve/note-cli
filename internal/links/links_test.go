package links

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mariusrueve/note-cli/internal/search"
)

func TestExtract(t *testing.T) {
	src := "# Title\n" +
		"See [ECFP](../chem/ecfp.md) and ![plot](img/plot%201.png \"Plot\").\n" +
		"[anchor](#local) [web](https://example.org/a.md) [mail](mailto:x@y) [proto](//cdn/x.md)\n" +
		"[frag](other.md#section) [query](other.md?x=1) [angle](<with space.md>)\n" +
		"[paren](wiki/Foo_(bar).md) [root](/top.md) [out](../../outside.md)\n" +
		"`[code](code.md)` and ``[also](code2.md)``\n" +
		"```md\n[fenced](fenced.md)\n```\n" +
		"~~~~\n[tilde](tilde.md)\n~~~~\n" +
		"   [ref]: <refs/target.md> \"Title\"\n" +
		"    [indented code ref]: notref.md\n"
	var got [][2]string
	for _, l := range Extract("notes/a.md", src) {
		got = append(got, [2]string{l.Target, l.Rel})
	}
	want := [][2]string{
		{"../chem/ecfp.md", "chem/ecfp.md"},
		{"img/plot%201.png", "notes/img/plot 1.png"},
		{"other.md#section", "notes/other.md"},
		{"other.md?x=1", "notes/other.md"},
		{"with space.md", "notes/with space.md"},
		{"wiki/Foo_(bar).md", "notes/wiki/Foo_(bar).md"},
		{"/top.md", "top.md"},
		{"../../outside.md", ""},
		{"refs/target.md", "notes/refs/target.md"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if l := Extract("a.md", "x\n[b](b.md)\r\n"); len(l) != 1 || l[0].Line != 2 || l[0].Text != "[b](b.md)" {
		t.Fatal(l)
	}
}

func TestBacklinksAndDead(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"chem/ecfp.md":   "# ECFP\n[home](../index.md)\n",
		"index.md":       "[ECFP](chem/ecfp.md)\n[case](chem/ECFP.md)\n[gone](missing.md)\n[dir](chem/)\n[out](../x.md)\n[link](linked/a.md)\n",
		"phd/reading.md": "[fp](../chem/ecfp.md#idea)\n[self](reading.md)\n",
	}
	for p, s := range files {
		os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0755)
		os.WriteFile(filepath.Join(root, p), []byte(s), 0600)
	}
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "a.md"), nil, 0600)
	os.Symlink(outside, filepath.Join(root, "linked"))
	all := []search.Result{search.Listing("chem/ecfp.md"), search.Listing("index.md"), search.Listing("phd/reading.md")}
	var paths []string
	for _, r := range Backlinks(root, all, "chem/ecfp.md", os.Stderr) {
		paths = append(paths, r.Path+":"+*r.Snippet)
	}
	if !reflect.DeepEqual(paths, []string{"index.md:[ECFP](chem/ecfp.md)", "phd/reading.md:[fp](../chem/ecfp.md#idea)"}) {
		t.Fatal(paths)
	}
	var dead []string
	for _, r := range Dead(root, all, os.Stderr) {
		dead = append(dead, r.Path+":"+*r.Snippet+":"+*r.Problem)
	}
	want := []string{"index.md:chem/ECFP.md:case differs", "index.md:missing.md:missing", "index.md:../x.md:outside root", "index.md:linked/a.md:symlink"}
	if !reflect.DeepEqual(dead, want) {
		t.Fatalf("got  %q\nwant %q", dead, want)
	}
}
