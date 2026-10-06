package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/pelletier/go-toml/v2"
)

type Tools struct {
	FZF []string `toml:"fzf"`
	RG  []string `toml:"rg"`
	Git []string `toml:"git"`
}
type Finder struct {
	Height        string `toml:"height"`
	PreviewWindow string `toml:"preview_window"`
}
type Search struct {
	CaseSensitive bool `toml:"case_sensitive"`
}
type Sync struct {
	CommitMessage string `toml:"commit_message"`
}
type Config struct {
	SchemaVersion   int      `toml:"schema_version"`
	Root            string   `toml:"root"`
	Editor          []string `toml:"-"`
	DefaultTemplate string   `toml:"default_template"`
	TemplatesDir    string   `toml:"templates_dir"`
	Tools           Tools    `toml:"tools"`
	Finder          Finder   `toml:"finder"`
	Search          Search   `toml:"search"`
	Sync            Sync     `toml:"sync"`
	File            string   `toml:"-"`
}
type disk struct {
	SchemaVersion   *int    `toml:"schema_version"`
	Root            *string `toml:"root"`
	Editor          any     `toml:"editor"`
	DefaultTemplate *string `toml:"default_template"`
	TemplatesDir    *string `toml:"templates_dir"`
	Tools           struct {
		FZF *[]string `toml:"fzf"`
		RG  *[]string `toml:"rg"`
		Git *[]string `toml:"git"`
	} `toml:"tools"`
	Finder struct {
		Height        *string `toml:"height"`
		PreviewWindow *string `toml:"preview_window"`
	} `toml:"finder"`
	Search Search `toml:"search"`
	Sync   struct {
		CommitMessage *string `toml:"commit_message"`
	} `toml:"sync"`
}

// Paths injects home resolution for tests without changing the process HOME.
type Paths struct {
	Home func() (string, error)
	Warn io.Writer
}

func (p Paths) home() (string, error) {
	if p.Home != nil {
		return p.Home()
	}
	return os.UserHomeDir()
}
func (p Paths) Base() (string, error) {
	x := os.Getenv("XDG_CONFIG_HOME")
	if filepath.IsAbs(x) {
		return filepath.Join(x, "note"), nil
	}
	if x != "" && p.Warn != nil {
		fmt.Fprintln(p.Warn, "Warning: XDG_CONFIG_HOME must be absolute; using ~/.config")
	}
	h, e := p.home()
	return filepath.Join(h, ".config", "note"), e
}
func (p Paths) Expand(s, base string) (string, error) {
	if s == "~" || strings.HasPrefix(s, "~/") {
		h, e := p.home()
		if e != nil {
			return "", e
		}
		if s == "~" {
			s = h
		} else {
			s = filepath.Join(h, strings.TrimPrefix(s, "~/"))
		}
	}
	if !filepath.IsAbs(s) {
		s = filepath.Join(base, s)
	}
	return filepath.Abs(s)
}
func (p Paths) File(override string) (string, error) {
	if override != "" {
		return p.Expand(override, ".")
	}
	b, e := p.Base()
	return filepath.Join(b, "config.toml"), e
}
func (p Paths) Load(override string) (Config, error) {
	var c Config
	f, e := p.File(override)
	if e != nil {
		return c, e
	}
	b, e := os.ReadFile(f)
	if e != nil {
		return c, fmt.Errorf("read config %s: %w; run note init", f, e)
	}
	var d disk
	if e = toml.NewDecoder(bytes.NewReader(b)).DisallowUnknownFields().Decode(&d); e != nil {
		return c, fmt.Errorf("config %s: %w", f, e)
	}
	base, e := p.Base()
	if e != nil {
		return c, e
	}
	c = Config{SchemaVersion: 1, Editor: []string{"hx"}, DefaultTemplate: "default", TemplatesDir: filepath.Join(base, "templates"), Tools: Tools{[]string{"fzf"}, []string{"rg"}, []string{"git"}}, Finder: Finder{"90%", "right:60%:wrap"}, Sync: Sync{"notes: sync {timestamp}"}, File: f, Search: d.Search}
	if d.SchemaVersion != nil {
		c.SchemaVersion = *d.SchemaVersion
	}
	if c.SchemaVersion != 1 {
		return c, fmt.Errorf("unsupported schema_version %d (supported: 1)", c.SchemaVersion)
	}
	if d.Root == nil || *d.Root == "" {
		return c, fmt.Errorf("config root is required")
	}
	c.Root, e = p.Expand(*d.Root, filepath.Dir(f))
	if e != nil {
		return c, e
	}
	if d.Editor != nil {
		switch v := d.Editor.(type) {
		case string:
			if strings.IndexFunc(v, unicode.IsSpace) >= 0 && !filepath.IsAbs(v) {
				return c, fmt.Errorf("editor string is one executable; use an argv list such as [\"code\", \"--wait\"]")
			}
			c.Editor = []string{v}
		case []any:
			c.Editor = nil
			for _, a := range v {
				s, ok := a.(string)
				if !ok {
					return c, fmt.Errorf("editor argv must contain strings")
				}
				c.Editor = append(c.Editor, s)
			}
		default:
			return c, fmt.Errorf("editor must be a string or nonempty argv list")
		}
	}
	if d.DefaultTemplate != nil {
		c.DefaultTemplate = *d.DefaultTemplate
	}
	if !Identifier(c.DefaultTemplate) {
		return c, fmt.Errorf("invalid default_template identifier")
	}
	if d.TemplatesDir != nil {
		if *d.TemplatesDir == "" {
			return c, fmt.Errorf("templates_dir cannot be empty")
		}
		c.TemplatesDir, e = p.Expand(*d.TemplatesDir, filepath.Dir(f))
		if e != nil {
			return c, e
		}
	}
	if d.Tools.FZF != nil {
		c.Tools.FZF = *d.Tools.FZF
	}
	if d.Tools.RG != nil {
		c.Tools.RG = *d.Tools.RG
	}
	if d.Tools.Git != nil {
		c.Tools.Git = *d.Tools.Git
	}
	for name, argv := range map[string][]string{"editor": c.Editor, "tools.fzf": c.Tools.FZF, "tools.rg": c.Tools.RG, "tools.git": c.Tools.Git} {
		if e = ValidateCommand(argv); e != nil {
			return c, fmt.Errorf("%s: %w", name, e)
		}
	}
	if d.Finder.Height != nil {
		c.Finder.Height = *d.Finder.Height
	}
	if d.Finder.PreviewWindow != nil {
		c.Finder.PreviewWindow = *d.Finder.PreviewWindow
	}
	if c.Finder.Height == "" || c.Finder.PreviewWindow == "" || strings.IndexFunc(c.Finder.Height+c.Finder.PreviewWindow, unicode.IsControl) >= 0 {
		return c, fmt.Errorf("finder settings must be nonempty and contain no control characters")
	}
	if d.Sync.CommitMessage != nil {
		c.Sync.CommitMessage = *d.Sync.CommitMessage
	}
	if strings.TrimSpace(c.Sync.CommitMessage) == "" {
		return c, fmt.Errorf("sync.commit_message cannot be empty")
	}
	return c, nil
}
func ValidateCommand(argv []string) error {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return fmt.Errorf("command must be a nonempty argv list with an executable")
	}
	for _, a := range argv {
		if strings.ContainsRune(a, 0) {
			return fmt.Errorf("command arguments cannot contain NUL")
		}
	}
	return nil
}
func Identifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}
func CanonicalRoot(c *Config) error {
	r, e := filepath.EvalSymlinks(c.Root)
	if e != nil {
		return fmt.Errorf("knowledge root %s: %w; only note init creates a root", c.Root, e)
	}
	i, e := os.Stat(r)
	if e != nil {
		return e
	}
	if !i.IsDir() {
		return fmt.Errorf("knowledge root must be a directory")
	}
	c.Root = r
	return nil
}

func (p Paths) Init(override, root, editor string) error {
	f, e := p.File(override)
	if e != nil {
		return e
	}
	if _, e = os.Lstat(f); e == nil {
		return fmt.Errorf("configuration already exists: %s", f)
	} else if !os.IsNotExist(e) {
		return e
	}
	if e = ValidateCommand([]string{editor}); e != nil {
		return e
	}
	if strings.IndexFunc(editor, unicode.IsSpace) >= 0 && !filepath.IsAbs(editor) {
		return fmt.Errorf("--editor takes one executable")
	}
	r, e := p.Expand(root, ".")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(r, 0755); e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(f), 0755); e != nil {
		return e
	}
	b, e := toml.Marshal(map[string]any{"schema_version": 1, "root": r, "editor": editor, "default_template": "default"})
	if e != nil {
		return e
	}
	b = append([]byte("# note configuration; see docs/configuration.md for optional settings.\n# Editor arguments use a list, for example: [\"code\", \"--wait\"].\n"), b...)
	file, e := os.OpenFile(f, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	_, we := file.Write(b)
	ce := file.Close()
	if we != nil {
		os.Remove(f)
		return we
	}
	if ce != nil {
		os.Remove(f)
		return ce
	}
	return nil
}
