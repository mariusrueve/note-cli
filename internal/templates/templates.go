package templates

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mariusrueve/note-cli/internal/config"
)

//go:embed defaults/*.md
var defaults embed.FS

type Info struct {
	Name, Source string
	Overrides    bool
}

func List(dir string) ([]Info, error) {
	entries, _ := defaults.ReadDir("defaults")
	m := map[string]Info{}
	for _, f := range entries {
		n := strings.TrimSuffix(f.Name(), ".md")
		m[n] = Info{n, "embedded", false}
	}
	u, e := os.ReadDir(dir)
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	for _, f := range u {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		n := strings.TrimSuffix(f.Name(), ".md")
		if !config.Identifier(n) {
			continue
		}
		if f.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlinked template is unsupported: %s", f.Name())
		}
		_, exists := m[n]
		m[n] = Info{n, filepath.Join(dir, f.Name()), exists}
	}
	out := make([]Info, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func Load(dir, name string) (string, error) {
	if !config.Identifier(name) {
		return "", fmt.Errorf("invalid template identifier %q", name)
	}
	p := filepath.Join(dir, name+".md")
	i, e := os.Lstat(p)
	var b []byte
	if e == nil {
		if !i.Mode().IsRegular() {
			return "", fmt.Errorf("template must be a regular non-symlink file: %s", p)
		}
		b, e = os.ReadFile(p)
	} else if os.IsNotExist(e) {
		b, e = defaults.ReadFile("defaults/" + name + ".md")
	}
	if e != nil {
		return "", fmt.Errorf("template %q: %w", name, e)
	}
	if !utf8.Valid(b) {
		return "", fmt.Errorf("template must be UTF-8: %s", name)
	}
	return string(b), nil
}
func Render(src string, values map[string]string) (string, error) {
	var b strings.Builder
	escaped := 0
	for i := 0; i < len(src); {
		if strings.HasPrefix(src[i:], `\{{`) {
			b.WriteString("{{")
			escaped++
			i += 3
			continue
		}
		if strings.HasPrefix(src[i:], "{{") {
			end := strings.Index(src[i+2:], "}}")
			if end < 0 {
				return "", fmt.Errorf("malformed template placeholder")
			}
			key := src[i+2 : i+2+end]
			v, ok := values[key]
			if !ok || key != "title" && key != "slug" && key != "path" {
				return "", fmt.Errorf("unknown template placeholder %q", key)
			}
			b.WriteString(v)
			i += end + 4
			continue
		}
		if strings.HasPrefix(src[i:], "}}") && escaped > 0 {
			b.WriteString("}}")
			escaped--
			i += 2
			continue
		}
		if strings.HasPrefix(src[i:], "}}") {
			return "", fmt.Errorf("malformed template placeholder closer")
		}
		b.WriteByte(src[i])
		i++
	}
	return b.String(), nil
}
