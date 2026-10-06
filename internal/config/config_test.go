package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConfig(t *testing.T) {
	h := t.TempDir()
	p := Paths{Home: func() (string, error) { return h, nil }}
	for _, x := range []string{"", "relative", filepath.Join(h, "xdg")} {
		t.Setenv("XDG_CONFIG_HOME", x)
		b, e := p.Base()
		if e != nil {
			t.Fatal(e)
		}
		want := filepath.Join(h, ".config", "note")
		if filepath.IsAbs(x) {
			want = filepath.Join(x, "note")
		}
		if b != want {
			t.Fatalf("base %q != %q", b, want)
		}
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, "xdg"))
	f := filepath.Join(t.TempDir(), "config.toml")
	for _, tt := range []struct {
		name, body string
		bad        bool
	}{
		{"defaults", `root="./kb"`, false},
		{"argv", "root=\"~/Knowledge\"\neditor=[\"code\",\"--wait\"]\n[search]\ncase_sensitive=false", false},
		{"unknown", "root=\"kb\"\nwrong=true", true},
		{"nested unknown", "root=\"kb\"\n[tools]\nbad=[\"rg\"]", true},
		{"version", "root=\"kb\"\nschema_version=2", true},
		{"wrong root", "root=42", true},
		{"missing root", "editor=\"hx\"", true},
		{"empty argv", "root=\"kb\"\neditor=[]", true},
		{"bad argv", "root=\"kb\"\neditor=[42]", true},
		{"shell editor", "root=\"kb\"\neditor=\"code --wait\"", true},
		{"empty tool", "root=\"kb\"\n[tools]\nrg=[]", true},
		{"wrong tool", "root=\"kb\"\n[tools]\nrg=\"rg\"", true},
		{"empty default", "root=\"kb\"\ndefault_template=\"\"", true},
		{"relative templates", "root=\"kb\"\ntemplates_dir=\"../templates\"", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if e := os.WriteFile(f, []byte(tt.body), 0600); e != nil {
				t.Fatal(e)
			}
			c, e := p.Load(f)
			if (e != nil) != tt.bad {
				t.Fatalf("load: %v", e)
			}
			if e != nil {
				return
			}
			if !filepath.IsAbs(c.Root) || !filepath.IsAbs(c.TemplatesDir) {
				t.Fatal(c)
			}
			if tt.name == "defaults" {
				if !reflect.DeepEqual(c.Editor, []string{"hx"}) || c.Search.CaseSensitive {
					t.Fatal(c)
				}
				if c.TemplatesDir != filepath.Join(h, "xdg", "note", "templates") {
					t.Fatal(c.TemplatesDir)
				}
			}
			if tt.name == "argv" && c.Root != filepath.Join(h, "Knowledge") {
				t.Fatal(c.Root)
			}
		})
	}
}
func TestInit(t *testing.T) {
	h := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "relative")
	var w bytes.Buffer
	p := Paths{Home: func() (string, error) { return h, nil }, Warn: &w}
	if e := p.Init("", "~/Knowledge", "hx"); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(w.String(), "must be absolute") {
		t.Fatal(w.String())
	}
	f, _ := p.File("")
	before, _ := os.ReadFile(f)
	if e := p.Init("", "~/Knowledge", "hx"); e == nil {
		t.Fatal("overwrote config")
	}
	after, _ := os.ReadFile(f)
	if !bytes.Equal(before, after) {
		t.Fatal("config changed")
	}
	c, e := p.Load("")
	if e != nil {
		t.Fatal(e)
	}
	if e = CanonicalRoot(&c); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(c.Root, ".git")); !os.IsNotExist(e) {
		t.Fatal("init created Git data")
	}
}
