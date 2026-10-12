package bib

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const sample = `% Exported by a bibliography manager
@comment{jabref-meta: x {nested} y}
@string{jcim = "J. Chem. Inf. Model."}
@article{rogersExtendedConnectivityFingerprints2010,
  title = {Extended-{{Connectivity Fingerprints}}: {{A}} Tool \& {{Its}} 100\% {{Use}}},
  author = {Rogers, David and Hahn, Mathew and {Open Science Consortium} and M{\"u}ller, J{\"o}rg and Fran\c{c}ois, {\'E}mile and Dvo\v{r}{\'a}k, Anton\'{\i}n and Stra\ss{}e, Anna and King, Jr, Martin},
  date = {2010-05-24},
  journaltitle = jcim # " (online)",
  doi = {10.1021/ci100050t},
  url = {https://example.org/a_b%20c~d},
  pages = {742--754}
}

@online{arxiv2206,
  title = "[2206.12411] {{Sample}} \textasciitilde{} Matters",
  year = 2022,
  author = {Doe, Jane}
}
@book(plainKey, title = {No Authors})
`

func TestParseAndValues(t *testing.T) {
	entries, e := Parse(sample)
	if e != nil {
		t.Fatal(e)
	}
	var keys []string
	for _, x := range entries {
		keys = append(keys, x.Type+":"+x.Key)
	}
	if !reflect.DeepEqual(keys, []string{"article:rogersExtendedConnectivityFingerprints2010", "online:arxiv2206", "book:plainKey"}) {
		t.Fatal(keys)
	}
	got := entries[0].Values()
	want := map[string]string{
		"citekey": "rogersExtendedConnectivityFingerprints2010",
		"title":   "Extended-Connectivity Fingerprints: A Tool & Its 100% Use",
		"authors": "David Rogers, Mathew Hahn, Open Science Consortium, Jörg Müller, Émile François, Antonín Dvořák, Anna Straße, Martin King Jr",
		"year":    "2010",
		"doi":     "10.1021/ci100050t",
		"url":     "https://example.org/a_b%20c~d",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if j := entries[0].Fields["journaltitle"]; j != "jcim (online)" {
		t.Fatal(j)
	}
	if v := entries[1].Values(); v["title"] != "[2206.12411] Sample ~ Matters" || v["year"] != "2022" || v["authors"] != "Jane Doe" {
		t.Fatal(v)
	}
	if v := entries[2].Values(); v["authors"] != "" || v["year"] != "" || v["title"] != "No Authors" {
		t.Fatal(v)
	}
	if x, ok := Find(entries, "arxiv2206"); !ok || x.Key != "arxiv2206" {
		t.Fatal(x)
	}
	if _, ok := Find(entries, "ARXIV2206"); ok {
		t.Fatal("keys are case-sensitive")
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{
		"@article{a, title = {unbalanced}",
		"@article{a, title = {x}, @article{b}",
		"@article{a, title {x}}",
		"@article{a, title = \"open}",
		"@article{, title = {x}}",
		"@article{a}\n@book{a}",
		"@article x",
		"@article{a, title = {x} junk}",
	} {
		if _, e := Parse(src); e == nil {
			t.Fatalf("accepted %q", src)
		}
	}
	if _, e := Parse("@article{a,\n title = {x\n"); e == nil || !strings.Contains(e.Error(), "line 2") {
		t.Fatal(e)
	}
}

func TestLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "My Library.bib")
	os.WriteFile(p, []byte(sample), 0400)
	entries, e := Load(p)
	if e != nil || len(entries) != 3 {
		t.Fatal(entries, e)
	}
	after, _ := os.ReadFile(p)
	if string(after) != sample {
		t.Fatal("bibliography modified")
	}
	if _, e = Load(p + ".missing"); e == nil {
		t.Fatal("missing file")
	}
}
