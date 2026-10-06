package notes

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPaths(t *testing.T) {
	if Title("123_example-title.md") != "123 Example title" || Title("ECFP.md") != "ECFP" {
		t.Fatal("derived title")
	}
	for input, want := range map[string]string{"Molecular Fingerprints": "molecular-fingerprints.md", "docker/Network_Config.md": "docker/network-config.md", "knowledge/Überblick": "knowledge/ueberblick.md", "cheminformatics/ECFP": "cheminformatics/ecfp.md", "Crème & Straße.MD": "creme-strasse.md"} {
		got, e := NewPath(input)
		if e != nil || got != want {
			t.Fatalf("%s => %s: %v", input, got, e)
		}
		twice, e := NewPath(got)
		if e != nil || twice != got {
			t.Fatal("not idempotent")
		}
	}
	for _, p := range []string{"", "/absolute", "../escape", "a/../b", "a/./b", "a//b", "a/", "a\x00b", "a\nb", "日本語", ".git/test"} {
		if _, e := NewPath(p); e == nil {
			t.Fatalf("accepted %q", p)
		}
	}
	root := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "a.md"), []byte("safe"), 0600)
	os.Symlink(outside, filepath.Join(root, "link"))
	if _, e := Safe(root, "link/a.md", false); e == nil {
		t.Fatal("symlink escape")
	}
	if _, e := Safe(root, "link/new.md", true); e == nil {
		t.Fatal("creation symlink escape")
	}
	for _, p := range []string{"spaces and 'quotes'.md", "Ünicode:$(x)`y`.md", "-leading.md", "line\nbreak.md"} {
		os.WriteFile(filepath.Join(root, p), nil, 0600)
		if _, e := Safe(root, p, false); e != nil {
			t.Fatal(e)
		}
	}
	os.WriteFile(filepath.Join(root, "Network.md"), []byte("original"), 0600)
	p, yes, e := Collision(root, "network.md")
	if e != nil || !yes || p != "Network.md" {
		t.Fatalf("collision: %s %v %v", p, yes, e)
	}
	// Case-insensitive filesystems cannot represent two distinct case variants.
	if e = os.WriteFile(filepath.Join(root, "network.md"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	entries, _ := os.ReadDir(root)
	count := 0
	for _, i := range entries {
		if i.Name() == "Network.md" || i.Name() == "network.md" {
			count++
		}
	}
	if count == 2 {
		if _, _, e = Collision(root, "network.md"); e == nil {
			t.Fatal("ambiguous collision accepted")
		}
	}
}
func TestExclusiveCreation(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, s := range []string{"one", "two"} {
		wg.Go(func() { errs <- Create(root, "race.md", s) })
	}
	wg.Wait()
	close(errs)
	success, exist := 0, 0
	for e := range errs {
		if e == nil {
			success++
		} else if os.IsExist(e) {
			exist++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || exist != 1 {
		t.Fatal(success, exist)
	}
	b, _ := os.ReadFile(filepath.Join(root, "race.md"))
	if string(b) != "one" && string(b) != "two" {
		t.Fatal(string(b))
	}
}
