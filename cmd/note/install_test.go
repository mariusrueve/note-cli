package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mariusrueve/note-cli/internal/update"
)

// Exercise the POSIX installer with real native binaries, tar and SHA256 tools.
// Only curl is replaced; no test contacts GitHub or installs into a personal bin.
func TestReleaseInstaller(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "release")
	if e := os.Mkdir(fixture, 0755); e != nil {
		t.Fatal(e)
	}
	binary := os.Getenv("NOTE_TEST_BINARY")
	version := ""
	if binary != "" {
		b, e := exec.Command(binary, "--version").Output()
		fields := strings.Fields(string(b))
		if e == nil && len(fields) >= 3 {
			version = fields[2]
		}
	}
	if _, e := update.ParseVersion(version); e != nil {
		version = "0.1.0"
		binary = filepath.Join(dir, "archive-note")
		cmd := exec.Command("go", "build", "-mod=readonly", "-ldflags=-X main.version="+version+" -X main.distribution=archive", "-o", binary, ".")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatal(string(b), e)
		}
	}
	data, e := os.ReadFile(binary)
	if e != nil {
		t.Fatal(e)
	}
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	for _, entry := range []struct {
		name string
		data []byte
	}{{"note", data}, {"README.md", []byte("readme")}, {"LICENSE", []byte("license")}} {
		if e := tw.WriteHeader(&tar.Header{Name: entry.name, Mode: 0755, Size: int64(len(entry.data))}); e != nil {
			t.Fatal(e)
		}
		if _, e := tw.Write(entry.data); e != nil {
			t.Fatal(e)
		}
	}
	if e := tw.Close(); e != nil {
		t.Fatal(e)
	}
	if e := gz.Close(); e != nil {
		t.Fatal(e)
	}
	name, e := update.ArchiveName(version, runtime.GOOS, runtime.GOARCH)
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(fixture, name), archive.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	sums := []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(archive.Bytes()), name))
	if e := os.WriteFile(filepath.Join(fixture, "checksums.txt"), sums, 0600); e != nil {
		t.Fatal(e)
	}
	tools := filepath.Join(dir, "tools")
	os.Mkdir(tools, 0755)
	curl := `#!/bin/sh
set -eu
destination=
address=
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) destination=$2; shift 2 ;;
    https://*) address=$1; shift ;;
    *) shift ;;
  esac
done
printf '%s\n' "$address" >> "$NOTE_INSTALL_FIXTURE/log"
case "$address" in
  https://github.com/mariusrueve/note-cli/releases/latest/download/checksums.txt|https://github.com/mariusrueve/note-cli/releases/download/v"$NOTE_INSTALL_FIXTURE_VERSION"/checksums.txt) file=checksums.txt ;;
  https://github.com/mariusrueve/note-cli/releases/download/v"$NOTE_INSTALL_FIXTURE_VERSION"/"$NOTE_INSTALL_FIXTURE_ARCHIVE") file=$NOTE_INSTALL_FIXTURE_ARCHIVE ;;
  *) printf 'Unexpected download URL: %s\n' "$address" >&2; exit 22 ;;
esac
cp "$NOTE_INSTALL_FIXTURE/$file" "$destination"
`
	if e := os.WriteFile(filepath.Join(tools, "curl"), []byte(curl), 0755); e != nil {
		t.Fatal(e)
	}
	script, e := filepath.Abs("../../scripts/install.sh")
	if e != nil {
		t.Fatal(e)
	}
	install := func(destination, pin string) ([]byte, error) {
		cmd := exec.Command("sh", script)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"), "NOTE_INSTALL_DIR="+destination, "NOTE_VERSION="+pin, "NOTE_INSTALL_FIXTURE="+fixture, "NOTE_INSTALL_FIXTURE_VERSION="+version, "NOTE_INSTALL_FIXTURE_ARCHIVE="+name)
		return cmd.CombinedOutput()
	}
	destination := filepath.Join(dir, "installed bin")
	if b, e := install(destination, ""); e != nil {
		t.Fatal(string(b), e)
	}
	installed := filepath.Join(destination, "note")
	if b, e := exec.Command(installed, "--version").Output(); e != nil || !strings.Contains(string(b), "note version "+version+" (") {
		t.Fatal(string(b), e)
	}
	if b, e := install(destination, version); e != nil {
		t.Fatal("pinned reinstallation", string(b), e)
	}
	unmanaged := filepath.Join(dir, "unmanaged")
	os.Mkdir(unmanaged, 0755)
	os.WriteFile(filepath.Join(unmanaged, "note"), []byte("another installer"), 0755)
	if b, e := install(unmanaged, ""); e == nil || !strings.Contains(string(b), "not managed") {
		t.Fatal(string(b), e)
	}
	if b, _ := os.ReadFile(filepath.Join(unmanaged, "note")); string(b) != "another installer" {
		t.Fatal("overwrote unmanaged binary")
	}
	before, _ := os.ReadFile(installed)
	markerBefore, _ := os.ReadFile(filepath.Join(destination, ".note-install.json"))
	for _, pin := range []string{"v0.1.0", "../../else", "01.0.0", "1.0.0\n2.0.0"} {
		if b, e := install(destination, pin); e == nil || !strings.Contains(string(b), "invalid NOTE_VERSION") {
			t.Fatal(pin, string(b), e)
		}
	}
	for _, badManifest := range [][]byte{
		[]byte(strings.ReplaceAll(string(sums), ".tar.gz", "XtarXgz")),
		append(bytes.Clone(sums), sums...),
	} {
		os.WriteFile(filepath.Join(fixture, "checksums.txt"), badManifest, 0600)
		if b, e := install(destination, ""); e == nil || !strings.Contains(string(b), "no unique, valid checksum") {
			t.Fatal("accepted invalid or duplicate archive name", string(b), e)
		}
	}
	badSums := []byte(strings.Repeat("0", 64) + "  " + name + "\n")
	os.WriteFile(filepath.Join(fixture, "checksums.txt"), badSums, 0600)
	if b, e := install(destination, ""); e == nil {
		t.Fatal("accepted incorrect checksum", string(b))
	}
	after, _ := os.ReadFile(installed)
	markerAfter, _ := os.ReadFile(filepath.Join(destination, ".note-install.json"))
	if !bytes.Equal(before, after) || !bytes.Equal(markerBefore, markerAfter) {
		t.Fatal("failed installation changed installed files")
	}
}
