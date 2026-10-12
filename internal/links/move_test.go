package links

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mariusrueve/note-cli/internal/search"
)

func TestRelativeAndRetarget(t *testing.T) {
	for _, c := range [][3]string{
		{"a.md", "b.md", "b.md"},
		{"x/a.md", "b.md", "../b.md"},
		{"x/a.md", "x/y/b.md", "y/b.md"},
		{"x/y/a.md", "z/b.md", "../../z/b.md"},
		{"a.md", ".", "."},
	} {
		if got := Relative(c[0], c[1]); got != c[2] {
			t.Fatal(c, got)
		}
	}
	link := func(target string, angle bool) Link { return Link{Target: target, Angle: angle} }
	for _, c := range []struct {
		l        Link
		from, to string
		want     string
	}{
		{link("old.md#part", false), "index.md", "new dir/new.md", "new%20dir/new.md#part"},
		{link("old.md?x=1", false), "sub/a.md", "new.md", "../new.md?x=1"},
		{link("/old.md", false), "sub/a.md", "dir/new.md", "/dir/new.md"},
		{link("old.md", true), "a.md", "with space.md", "with space.md"},
		{link("old.md", true), "a.md", "100%.md", "100%25.md"},
	} {
		if got := retarget(c.l, c.from, c.to); got != c.want {
			t.Fatal(c, got)
		}
	}
}

func TestPlanAndApplyMove(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"inbox/idea.md": "# Idea\n[ecfp](../chem/ecfp.md) [self](idea.md#top) [img](img/a%20b.png)\n[web](https://x.org) [anchor](#here) [root](/index.md)\n[ref]: ../chem/ecfp.md\n",
		"index.md":      "[idea](inbox/idea.md) and [again](inbox/idea.md#top)\r\n`[code](inbox/idea.md)`\n[ref]: <inbox/idea.md>\n",
		"chem/ecfp.md":  "back to [idea](../inbox/idea.md)\n",
		"other.md":      "no links here\n",
	}
	for p, s := range files {
		os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0755)
		os.WriteFile(filepath.Join(root, p), []byte(s), 0640)
	}
	all := []search.Result{search.Listing("chem/ecfp.md"), search.Listing("inbox/idea.md"), search.Listing("index.md"), search.Listing("other.md")}
	plan, e := PlanMove(root, all, "inbox/idea.md", "research/ideas/idea.md", io.Discard)
	if e != nil {
		t.Fatal(e)
	}
	var got []string
	for _, ed := range plan.Edits {
		got = append(got, ed.Path+":"+ed.Old+">"+ed.New)
	}
	want := []string{
		"chem/ecfp.md:../inbox/idea.md>../research/ideas/idea.md",
		"index.md:inbox/idea.md>research/ideas/idea.md",
		"index.md:inbox/idea.md#top>research/ideas/idea.md#top",
		"index.md:inbox/idea.md>research/ideas/idea.md",
		"research/ideas/idea.md:../chem/ecfp.md>../../chem/ecfp.md",
		"research/ideas/idea.md:img/a%20b.png>../../inbox/img/a%20b.png",
		"research/ideas/idea.md:../chem/ecfp.md>../../chem/ecfp.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if e = plan.Unchanged(root); e != nil {
		t.Fatal(e)
	}
	os.MkdirAll(filepath.Join(root, "research", "ideas"), 0755)
	if e = os.Rename(filepath.Join(root, "inbox", "idea.md"), filepath.Join(root, "research", "ideas", "idea.md")); e != nil {
		t.Fatal(e)
	}
	done, e := plan.Apply(root)
	if e != nil || !reflect.DeepEqual(done, []string{"chem/ecfp.md", "index.md", "research/ideas/idea.md"}) {
		t.Fatal(done, e)
	}
	b, _ := os.ReadFile(filepath.Join(root, "index.md"))
	if string(b) != "[idea](research/ideas/idea.md) and [again](research/ideas/idea.md#top)\r\n`[code](inbox/idea.md)`\n[ref]: <research/ideas/idea.md>\n" {
		t.Fatalf("%q", b)
	}
	b, _ = os.ReadFile(filepath.Join(root, "research", "ideas", "idea.md"))
	if !strings.Contains(string(b), "[self](idea.md#top)") || !strings.Contains(string(b), "[root](/index.md)") || !strings.Contains(string(b), "[ref]: ../../chem/ecfp.md") {
		t.Fatalf("%q", b)
	}
	if i, _ := os.Stat(filepath.Join(root, "index.md")); i.Mode().Perm() != 0640 {
		t.Fatal(i.Mode())
	}
	if entries, _ := os.ReadDir(root); len(entries) != 5 {
		t.Fatal("temporary files left behind", entries)
	}
	if dead := Dead(root, []search.Result{search.Listing("chem/ecfp.md"), search.Listing("index.md"), search.Listing("research/ideas/idea.md")}, io.Discard); len(dead) != 1 || *dead[0].Snippet != "../../inbox/img/a%20b.png" {
		t.Fatal(dead)
	}
	// Changes after planning are detected before anything moves.
	plan, _ = PlanMove(root, []search.Result{search.Listing("index.md"), search.Listing("research/ideas/idea.md")}, "research/ideas/idea.md", "idea.md", io.Discard)
	os.WriteFile(filepath.Join(root, "index.md"), []byte("edited\n"), 0640)
	if e = plan.Unchanged(root); e == nil {
		t.Fatal("concurrent edit not detected")
	}
}
