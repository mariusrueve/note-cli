package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
)

func moduleDirectory(path string) (string, error) {
	b, e := exec.Command("go", "list", "-mod=readonly", "-m", "-f", "{{.Dir}}", path).Output()
	if e != nil {
		return "", fmt.Errorf("resolve license source for %s: %w", path, e)
	}
	dir := strings.TrimSpace(string(b))
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("module %s has no downloaded license source", path)
	}
	return dir, nil
}

func noticeFile(name string) bool {
	n := strings.ToUpper(name)
	for _, prefix := range []string{"LICENSE", "COPYING", "NOTICE", "COPYRIGHT", "PATENTS"} {
		if n == prefix || strings.HasPrefix(n, prefix+".") || strings.HasPrefix(n, prefix+"-") {
			return true
		}
	}
	return false
}

func collectNotices(label, dir string, recursive, required bool) ([]byte, error) {
	var files []string
	hasLicense := false
	e := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if entry.IsDir() {
			if path != dir && !recursive {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || !noticeFile(entry.Name()) {
			return nil
		}
		name := strings.ToUpper(entry.Name())
		if strings.HasPrefix(name, "LICENSE") || strings.HasPrefix(name, "COPYING") {
			hasLicense = true
		}
		files = append(files, path)
		return nil
	})
	if e != nil {
		if !required && os.IsNotExist(e) {
			return nil, nil
		}
		return nil, e
	}
	if required && !hasLicense {
		return nil, fmt.Errorf("no license file found for %s", label)
	}
	sort.Strings(files)
	var out bytes.Buffer
	for _, path := range files {
		b, e := os.ReadFile(path)
		if e != nil {
			return nil, e
		}
		rel, _ := filepath.Rel(dir, path)
		fmt.Fprintf(&out, "\n--- %s: %s ---\n\n", label, filepath.ToSlash(rel))
		out.Write(b)
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

// Select modules from the actual linked binary, excluding development/test-only
// dependencies. Keep the project MIT license first in each archive's LICENSE.
func thirdPartyNotices(build *debug.BuildInfo, goroot string, directory func(string) (string, error)) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("Bundled third-party notices\n==========================\nThe following notices apply to components linked into the note executable.\n")
	for _, source := range []struct {
		label, dir          string
		recursive, required bool
	}{
		{"Go standard library (" + build.GoVersion + ")", goroot, false, true},
		{"Go standard library vendor components", filepath.Join(goroot, "src", "vendor"), true, false},
	} {
		b, e := collectNotices(source.label, source.dir, source.recursive, source.required)
		if e != nil {
			return nil, e
		}
		out.Write(b)
	}
	modules := append([]*debug.Module(nil), build.Deps...)
	sort.Slice(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })
	for _, m := range modules {
		if m.Replace != nil {
			return nil, fmt.Errorf("release dependency %s has a replacement; use published locked modules", m.Path)
		}
		dir, e := directory(m.Path)
		if e != nil {
			return nil, e
		}
		b, e := collectNotices(m.Path+" "+m.Version, dir, false, true)
		if e != nil {
			return nil, e
		}
		out.Write(b)
	}
	return out.Bytes(), nil
}
