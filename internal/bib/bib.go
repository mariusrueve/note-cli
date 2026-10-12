// Package bib reads BibTeX/BibLaTeX files, such as Better BibTeX exports, without
// modifying them. It supports what bibliography managers emit: braced, quoted,
// and bare values, # concatenation, and skipped @comment/@string/@preamble blocks.
package bib

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type Entry struct {
	Type, Key string
	Fields    map[string]string // lowercase names; raw values without outer delimiters
}

const maxFile = 64 << 20

func Load(path string) ([]Entry, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, fmt.Errorf("bibliography: %w", e)
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, maxFile+1))
	if e != nil {
		return nil, fmt.Errorf("bibliography: %w", e)
	}
	if len(b) > maxFile {
		return nil, fmt.Errorf("bibliography %s is larger than 64 MiB", path)
	}
	entries, e := Parse(string(b))
	if e != nil {
		return nil, fmt.Errorf("bibliography %s: %w", path, e)
	}
	return entries, nil
}

type parser struct {
	src string
	i   int
}

func (p *parser) fail(format string, v ...any) error {
	return fmt.Errorf("line %d: %s", strings.Count(p.src[:p.i], "\n")+1, fmt.Sprintf(format, v...))
}
func (p *parser) space() {
	for p.i < len(p.src) && unicode.IsSpace(rune(p.src[p.i])) {
		p.i++
	}
}
func (p *parser) name() string {
	start := p.i
	for p.i < len(p.src) && (isLetter(p.src[p.i]) || strings.IndexByte("0123456789_-:.+/", p.src[p.i]) >= 0) {
		p.i++
	}
	return p.src[start:p.i]
}
func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// braced returns the text inside balanced braces starting at an opening brace.
func (p *parser) braced() (string, error) {
	start, depth := p.i, 0
	for ; p.i < len(p.src); p.i++ {
		switch p.src[p.i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				p.i++
				return p.src[start+1 : p.i-1], nil
			}
		}
	}
	p.i = start
	return "", p.fail("unbalanced braces")
}

func Parse(src string) ([]Entry, error) {
	p := &parser{src: src}
	out := []Entry{}
	seen := map[string]bool{}
	for {
		at := strings.IndexByte(p.src[p.i:], '@')
		if at < 0 {
			return out, nil
		}
		p.i += at + 1
		typ := strings.ToLower(p.name())
		p.space()
		if p.i >= len(p.src) || p.src[p.i] != '{' && p.src[p.i] != '(' {
			return nil, p.fail("expected { after @%s", typ)
		}
		closer := byte('}')
		if p.src[p.i] == '(' {
			closer = ')'
		}
		if typ == "comment" || typ == "string" || typ == "preamble" {
			if closer == ')' {
				end := strings.IndexByte(p.src[p.i:], ')')
				if end < 0 {
					return nil, p.fail("unterminated @%s", typ)
				}
				p.i += end + 1
			} else if _, e := p.braced(); e != nil {
				return nil, e
			}
			continue
		}
		p.i++
		p.space()
		start := p.i
		for p.i < len(p.src) && p.src[p.i] != ',' && p.src[p.i] != closer && !unicode.IsSpace(rune(p.src[p.i])) {
			p.i++
		}
		key := p.src[start:p.i]
		p.space()
		if key == "" || p.i >= len(p.src) || p.src[p.i] != ',' && p.src[p.i] != closer {
			return nil, p.fail("invalid citation key")
		}
		if seen[key] {
			return nil, p.fail("duplicate citation key %q", key)
		}
		seen[key] = true
		entry := Entry{Type: typ, Key: key, Fields: map[string]string{}}
		for {
			for p.i < len(p.src) && (p.src[p.i] == ',' || unicode.IsSpace(rune(p.src[p.i]))) {
				p.i++
			}
			if p.i >= len(p.src) {
				return nil, p.fail("unterminated entry %q", key)
			}
			if p.src[p.i] == closer {
				p.i++
				break
			}
			field := strings.ToLower(p.name())
			p.space()
			if field == "" || p.i >= len(p.src) || p.src[p.i] != '=' {
				return nil, p.fail("expected field = value in %q", key)
			}
			p.i++
			v, e := p.value(closer)
			if e != nil {
				return nil, e
			}
			entry.Fields[field] = v
		}
		out = append(out, entry)
	}
}

// value reads one field value, joining # concatenations.
func (p *parser) value(closer byte) (string, error) {
	var b strings.Builder
	for {
		p.space()
		if p.i >= len(p.src) {
			return "", p.fail("missing value")
		}
		switch c := p.src[p.i]; {
		case c == '{':
			s, e := p.braced()
			if e != nil {
				return "", e
			}
			b.WriteString(s)
		case c == '"':
			start, depth := p.i, 0
			for p.i++; p.i < len(p.src) && !(p.src[p.i] == '"' && depth == 0); p.i++ {
				if p.src[p.i] == '{' {
					depth++
				} else if p.src[p.i] == '}' {
					depth--
				}
			}
			if p.i >= len(p.src) {
				p.i = start
				return "", p.fail("unterminated quoted value")
			}
			b.WriteString(p.src[start+1 : p.i])
			p.i++
		default:
			s := p.name()
			if s == "" {
				return "", p.fail("invalid value")
			}
			b.WriteString(s)
		}
		p.space()
		if p.i < len(p.src) && p.src[p.i] == '#' {
			p.i++
			continue
		}
		if p.i < len(p.src) && p.src[p.i] != ',' && p.src[p.i] != closer {
			return "", p.fail("expected , or end of entry")
		}
		return b.String(), nil
	}
}

// accents map LaTeX accent commands to Unicode combining marks.
var accents = map[string]rune{`"`: '\u0308', "'": '\u0301', "`": '\u0300', "^": '\u0302', "~": '\u0303', "=": '\u0304', ".": '\u0307', "c": '\u0327', "v": '\u030c', "u": '\u0306', "H": '\u030b', "k": '\u0328', "r": '\u030a'}

var named = map[string]string{"textasciitilde": "~", "textbackslash": `\`, "ss": "ß", "ae": "æ", "AE": "Æ", "oe": "œ", "OE": "Œ", "aa": "å", "AA": "Å", "o": "ø", "O": "Ø", "l": "ł", "L": "Ł", "i": "ı"}

// Clean turns a LaTeX-ish value into plain text: protective braces go away,
// escaped specials become literal, accent commands keep their base letter,
// ties become spaces, and whitespace collapses.
func Clean(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && strings.IndexByte(`&%$#_{}`, s[i+1]) >= 0:
			b.WriteByte(s[i+1])
			i++
		case c == '\\' && i+1 < len(s) && isLetter(s[i+1]):
			j := i + 1
			for j < len(s) && isLetter(s[j]) {
				j++
			}
			if mark, ok := accents[s[i+1:j]]; ok && j-i == 2 && j < len(s) && (s[j] == '{' || s[j] == ' ') {
				i = accent(&b, s, j, mark)
				continue
			}
			b.WriteString(named[s[i+1:j]])
			if strings.HasPrefix(s[j:], "{}") {
				j += 2
			} else if j < len(s) && s[j] == ' ' {
				j++
			}
			i = j - 1
		case c == '\\' && i+1 < len(s):
			if mark, ok := accents[s[i+1:i+2]]; ok {
				i = accent(&b, s, i+2, mark)
			} else {
				i++
			}
		case c == '{' || c == '}':
		case c == '~':
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}
	return norm.NFC.String(strings.Join(strings.Fields(b.String()), " "))
}

// accent writes the base character at or after s[j] (skipping braces/spaces)
// followed by mark, and returns the index of the last consumed byte.
func accent(b *strings.Builder, s string, j int, mark rune) int {
	for j < len(s) && (s[j] == '{' || s[j] == ' ') {
		j++
	}
	if j >= len(s) {
		return j - 1
	}
	if s[j] == '\\' && j+1 < len(s) && (s[j+1] == 'i' || s[j+1] == 'j') {
		b.WriteByte(s[j+1]) // dotless i/j under an accent
		j += 2
	} else {
		r, n := utf8.DecodeRuneInString(s[j:])
		b.WriteRune(r)
		j += n
	}
	b.WriteRune(mark)
	return j - 1
}

// Authors renders "Last, First and {Org}" as "First Last, Org".
func Authors(s string) string {
	var names []string
	for _, a := range splitTop(s, " and ") {
		parts := splitTop(a, ",")
		if len(parts) > 1 {
			// Last, First or Last, Jr, First
			name := append([]string{parts[len(parts)-1], parts[0]}, parts[1:len(parts)-1]...)
			a = strings.Join(name, " ")
		}
		if a = Clean(a); a != "" {
			names = append(names, a)
		}
	}
	return strings.Join(names, ", ")
}

// splitTop splits on sep outside braces, case-insensitively.
func splitTop(s, sep string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
		}
		if depth == 0 && i+len(sep) <= len(s) && strings.EqualFold(s[i:i+len(sep)], sep) {
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	return append(out, strings.TrimSpace(s[start:]))
}

// Values are the template placeholders for an entry.
func (e Entry) Values() map[string]string {
	year := Clean(e.Fields["year"])
	if d := strings.TrimSpace(e.Fields["date"]); year == "" && len(d) >= 4 {
		year = d[:4]
	}
	venue := ""
	for _, f := range []string{"journaltitle", "journal", "booktitle", "publisher", "eprinttype"} {
		if venue = Clean(e.Fields[f]); venue != "" {
			break
		}
	}
	return map[string]string{
		"citekey":  e.Key,
		"venue":    venue,
		"abstract": Clean(e.Fields["abstract"]),
		"title":    Clean(e.Fields["title"]),
		"authors":  Authors(e.Fields["author"]),
		"year":     year,
		"doi":      strings.TrimSpace(e.Fields["doi"]),
		"url":      strings.TrimSpace(e.Fields["url"]),
	}
}

// Find returns the entry with exactly this key.
func Find(entries []Entry, key string) (Entry, bool) {
	for _, e := range entries {
		if e.Key == key {
			return e, true
		}
	}
	return Entry{}, false
}
