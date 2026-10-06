package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

// The editor helper is a Go executable, so these tests need no editor installation.
func TestPTYEditor(t *testing.T) {
	if os.Getenv("NOTE_PTY_EDITOR") != "1" {
		return
	}
	fmt.Printf("\nEDITOR-OPEN:%s\n", os.Args[len(os.Args)-1])
	os.Exit(0)
}

func TestInteractiveTerminal(t *testing.T) {
	dir := t.TempDir()
	binary := os.Getenv("NOTE_TEST_BINARY")
	if binary == "" {
		binary = filepath.Join(dir, "note")
		build := exec.Command("go", "build", "-mod=readonly", "-o", binary, ".")
		if b, e := build.CombinedOutput(); e != nil {
			t.Fatal(string(b), e)
		}
	}
	root := filepath.Join(dir, "knowledge")
	if e := os.Mkdir(root, 0755); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "example.md")
	content := []byte("# Example\n\nPREVIEW-SOURCE-READY\n")
	if e := os.WriteFile(path, content, 0600); e != nil {
		t.Fatal(e)
	}
	editor, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	quote := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	cfg := filepath.Join(dir, "config.toml")
	if e := os.WriteFile(cfg, []byte("root="+quote(root)+"\neditor=["+quote(editor)+",\"-test.run=^TestPTYEditor$\"]\n[finder]\nheight=\"100%\"\npreview_window=\"right:60%:wrap\"\n"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name                 string
		args                 []string
		ready, input, result string
		code                 int
	}{
		{"picker opens editor", nil, "PREVIEW-SOURCE-READY", "\r", "example.md", 0},
		{"picker cancellation", nil, "PREVIEW-SOURCE-READY", "\x03", "", 130},
		{"creation opens editor", []string{"new", "created"}, "EDITOR-OPEN:", "", "created.md", 0},
		{"existing note cancellation", []string{"new", "example"}, "Enter", "\x03", "", 130},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binary, append([]string{"--config", cfg}, tc.args...)...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "TERM=xterm-256color", "NOTE_PTY_EDITOR=1", "XDG_CONFIG_HOME="+filepath.Join(dir, "xdg"))
			terminal, e := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 140})
			if e != nil {
				t.Fatal(e)
			}
			defer terminal.Close()
			defer func() {
				if cmd.ProcessState == nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			}()
			var mu sync.Mutex
			var output bytes.Buffer
			readDone := make(chan struct{})
			go func() {
				defer close(readDone)
				buf := make([]byte, 8192)
				for {
					n, err := terminal.Read(buf)
					mu.Lock()
					output.Write(buf[:n])
					mu.Unlock()
					// Real terminals answer cursor-position requests from fzf.
					if bytes.Contains(buf[:n], []byte("\x1b[6n")) {
						_, _ = io.WriteString(terminal, "\x1b[1;1R")
					}
					if err != nil {
						return
					}
				}
			}()
			transcript := func() string { mu.Lock(); defer mu.Unlock(); return output.String() }
			deadline := time.Now().Add(15 * time.Second)
			for !strings.Contains(transcript(), tc.ready) && time.Now().Before(deadline) {
				time.Sleep(20 * time.Millisecond)
			}
			if !strings.Contains(transcript(), tc.ready) {
				t.Fatalf("terminal never became ready: %q", transcript())
			}
			if tc.input != "" {
				if _, e = io.WriteString(terminal, tc.input); e != nil {
					t.Fatal(e)
				}
			}
			finished := make(chan error, 1)
			go func() { finished <- cmd.Wait() }()
			select {
			case e = <-finished:
			case <-time.After(15 * time.Second):
				_ = cmd.Process.Kill()
				<-finished
				t.Fatalf("terminal process did not exit: %q", transcript())
			}
			_ = terminal.Close()
			<-readDone
			code := 0
			if e != nil {
				if exit, ok := e.(*exec.ExitError); ok {
					code = exit.ExitCode()
				} else {
					t.Fatal(e)
				}
			}
			if code != tc.code || !strings.Contains(transcript(), tc.result) {
				t.Fatalf("exit %d, wanted %d: %q", code, tc.code, transcript())
			}
			if b, e := os.ReadFile(path); e != nil || !bytes.Equal(b, content) {
				t.Fatal("existing note changed", string(b), e)
			}
		})
	}
}
