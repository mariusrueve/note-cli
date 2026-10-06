package search

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mariusrueve/note-cli/internal/config"
	"github.com/mariusrueve/note-cli/internal/notes"
	"github.com/mariusrueve/note-cli/internal/process"
	"golang.org/x/text/cases"
)

type Result struct {
	Path       string   `json:"path"`
	MatchKinds []string `json:"match_kinds"`
	Line       *int     `json:"line"`
	Snippet    *string  `json:"snippet"`
	MtimeNS    *int64   `json:"mtime_ns,omitempty"`
}

func Listing(path string) Result { return Result{Path: path, MatchKinds: []string{}} }
func Discover(ctx context.Context, c config.Config, warn io.Writer) ([]Result, error) {
	cmd := process.Background(ctx, c.Tools.RG, "--no-config", "--files", "--null")
	cmd.Dir = c.Root
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	b, e := cmd.Output()
	if e != nil && process.ExitCode(e) != 1 {
		return nil, fmt.Errorf("ripgrep discovery: %w: %s", e, diagnostic.String())
	}
	out := []Result{}
	for _, p := range bytes.Split(b, []byte{0}) {
		if len(p) == 0 {
			continue
		}
		if !utf8.Valid(p) {
			fmt.Fprintln(warn, "Warning: skipping non-UTF-8 filename")
			continue
		}
		rel := filepath.ToSlash(string(p))
		if !notes.Markdown(rel) {
			continue
		}
		if _, e := notes.Safe(c.Root, rel, false); e != nil {
			fmt.Fprintf(warn, "Warning: skipping %s: %v\n", Visible(rel), e)
			continue
		}
		out = append(out, Listing(rel))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
func Find(all []Result, q string, sensitive bool) []Result {
	out := []Result{}
	fold := cases.Fold()
	if !sensitive {
		q = fold.String(q)
	}
	for _, r := range all {
		p := r.Path
		if !sensitive {
			p = fold.String(p)
		}
		if strings.Contains(p, q) {
			r.MatchKinds = []string{"path"}
			out = append(out, r)
		}
	}
	return out
}

type rgString struct {
	Text  *string `json:"text"`
	Bytes string  `json:"bytes"`
}

func (s rgString) value() (string, error) {
	if s.Text != nil {
		return *s.Text, nil
	}
	b, e := base64.StdEncoding.DecodeString(s.Bytes)
	return string(b), e
}

type rgEvent struct {
	Type string `json:"type"`
	Data struct {
		Path       rgString `json:"path"`
		Lines      rgString `json:"lines"`
		LineNumber int      `json:"line_number"`
	} `json:"data"`
}

func Grep(ctx context.Context, c config.Config, all []Result, q string, sensitive bool) ([]Result, error) {
	out := []Result{}
	// Keep batches well below macOS/Linux ARG_MAX, including the environment.
	for start := 0; start < len(all); {
		end := start
		size := 0
		for end < len(all) && size+len(all[end].Path)+3 < 32000 {
			size += len(all[end].Path) + 3
			end++
		}
		if end == start {
			end++
		}
		args := []string{"--no-config", "--json", "--fixed-strings", "--line-number"}
		if !sensitive {
			args = append(args, "--ignore-case")
		}
		args = append(args, "--regexp", q, "--")
		allowed := map[string]bool{}
		for _, r := range all[start:end] {
			args = append(args, "./"+r.Path)
			allowed[r.Path] = true
		}
		cmd := process.Background(ctx, c.Tools.RG, args...)
		cmd.Dir = c.Root
		var diagnostic bytes.Buffer
		cmd.Stderr = &diagnostic
		b, e := cmd.Output()
		if e != nil && process.ExitCode(e) != 1 {
			return nil, fmt.Errorf("ripgrep search: %w: %s", e, diagnostic.String())
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		for {
			var ev rgEvent
			e = dec.Decode(&ev)
			if e == io.EOF {
				break
			}
			if e != nil {
				return nil, fmt.Errorf("ripgrep JSON: %w", e)
			}
			if ev.Type != "match" {
				continue
			}
			p, e := ev.Data.Path.value()
			if e != nil || !utf8.ValidString(p) {
				return nil, fmt.Errorf("invalid path identity in ripgrep output")
			}
			p = strings.TrimPrefix(filepath.ToSlash(p), "./")
			if !allowed[p] {
				return nil, fmt.Errorf("ripgrep returned an undiscovered path")
			}
			s, e := ev.Data.Lines.value()
			if e != nil {
				return nil, e
			}
			s = strings.ToValidUTF8(strings.TrimSuffix(strings.TrimSuffix(s, "\n"), "\r"), "�")
			line := ev.Data.LineNumber
			out = append(out, Result{Path: p, MatchKinds: []string{"content"}, Line: &line, Snippet: &s})
		}
		start = end
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return *out[i].Line < *out[j].Line
	})
	return out, nil
}
func Union(paths, content []Result) []Result {
	m := map[string]Result{}
	for _, r := range paths {
		m[r.Path] = r
	}
	for _, r := range content {
		old, exists := m[r.Path]
		if exists {
			if old.Line != nil {
				continue
			}
			r.MatchKinds = []string{"path", "content"}
		}
		m[r.Path] = r
	}
	out := make([]Result, 0, len(m))
	for _, r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		a := out[i].MatchKinds[0] == "path"
		b := out[j].MatchKinds[0] == "path"
		if a != b {
			return a
		}
		return out[i].Path < out[j].Path
	})
	return out
}
func Recent(root string, all []Result, limit int, warn io.Writer) []Result {
	out := []Result{}
	for _, r := range all {
		p, e := notes.Safe(root, r.Path, false)
		if e == nil {
			var i os.FileInfo
			i, e = os.Stat(p)
			if e == nil {
				ns := i.ModTime().UnixNano()
				r.MtimeNS = &ns
				out = append(out, r)
			}
		}
		if e != nil {
			fmt.Fprintf(warn, "Warning: skipping %s: %v\n", Visible(r.Path), e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if *out[i].MtimeNS != *out[j].MtimeNS {
			return *out[i].MtimeNS > *out[j].MtimeNS
		}
		return out[i].Path < out[j].Path
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
func Equal(a, b string, sensitive bool) bool {
	if sensitive {
		return a == b
	}
	return cases.Fold().String(a) == cases.Fold().String(b)
}

// Visible escapes controls without changing the identity carried in JSON/argv.
func Visible(s string) string {
	var b strings.Builder
	for _, r := range strings.ToValidUTF8(s, "�") {
		switch r {
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
				fmt.Fprintf(&b, `\u{%04x}`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
func Print(w io.Writer, kind string, results []Result, asJSON bool) error {
	if results == nil {
		results = []Result{}
	}
	if asJSON {
		return json.NewEncoder(w).Encode(results)
	}
	b := bufio.NewWriter(w)
	for _, r := range results {
		var e error
		if kind == "grep" {
			_, e = fmt.Fprintf(b, "%s:%d:%s\n", Visible(r.Path), *r.Line, Visible(*r.Snippet))
		} else {
			_, e = fmt.Fprintln(b, Visible(r.Path))
		}
		if e != nil {
			return e
		}
	}
	return b.Flush()
}
