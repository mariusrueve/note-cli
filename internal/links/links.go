// Package links extracts ordinary relative Markdown links at invocation time.
// It is intentionally not a Markdown parser: it recognizes inline links/images,
// reference definitions, fenced code blocks, and inline code spans.
package links

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mariusrueve/note-cli/internal/notes"
	"github.com/mariusrueve/note-cli/internal/search"
)

// Link is one local link occurrence in a note.
type Link struct {
	Line   int
	Text   string // the source line, for snippets
	Target string // the target as written
	Rel    string // root-relative resolution without fragment/query; "" when outside the root
}

var (
	reference = regexp.MustCompile(`^ {0,3}\[[^\]]+\]:[ \t]*(<[^>]*>|\S+)`)
	scheme    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

// maxNote bounds how much of a single note is read.
const maxNote = 8 << 20

// Extract returns the local links in src, which lives at root-relative from.
func Extract(from, src string) []Link {
	out := []Link{}
	fence := ""
	for n, line := range strings.Split(src, "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimLeft(line, " ")
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]+" \t") == "" {
				fence = ""
			}
			continue
		}
		if len(line)-len(trimmed) < 4 {
			if f := fenceOf(trimmed); f != "" {
				fence = f
				continue
			}
		}
		var targets []string
		if m := reference.FindStringSubmatch(line); m != nil {
			targets = append(targets, strings.TrimSuffix(strings.TrimPrefix(m[1], "<"), ">"))
		} else {
			targets = inline(stripCode(line))
		}
		for _, t := range targets {
			if rel, local := resolve(from, t); local {
				out = append(out, Link{Line: n + 1, Text: line, Target: t, Rel: rel})
			}
		}
	}
	return out
}

func fenceOf(s string) string {
	for _, c := range []string{"`", "~"} {
		n := len(s) - len(strings.TrimLeft(s, c))
		if n >= 3 && !(c == "`" && strings.Contains(s[n:], "`")) {
			return strings.Repeat(c, n)
		}
	}
	return ""
}

// stripCode blanks inline code spans so their contents are not read as links.
func stripCode(s string) string {
	b := []byte(s)
	for i := 0; i < len(b); {
		if b[i] != '`' {
			i++
			continue
		}
		n := 1
		for i+n < len(b) && b[i+n] == '`' {
			n++
		}
		end := closing(b, i+n, n)
		if end < 0 {
			i += n
			continue
		}
		for j := i; j < end+n; j++ {
			b[j] = ' '
		}
		i = end + n
	}
	return string(b)
}
func closing(b []byte, from, n int) int {
	for i := from; i < len(b); i++ {
		if b[i] != '`' {
			continue
		}
		m := 1
		for i+m < len(b) && b[i+m] == '`' {
			m++
		}
		if m == n {
			return i
		}
		i += m - 1
	}
	return -1
}

// inline returns the destinations of [text](dest) and ![alt](dest) on a line.
func inline(s string) []string {
	var out []string
	for i := 0; ; {
		k := strings.Index(s[i:], "](")
		if k < 0 {
			return out
		}
		j := i + k + 2
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		start, depth := j, 0
		if j < len(s) && s[j] == '<' {
			end := strings.IndexByte(s[j+1:], '>')
			if end < 0 {
				i = j
				continue
			}
			out = append(out, s[j+1:j+1+end])
			i = j + 1 + end
			continue
		}
		for ; j < len(s); j++ {
			c := s[j]
			if c == '\\' {
				j++
				continue
			}
			if c == ' ' || c == '\t' || c == ')' && depth == 0 {
				break
			}
			if c == '(' {
				depth++
			} else if c == ')' {
				depth--
			}
		}
		if j > len(s) {
			j = len(s)
		}
		if j > start {
			out = append(out, s[start:j])
		}
		i = j
	}
}

// resolve maps a destination to a root-relative path. External URLs and
// anchor-only links are not local. A leading slash means the knowledge root.
func resolve(from, target string) (string, bool) {
	if target == "" || strings.HasPrefix(target, "//") || scheme.MatchString(target) {
		return "", false
	}
	t, _, _ := strings.Cut(target, "#")
	t, _, _ = strings.Cut(t, "?")
	if t == "" {
		return "", false
	}
	if u, e := url.PathUnescape(t); e == nil {
		t = u
	}
	var rel string
	if strings.HasPrefix(t, "/") {
		rel = path.Clean(strings.TrimLeft(t, "/"))
	} else {
		rel = path.Clean(path.Join(path.Dir(from), t))
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", true
	}
	return rel, true
}

// Scan reads each discovered note once and returns its local links.
func Scan(root string, all []search.Result, warn io.Writer) map[string][]Link {
	out := map[string][]Link{}
	for _, r := range all {
		p, e := notes.Safe(root, r.Path, false)
		var b []byte
		if e == nil {
			b, e = read(p)
		}
		if e != nil {
			fmt.Fprintf(warn, "Warning: skipping %s: %v\n", search.Visible(r.Path), e)
			continue
		}
		out[r.Path] = Extract(r.Path, string(b))
	}
	return out
}
func read(p string) ([]byte, error) {
	f, e := os.Open(p)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, maxNote+1))
	if e == nil && len(b) > maxNote {
		e = fmt.Errorf("larger than 8 MiB")
	}
	return b, e
}

func result(path string, l Link, snippet string) search.Result {
	line := l.Line
	snippet = strings.ToValidUTF8(snippet, "�")
	return search.Result{Path: path, MatchKinds: []string{"link"}, Line: &line, Snippet: &snippet}
}

// Backlinks lists link occurrences in all whose resolved target is rel.
func Backlinks(root string, all []search.Result, rel string, warn io.Writer) []search.Result {
	scanned := Scan(root, all, warn)
	out := []search.Result{}
	for _, r := range all {
		for _, l := range scanned[r.Path] {
			if l.Rel == rel {
				out = append(out, result(r.Path, l, l.Text))
			}
		}
	}
	return out
}

// Dead lists links whose target does not exist with exactly this spelling.
// The snippet is the target as written; Problem says why it is broken.
func Dead(root string, all []search.Result, warn io.Writer) []search.Result {
	scanned := Scan(root, all, warn)
	dirs := map[string]map[string]bool{}
	out := []search.Result{}
	for _, r := range all {
		for _, l := range scanned[r.Path] {
			problem := check(root, l.Rel, dirs)
			if problem == "" {
				continue
			}
			res := result(r.Path, l, l.Target)
			res.Problem = &problem
			out = append(out, res)
		}
	}
	return out
}

// check walks rel without following symlinks; names must match exactly so links
// also work on case-sensitive filesystems.
func check(root, rel string, dirs map[string]map[string]bool) string {
	if rel == "" {
		return "outside root"
	}
	if rel == "." {
		return ""
	}
	cur := root
	for _, part := range strings.Split(rel, "/") {
		names, ok := dirs[cur]
		if !ok {
			names = map[string]bool{}
			if entries, e := os.ReadDir(cur); e == nil {
				for _, i := range entries {
					names[i.Name()] = true
				}
			}
			dirs[cur] = names
		}
		if !names[part] {
			for name := range names {
				if strings.EqualFold(name, part) {
					return "case differs"
				}
			}
			return "missing"
		}
		cur = filepath.Join(cur, part)
		i, e := os.Lstat(cur)
		if e != nil {
			return "missing"
		}
		if i.Mode()&os.ModeSymlink != 0 {
			return "symlink"
		}
	}
	return ""
}
