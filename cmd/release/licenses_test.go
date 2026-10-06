package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func TestThirdPartyNotices(t *testing.T) {
	dir := t.TempDir()
	sdk := filepath.Join(dir, "sdk")
	module := filepath.Join(dir, "runtime-module")
	os.MkdirAll(filepath.Join(sdk, "src", "vendor", "dependency"), 0755)
	os.Mkdir(module, 0755)
	for path, text := range map[string]string{
		filepath.Join(sdk, "LICENSE"):                                "standard library license",
		filepath.Join(sdk, "PATENTS"):                                "patent notice",
		filepath.Join(sdk, "src", "vendor", "dependency", "LICENSE"): "vendor license",
		filepath.Join(module, "LICENSE.txt"):                         "module license",
		filepath.Join(module, "NOTICE"):                              "module copyright notice",
		filepath.Join(module, "unrelated.go"):                        "not included",
	} {
		if e := os.WriteFile(path, []byte(text), 0600); e != nil {
			t.Fatal(e)
		}
	}
	build := &debug.BuildInfo{GoVersion: "go1.27.1", Deps: []*debug.Module{{Path: "example.org/runtime", Version: "v1.2.3"}}}
	lookup := func(path string) (string, error) {
		if path != "example.org/runtime" {
			t.Fatal("unlinked dependency", path)
		}
		return module, nil
	}
	b, e := thirdPartyNotices(build, sdk, lookup)
	if e != nil {
		t.Fatal(e)
	}
	for _, text := range []string{"go1.27.1", "standard library license", "vendor license", "patent notice", "example.org/runtime v1.2.3", "module license", "module copyright notice"} {
		if !bytes.Contains(b, []byte(text)) {
			t.Fatal("missing notice", text)
		}
	}
	if bytes.Contains(b, []byte("not included")) || bytes.Contains(b, []byte(dir)) {
		t.Fatal("included unrelated data or machine paths")
	}
	second, e := thirdPartyNotices(build, sdk, lookup)
	if e != nil || !bytes.Equal(b, second) {
		t.Fatal("non-deterministic notices", e)
	}
	os.Remove(filepath.Join(module, "LICENSE.txt"))
	if _, e = thirdPartyNotices(build, sdk, lookup); e == nil || !strings.Contains(e.Error(), "no license file") {
		t.Fatal("allowed missing dependency license", e)
	}
	build.Deps[0].Replace = &debug.Module{Path: "local"}
	if _, e = thirdPartyNotices(build, sdk, lookup); e == nil {
		t.Fatal("allowed unpublished module replacement")
	}
}
