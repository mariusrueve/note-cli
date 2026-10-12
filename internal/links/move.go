package links

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mariusrueve/note-cli/internal/notes"
	"github.com/mariusrueve/note-cli/internal/search"
)

// Edit replaces one link target. Path is the file's root-relative path after
// the move, so edits to the moved note's own links name the new path.
type Edit struct {
	Path  string `json:"path"`
	Line  int    `json:"line"`
	Old   string `json:"old"`
	New   string `json:"new"`
	start int
	end   int
}

// Plan is a note move with the link edits it needs. Contents holds each edited
// file's bytes as read during planning, keyed by its pre-move path.
type Plan struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Edits    []Edit `json:"edits"`
	contents map[string]string
}

// Relative returns the link path from the note at from to the root-relative target.
func Relative(from, target string) string {
	r, e := filepath.Rel(filepath.FromSlash(path.Dir(from)), filepath.FromSlash(target))
	if e != nil {
		return target
	}
	return filepath.ToSlash(r)
}

// retarget rewrites a target as written so it points at rel from the note at
// from, keeping root style, fragment, and query. Spaces are escaped outside
// angle form; percent signs are always escaped because resolution decodes them.
func retarget(l Link, from, rel string) string {
	suffix := ""
	if i := strings.IndexAny(l.Target, "#?"); i >= 0 {
		suffix = l.Target[i:]
	}
	p := Relative(from, rel)
	if strings.HasPrefix(l.Target, "/") {
		p = "/" + rel
	}
	p = strings.ReplaceAll(p, "%", "%25")
	if !l.Angle {
		p = strings.ReplaceAll(p, " ", "%20")
	}
	return p + suffix
}

// PlanMove finds inbound links to oldRel in all discovered notes and the moved
// note's own relative links, and computes their targets after moving to newRel.
func PlanMove(root string, all []search.Result, oldRel, newRel string, warn io.Writer) (Plan, error) {
	plan := Plan{From: oldRel, To: newRel, Edits: []Edit{}, contents: map[string]string{}}
	sources := append([]search.Result{}, all...)
	found := false
	for _, r := range all {
		found = found || r.Path == oldRel
	}
	if !found {
		sources = append(sources, search.Listing(oldRel))
	}
	for _, r := range sources {
		p, e := notes.Safe(root, r.Path, false)
		var b []byte
		if e == nil {
			b, e = read(p)
		}
		if e != nil {
			if r.Path == oldRel {
				return plan, e
			}
			fmt.Fprintf(warn, "Warning: not updating links in %s: %v\n", search.Visible(r.Path), e)
			continue
		}
		after := r.Path
		if r.Path == oldRel {
			after = newRel
		}
		edited := false
		for _, l := range Extract(r.Path, string(b)) {
			target := ""
			switch {
			case l.Rel == oldRel:
				target = retarget(l, after, newRel)
			case r.Path == oldRel && l.Rel != "" && !strings.HasPrefix(l.Target, "/"):
				target = retarget(l, after, l.Rel)
			}
			if target == "" || target == l.Target {
				continue
			}
			plan.Edits = append(plan.Edits, Edit{Path: after, Line: l.Line, Old: l.Target, New: target, start: l.Start, end: l.End})
			edited = true
		}
		if edited {
			plan.contents[r.Path] = string(b)
		}
	}
	sort.SliceStable(plan.Edits, func(i, j int) bool {
		if plan.Edits[i].Path != plan.Edits[j].Path {
			return plan.Edits[i].Path < plan.Edits[j].Path
		}
		return plan.Edits[i].Line < plan.Edits[j].Line
	})
	return plan, nil
}

// Unchanged confirms that every file to edit still has its planned contents.
func (p Plan) Unchanged(root string) error {
	for rel, want := range p.contents {
		abs, e := notes.Safe(root, rel, false)
		var b []byte
		if e == nil {
			b, e = read(abs)
		}
		if e != nil || string(b) != want {
			return fmt.Errorf("%s changed while planning the move; nothing was moved", search.Visible(rel))
		}
	}
	return nil
}

// Apply rewrites the planned links after the note itself has moved. It returns
// the files it updated, so a failure can report exactly what changed.
func (p Plan) Apply(root string) ([]string, error) {
	byFile := map[string][]Edit{}
	for _, e := range p.Edits {
		byFile[e.Path] = append(byFile[e.Path], e)
	}
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	var done []string
	for _, f := range files {
		before := f
		if f == p.To {
			before = p.From
		}
		if e := rewrite(root, f, p.contents[before], byFile[f]); e != nil {
			return done, fmt.Errorf("updating %s: %w", search.Visible(f), e)
		}
		done = append(done, f)
	}
	return done, nil
}

// rewrite applies edits to src line by line, from the end of each line backwards,
// and atomically replaces the file through a temporary file in its directory.
func rewrite(root, rel, src string, edits []Edit) error {
	abs, e := notes.Safe(root, rel, false)
	if e != nil {
		return e
	}
	info, e := os.Lstat(abs)
	if e != nil {
		return e
	}
	lines := strings.Split(src, "\n")
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].Line != edits[j].Line {
			return edits[i].Line < edits[j].Line
		}
		return edits[i].start > edits[j].start
	})
	for _, ed := range edits {
		l := lines[ed.Line-1]
		if ed.end > len(l) || l[ed.start:ed.end] != ed.Old {
			return fmt.Errorf("line %d no longer matches", ed.Line)
		}
		lines[ed.Line-1] = l[:ed.start] + ed.New + l[ed.end:]
	}
	tmp, e := os.CreateTemp(filepath.Dir(abs), ".note-move-*")
	if e != nil {
		return e
	}
	defer os.Remove(tmp.Name())
	_, we := io.WriteString(tmp, strings.Join(lines, "\n"))
	ce := tmp.Close()
	if we != nil {
		return we
	}
	if ce != nil {
		return ce
	}
	if e = os.Chmod(tmp.Name(), info.Mode().Perm()); e != nil {
		return e
	}
	return os.Rename(tmp.Name(), abs)
}
