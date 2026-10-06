// release builds local archives and their dependency/build record. Publication is
// handled only by the gated GitHub release workflow.
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
)

type targetInfo struct {
	OS           string           `json:"os"`
	Arch         string           `json:"arch"`
	BinarySHA256 string           `json:"binary_sha256"`
	Build        *debug.BuildInfo `json:"go_build_info"`
}
type releaseInfo struct {
	Version string       `json:"version"`
	Commit  string       `json:"commit"`
	Dirty   bool         `json:"dirty"`
	Targets []targetInfo `json:"targets"`
}

func archive(path, binary string, notices []byte) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, entry := range []struct {
		name, src string
		mode      int64
	}{{"note", binary, 0755}, {"README.md", "README.md", 0644}, {"LICENSE", "LICENSE", 0644}} {
		b, e := os.ReadFile(entry.src)
		if e != nil {
			tw.Close()
			gz.Close()
			f.Close()
			return e
		}
		if entry.name == "LICENSE" {
			b = append(append(b, '\n'), notices...)
		}
		if e = tw.WriteHeader(&tar.Header{Name: entry.name, Mode: entry.mode, Size: int64(len(b))}); e != nil {
			tw.Close()
			gz.Close()
			f.Close()
			return e
		}
		if _, e = tw.Write(b); e != nil {
			tw.Close()
			gz.Close()
			f.Close()
			return e
		}
	}
	if e = tw.Close(); e != nil {
		gz.Close()
		f.Close()
		return e
	}
	if e = gz.Close(); e != nil {
		f.Close()
		return e
	}
	return f.Close()
}
func run(version, output string) error {
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`).MatchString(version) {
		return fmt.Errorf("invalid version")
	}
	commit, e := exec.Command("git", "rev-parse", "HEAD").Output()
	if e != nil {
		return e
	}
	sha := strings.TrimSpace(string(commit))
	status, e := exec.Command("git", "status", "--porcelain", "--untracked-files=normal").Output()
	if e != nil {
		return e
	}
	info := releaseInfo{Version: version, Commit: sha, Dirty: len(status) != 0}
	if version != "snapshot" && info.Dirty {
		return fmt.Errorf("versioned releases require a clean Git checkout; commit changes first or build a snapshot")
	}
	dest := filepath.Join(output, version)
	if e = os.MkdirAll(dest, 0755); e != nil {
		return e
	}
	temp, e := os.MkdirTemp(dest, "build-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(temp)
	var sums strings.Builder
	goRootBytes, e := exec.Command("go", "env", "GOROOT").Output()
	if e != nil {
		return e
	}
	goRoot := strings.TrimSpace(string(goRootBytes))
	for _, target := range []struct{ os, arch string }{{"darwin", "arm64"}, {"darwin", "amd64"}, {"linux", "arm64"}, {"linux", "amd64"}} {
		bin := filepath.Join(temp, "note")
		cmd := exec.Command("go", "build", "-mod=readonly", "-buildvcs=false", "-trimpath", "-ldflags=-s -w -X main.version="+version+" -X main.commit="+sha+" -X main.distribution=archive", "-o", bin, "./cmd/note")
		for _, v := range os.Environ() {
			key, _, _ := strings.Cut(v, "=")
			if key != "GOOS" && key != "GOARCH" && key != "CGO_ENABLED" {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, "GOOS="+target.os, "GOARCH="+target.arch, "CGO_ENABLED=0")
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if e = cmd.Run(); e != nil {
			return e
		}
		build, e := buildinfo.ReadFile(bin)
		if e != nil {
			return e
		}
		binaryBytes, e := os.ReadFile(bin)
		if e != nil {
			return e
		}
		info.Targets = append(info.Targets, targetInfo{target.os, target.arch, fmt.Sprintf("%x", sha256.Sum256(binaryBytes)), build})
		name := "note_" + version + "_" + target.os + "_" + target.arch + ".tar.gz"
		p := filepath.Join(dest, name)
		notices, e := thirdPartyNotices(build, goRoot, moduleDirectory)
		if e != nil {
			return e
		}
		if e = archive(p, bin, notices); e != nil {
			return e
		}
		f, e := os.Open(p)
		if e != nil {
			return e
		}
		hash := sha256.New()
		_, e = io.Copy(hash, f)
		ce := f.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
		fmt.Fprintf(&sums, "%x  %s\n", hash.Sum(nil), name)
		fmt.Fprintln(os.Stderr, "Built", p)
	}
	metadata, e := json.MarshalIndent(info, "", "  ")
	if e != nil {
		return e
	}
	metadata = append(metadata, '\n')
	if e = os.WriteFile(filepath.Join(dest, "build-info.json"), metadata, 0644); e != nil {
		return e
	}
	fmt.Fprintf(&sums, "%x  build-info.json\n", sha256.Sum256(metadata))
	installer, e := os.ReadFile("scripts/install.sh")
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(dest, "install.sh"), installer, 0644); e != nil {
		return e
	}
	fmt.Fprintf(&sums, "%x  install.sh\n", sha256.Sum256(installer))
	return os.WriteFile(filepath.Join(dest, "checksums.txt"), []byte(sums.String()), 0644)
}
func main() {
	version := flag.String("version", "snapshot", "Version stamp")
	output := flag.String("output", "work/releases", "Local output directory")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(2)
	}
	if e := run(*version, *output); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
