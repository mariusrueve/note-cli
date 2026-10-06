package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mariusrueve/note-cli/internal/errs"
	"golang.org/x/text/unicode/norm"
)

func Syntax(path string, creating bool) error {
	if path == "" || filepath.IsAbs(path) || !utf8.ValidString(path) || strings.ContainsRune(path, 0) {
		return errs.New(2, "invalid root-relative note path %q", path)
	}
	for _, p := range strings.Split(path, "/") {
		if p == "" || p == "." || p == ".." || p == ".git" {
			return errs.New(2, "invalid path segment in %q", path)
		}
	}
	if creating && strings.IndexFunc(path, unicode.IsControl) >= 0 {
		return errs.New(2, "new paths cannot contain control characters")
	}
	return nil
}

// Safe rechecks every existing component, allowing only a missing creation tail.
func Safe(root, rel string, missing bool) (string, error) {
	if e := Syntax(rel, false); e != nil {
		return "", e
	}
	abs := filepath.Join(root, filepath.FromSlash(rel))
	r, e := filepath.Rel(root, abs)
	if e != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", errs.New(2, "path is outside root")
	}
	cur := root
	parts := strings.Split(rel, "/")
	for n, p := range parts {
		cur = filepath.Join(cur, p)
		i, e := os.Lstat(cur)
		if os.IsNotExist(e) && missing {
			return abs, nil
		}
		if e != nil {
			return "", e
		}
		if i.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink paths are unsupported: %s", rel)
		}
		if n < len(parts)-1 {
			if !i.IsDir() {
				return "", fmt.Errorf("parent is not a directory: %s", cur)
			}
			if _, e := os.Lstat(filepath.Join(cur, ".git")); e == nil {
				return "", fmt.Errorf("nested repository is unsupported: %s", cur)
			}
		} else if !i.Mode().IsRegular() {
			return "", fmt.Errorf("note must be a regular file: %s", rel)
		}
	}
	return abs, nil
}
func Markdown(path string) bool { return strings.EqualFold(filepath.Ext(path), ".md") }
func WithoutMD(s string) string {
	if Markdown(s) {
		return s[:len(s)-3]
	}
	return s
}
func NewPath(input string) (string, error) {
	if e := Syntax(input, true); e != nil {
		return "", e
	}
	name := WithoutMD(filepath.Base(input))
	name = strings.NewReplacer("ä", "ae", "Ä", "Ae", "ö", "oe", "Ö", "Oe", "ü", "ue", "Ü", "Ue", "ß", "ss", "ẞ", "SS").Replace(name)
	var b strings.Builder
	hyphen := false
	for _, r := range norm.NFKD.String(name) {
		if unicode.Is(unicode.M, r) {
			continue
		}
		r = unicode.ToLower(r)
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if hyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			hyphen = false
		} else {
			hyphen = true
		}
	}
	if b.Len() == 0 {
		return "", errs.New(2, "basename has no ASCII letters or digits; supply an ASCII basename and use --title for the heading")
	}
	dir := filepath.Dir(input)
	if dir == "." {
		return b.String() + ".md", nil
	}
	return filepath.ToSlash(filepath.Join(dir, b.String()+".md")), nil
}
func Title(input string) string {
	s := WithoutMD(filepath.Base(input))
	s = strings.NewReplacer("-", " ", "_", " ").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	for i, ch := range r {
		if unicode.IsLetter(ch) {
			r[i] = unicode.ToUpper(ch)
			break
		}
	}
	return string(r)
}

// Collision returns the actual legacy spelling and refuses ambiguous siblings.
func Collision(root, rel string) (string, bool, error) {
	if _, e := Safe(root, rel, true); e != nil {
		return "", false, e
	}
	entries, e := os.ReadDir(filepath.Join(root, filepath.Dir(rel)))
	if os.IsNotExist(e) {
		return rel, false, nil
	}
	if e != nil {
		return "", false, e
	}
	var matches []string
	for _, i := range entries {
		if strings.EqualFold(i.Name(), filepath.Base(rel)) {
			matches = append(matches, filepath.ToSlash(filepath.Join(filepath.Dir(rel), i.Name())))
		}
	}
	if len(matches) > 1 {
		return "", false, fmt.Errorf("ambiguous case-insensitive basename collision for %s: %q", rel, matches)
	}
	if len(matches) == 1 {
		if _, e = Safe(root, matches[0], false); e != nil {
			return "", false, e
		}
		return matches[0], true, nil
	}
	return rel, false, nil
}
