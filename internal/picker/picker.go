package picker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mariusrueve/note-cli/internal/config"
	"github.com/mariusrueve/note-cli/internal/errs"
	"github.com/mariusrueve/note-cli/internal/notes"
	"github.com/mariusrueve/note-cli/internal/process"
	"github.com/mariusrueve/note-cli/internal/search"
)

type Manifest struct {
	SchemaVersion int             `json:"schema_version"`
	Root          string          `json:"root"`
	Records       []search.Result `json:"records"`
}

func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
func Environment() []string {
	out := []string{}
	for _, e := range os.Environ() {
		key, _, _ := strings.Cut(e, "=")
		if key != "SHELL" && key != "FZF_DEFAULT_OPTS" && key != "FZF_DEFAULT_OPTS_FILE" && key != "FZF_DEFAULT_COMMAND" {
			out = append(out, e)
		}
	}
	return append(out, "SHELL=/bin/sh", "FZF_DEFAULT_OPTS=", "FZF_DEFAULT_OPTS_FILE=", "FZF_DEFAULT_COMMAND=")
}
func Pick(ctx context.Context, c config.Config, results []search.Result, kind, executable string, stderr io.Writer) (search.Result, error) {
	if len(results) == 0 {
		return search.Result{}, errs.New(1, "no notes found")
	}
	f, e := os.CreateTemp("", "note-picker-*.json")
	if e != nil {
		return search.Result{}, e
	}
	defer os.Remove(f.Name())
	e = json.NewEncoder(f).Encode(Manifest{1, c.Root, results})
	ce := f.Close()
	if e != nil {
		return search.Result{}, e
	}
	if ce != nil {
		return search.Result{}, ce
	}
	if executable == "" {
		executable, e = os.Executable()
		if e != nil {
			return search.Result{}, e
		}
	}
	executable, e = filepath.Abs(executable)
	if e != nil {
		return search.Result{}, e
	}
	preview := Quote(executable) + " __preview --manifest " + Quote(f.Name()) + " --id {1}"
	args := []string{"--read0", "--print0", "--delimiter=\t", "--with-nth=2..", "--nth=1..", "--no-multi", "--no-sort", "--query=", "--height=" + c.Finder.Height, "--preview-window=" + c.Finder.PreviewWindow, "--preview=" + preview}
	var records bytes.Buffer
	for i, r := range results {
		display := search.Visible(r.Path)
		if kind == "grep" {
			display += fmt.Sprintf(":%d: %s", *r.Line, search.Visible(*r.Snippet))
		} else if kind == "search" {
			display += " [" + strings.Join(r.MatchKinds, "+") + "]"
		}
		fmt.Fprintf(&records, "%d\t%s%c", i+1, display, 0)
	}
	cmd := process.Command(ctx, c.Tools.FZF, args...)
	cmd.Dir = c.Root
	cmd.Env = Environment()
	cmd.Stdin = &records
	cmd.Stderr = stderr
	b, e := cmd.Output()
	if ctx.Err() != nil {
		return search.Result{}, ctx.Err()
	}
	if e != nil {
		code := process.ExitCode(e)
		if code == 1 || code == 130 {
			return search.Result{}, errs.New(130, "selection canceled")
		}
		return search.Result{}, fmt.Errorf("fzf: %w", e)
	}
	recordsOut := bytes.Split(b, []byte{0})
	if len(recordsOut) != 2 || len(recordsOut[1]) != 0 {
		return search.Result{}, fmt.Errorf("invalid fzf selection framing")
	}
	id, _, ok := strings.Cut(string(recordsOut[0]), "\t")
	n, e := strconv.Atoi(id)
	if !ok || e != nil || n <= 0 || n > len(results) {
		return search.Result{}, fmt.Errorf("invalid fzf selection ID")
	}
	r := results[n-1]
	if _, e = notes.Safe(c.Root, r.Path, false); e != nil {
		return search.Result{}, e
	}
	return r, nil
}
func Preview(manifest string, id int, w io.Writer) error {
	f, e := os.Open(manifest)
	if e != nil {
		return e
	}
	defer f.Close()
	var m Manifest
	d := json.NewDecoder(io.LimitReader(f, 16<<20))
	d.DisallowUnknownFields()
	if e = d.Decode(&m); e != nil {
		return fmt.Errorf("preview manifest: %w", e)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("preview manifest has trailing data")
	}
	if m.SchemaVersion != 1 || !filepath.IsAbs(m.Root) || id < 1 || id > len(m.Records) {
		return fmt.Errorf("invalid preview manifest/root/ID")
	}
	root, e := filepath.EvalSymlinks(m.Root)
	if e != nil {
		return e
	}
	if root != m.Root {
		return fmt.Errorf("preview root must be canonical")
	}
	r := m.Records[id-1]
	if !notes.Markdown(r.Path) {
		return fmt.Errorf("preview requires a Markdown file")
	}
	p, e := notes.Safe(root, r.Path, false)
	if e != nil {
		return e
	}
	f2, e := os.Open(p)
	if e != nil {
		return e
	}
	defer f2.Close()
	const budget = 1 << 20
	b, e := io.ReadAll(io.LimitReader(f2, budget+1))
	if e != nil {
		return e
	}
	truncated := len(b) > budget
	if truncated {
		b = b[:budget]
	}
	if !utf8.Valid(b) {
		fmt.Fprintln(w, "[preview contains invalid UTF-8; replacement characters shown]")
	}
	lines := bytes.Split(b, []byte{'\n'})
	target := 1
	if r.Line != nil && *r.Line > 0 {
		target = *r.Line
	}
	start := target - 20
	if start < 1 {
		start = 1
	}
	if start > len(lines) {
		start = 1
		fmt.Fprintln(w, "[match is beyond preview read budget]")
	}
	end := start + 79
	if end > len(lines) {
		end = len(lines)
	}
	out := bufio.NewWriter(w)
	for n := start; n <= end; n++ {
		prefix := " "
		if n == target {
			prefix = ">"
		}
		if _, e = fmt.Fprintf(out, "%s %5d  %s\n", prefix, n, search.Visible(string(lines[n-1]))); e != nil {
			return e
		}
	}
	if truncated {
		fmt.Fprintln(out, "[preview truncated at 1 MiB]")
	}
	return out.Flush()
}
