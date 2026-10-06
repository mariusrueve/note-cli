package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mariusrueve/note-cli/internal/config"
	"github.com/mariusrueve/note-cli/internal/errs"
	"github.com/mariusrueve/note-cli/internal/notes"
	"github.com/mariusrueve/note-cli/internal/picker"
	"github.com/mariusrueve/note-cli/internal/process"
	"github.com/mariusrueve/note-cli/internal/search"
	"github.com/mariusrueve/note-cli/internal/syncgit"
	"github.com/mariusrueve/note-cli/internal/templates"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type Options struct {
	In              io.Reader
	Out, Err        io.Writer
	Interactive     func() bool
	Home            func() (string, error)
	Executable      string
	Version, Commit string
}
type app struct {
	Options
	configPath string
	reader     *bufio.Reader
}

func New(o Options) *cobra.Command {
	if o.In == nil {
		o.In = os.Stdin
	}
	if o.Out == nil {
		o.Out = os.Stdout
	}
	if o.Err == nil {
		o.Err = os.Stderr
	}
	if o.Interactive == nil {
		o.Interactive = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) }
	}
	if o.Version == "" {
		o.Version = "dev"
	}
	if o.Commit == "" {
		o.Commit = "unknown"
	}
	a := &app{Options: o, reader: bufio.NewReader(o.In)}
	root := &cobra.Command{Use: "note [QUERY]", Short: "Create, find, open, and sync a Markdown knowledge base", Version: o.Version + " (" + o.Commit + ")", SilenceUsage: true, SilenceErrors: true, Args: args(0, 1), RunE: func(cmd *cobra.Command, v []string) error {
		if len(v) > 0 {
			if e := query(v[0]); e != nil {
				return e
			}
		}
		c, e := a.load()
		if e != nil {
			return e
		}
		if len(v) == 0 {
			return a.browse(cmd.Context(), c)
		}
		return a.lookup(cmd.Context(), c, v[0])
	}, ValidArgsFunction: a.completePaths}
	root.SetIn(o.In)
	root.SetOut(o.Out)
	root.SetErr(o.Err)
	root.SetFlagErrorFunc(func(_ *cobra.Command, e error) error { return errs.Wrap(2, e) })
	root.PersistentFlags().StringVar(&a.configPath, "config", "", "Configuration file (default: XDG_CONFIG_HOME/note/config.toml)")
	root.CompletionOptions.DisableDefaultCmd = true
	initCmd := &cobra.Command{Use: "init", Short: "Create a root and configuration without initializing Git", Args: args(0, 0)}
	var initRoot, editor string
	initCmd.Flags().StringVar(&initRoot, "root", "~/Knowledge", "Knowledge root")
	initCmd.Flags().StringVar(&editor, "editor", "hx", "Editor executable")
	initCmd.RunE = func(_ *cobra.Command, _ []string) error {
		if e := a.paths(false).Init(a.configPath, initRoot, editor); e != nil {
			return e
		}
		fmt.Fprintln(a.Err, "Initialized note configuration and root.")
		return nil
	}
	open := &cobra.Command{Use: "open [PATH]", Short: "Open an existing Markdown note, or select one", Args: args(0, 1), ValidArgsFunction: a.completePaths, RunE: func(cmd *cobra.Command, v []string) error {
		if len(v) > 0 {
			if e := notes.Syntax(v[0], false); e != nil {
				return e
			}
		}
		c, e := a.load()
		if e != nil {
			return e
		}
		if len(v) == 0 {
			return a.browse(cmd.Context(), c)
		}
		r, exists, e := exact(c.Root, v[0])
		if e != nil {
			return e
		}
		if !exists {
			return errs.New(1, "note does not exist: %s", search.Visible(v[0]))
		}
		return a.edit(cmd.Context(), c, r)
	}}
	newCmd := &cobra.Command{Use: "new PATH", Short: "Create a note exclusively and open it", Args: args(1, 1), ValidArgsFunction: a.completePaths}
	var template, title string
	var noOpen, openExisting bool
	newCmd.Flags().StringVar(&template, "template", "", "Template name")
	newCmd.Flags().StringVar(&title, "title", "", "Heading title (preserves Unicode)")
	newCmd.Flags().BoolVar(&noOpen, "no-open", false, "Create without opening an editor")
	newCmd.Flags().BoolVar(&openExisting, "open-existing", false, "Explicitly open an existing note without prompting")
	newCmd.RunE = func(cmd *cobra.Command, v []string) error {
		if cmd.Flags().Changed("template") && !config.Identifier(template) {
			return errs.New(2, "invalid template identifier")
		}
		if noOpen && openExisting {
			return errs.New(2, "--no-open and --open-existing are mutually exclusive")
		}
		if _, e := notes.NewPath(v[0]); e != nil {
			return e
		}
		c, e := a.load()
		if e != nil {
			return e
		}
		return a.create(cmd.Context(), c, v[0], template, title, cmd.Flags().Changed("title"), noOpen, openExisting)
	}
	newCmd.RegisterFlagCompletionFunc("template", a.completeTemplates)
	tmpl := &cobra.Command{Use: "templates", Short: "List templates, sources, overrides, and the default", Args: args(0, 0), RunE: func(_ *cobra.Command, _ []string) error {
		c, e := a.load()
		if e != nil {
			return e
		}
		list, e := templates.List(c.TemplatesDir)
		if e != nil {
			return e
		}
		for _, t := range list {
			suffix := ""
			if t.Overrides {
				suffix += " (overrides embedded)"
			}
			if t.Name == c.DefaultTemplate {
				suffix += " [default]"
			}
			fmt.Fprintf(a.Out, "%s\t%s%s\n", t.Name, search.Visible(t.Source), suffix)
		}
		return nil
	}}
	completion := &cobra.Command{Use: "completion SHELL", Short: "Print a Bash, Zsh, or Fish completion script", Args: args(1, 1), ValidArgs: []string{"bash", "zsh", "fish"}, RunE: func(cmd *cobra.Command, v []string) error {
		switch v[0] {
		case "bash":
			return cmd.Root().GenBashCompletionV2(a.Out, true)
		case "zsh":
			return cmd.Root().GenZshCompletion(a.Out)
		case "fish":
			return cmd.Root().GenFishCompletion(a.Out, true)
		default:
			return errs.New(2, "unsupported shell %q; use bash, zsh, or fish", v[0])
		}
	}}
	preview := &cobra.Command{Use: "__preview", Hidden: true, Args: args(0, 0)}
	var manifest string
	var id int
	preview.Flags().StringVar(&manifest, "manifest", "", "Manifest path")
	preview.Flags().IntVar(&id, "id", 0, "Record ID")
	preview.RunE = func(_ *cobra.Command, _ []string) error { return picker.Preview(manifest, id, a.Out) }
	doctor := &cobra.Command{Use: "doctor", Short: "Check configuration, tools, and completion prerequisites", Args: args(0, 0), RunE: func(cmd *cobra.Command, _ []string) error { return a.doctor(cmd.Context()) }}
	syncCmd := &cobra.Command{Use: "sync", Short: "Commit, fetch/rebase, and push a dedicated knowledge repository", Args: args(0, 0)}
	var message string
	syncCmd.Flags().StringVar(&message, "message", "", "Literal sync commit message override")
	syncCmd.RunE = func(cmd *cobra.Command, _ []string) error {
		var override *string
		if cmd.Flags().Changed("message") {
			if strings.TrimSpace(message) == "" {
				return errs.New(2, "--message cannot be empty")
			}
			override = &message
		}
		c, e := a.load()
		if e != nil {
			return e
		}
		return syncgit.Run(cmd.Context(), c, syncgit.Options{In: a.In, Err: a.Err, Message: override})
	}
	root.AddCommand(initCmd, open, newCmd, tmpl, completion, preview, doctor, syncCmd)
	for _, kind := range []string{"find", "grep", "search", "recent"} {
		root.AddCommand(a.resultCommand(kind))
	}
	return root
}
func args(min, max int) cobra.PositionalArgs {
	return func(_ *cobra.Command, v []string) error {
		if len(v) < min || len(v) > max {
			return errs.New(2, "expected %d to %d arguments, got %d", min, max, len(v))
		}
		return nil
	}
}
func query(q string) error {
	if q == "" || strings.ContainsAny(q, "\r\n") || strings.ContainsRune(q, 0) {
		return errs.New(2, "query must be nonempty and contain no line breaks or NUL")
	}
	return nil
}
func (a *app) paths(quiet bool) config.Paths {
	p := config.Paths{Home: a.Home}
	if !quiet {
		p.Warn = a.Err
	}
	return p
}
func (a *app) load() (config.Config, error) {
	c, e := a.paths(false).Load(a.configPath)
	if e != nil {
		return c, e
	}
	e = config.CanonicalRoot(&c)
	return c, e
}
func (a *app) edit(ctx context.Context, c config.Config, r search.Result) error {
	p, e := notes.Safe(c.Root, r.Path, false)
	if e != nil {
		return e
	}
	if _, e = exec.LookPath(c.Editor[0]); e != nil {
		return fmt.Errorf("editor %q: %w; note remains at %s", c.Editor[0], e, search.Visible(r.Path))
	}
	if filepath.Base(c.Editor[0]) == "hx" && r.Line != nil && !strings.Contains(p, ":") {
		p = fmt.Sprintf("%s:%d", p, *r.Line)
	}
	cmd := process.Command(ctx, c.Editor, p)
	cmd.Stdin = a.In
	cmd.Stdout = a.Out
	cmd.Stderr = a.Err
	if e = cmd.Run(); ctx.Err() != nil {
		return ctx.Err()
	}
	if e != nil {
		return fmt.Errorf("editor: %w; note remains at %s; retry note open PATH", e, search.Visible(r.Path))
	}
	return nil
}
func exact(root, input string) (search.Result, bool, error) {
	if e := notes.Syntax(input, false); e != nil {
		return search.Result{}, false, e
	}
	paths := []string{input}
	if !notes.Markdown(input) {
		paths = []string{input + ".md"}
	}
	for _, rel := range paths {
		if _, e := notes.Safe(root, rel, false); e == nil {
			return search.Listing(rel), true, nil
		} else if !os.IsNotExist(e) {
			return search.Result{}, false, e
		}
	}
	return search.Result{}, false, nil
}
func (a *app) line(ctx context.Context) (string, error) {
	type response struct {
		s string
		e error
	}
	ch := make(chan response, 1)
	go func() { s, e := a.reader.ReadString('\n'); ch <- response{s, e} }()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		if r.e != nil {
			return "", errs.New(130, "prompt canceled")
		}
		return strings.TrimSuffix(strings.TrimSuffix(r.s, "\n"), "\r"), nil
	}
}
func (a *app) existing(ctx context.Context, c config.Config, rel string, noOpen, authorized bool) error {
	fmt.Fprintf(a.Err, "Note %s already exists.\n", search.Visible(rel))
	if noOpen || !a.Interactive() && !authorized {
		return errs.New(4, "existing note was preserved; use --open-existing or note open PATH")
	}
	if !authorized {
		for {
			fmt.Fprint(a.Err, "Press Enter to open it, or Ctrl-C to cancel: ")
			s, e := a.line(ctx)
			if e != nil {
				return e
			}
			if s == "" {
				break
			}
		}
	}
	return a.edit(ctx, c, search.Listing(rel))
}
func (a *app) create(ctx context.Context, c config.Config, input, template, title string, explicitTitle, noOpen, authorized bool) error {
	rel, e := notes.NewPath(input)
	if e != nil {
		return e
	}
	actual, exists, e := notes.Collision(c.Root, rel)
	if e != nil {
		return e
	}
	if exists {
		return a.existing(ctx, c, actual, noOpen, authorized)
	}
	if template == "" {
		template = c.DefaultTemplate
	}
	src, e := templates.Load(c.TemplatesDir, template)
	if e != nil {
		return e
	}
	if !explicitTitle {
		title = notes.Title(input)
	}
	contents, e := templates.Render(src, map[string]string{"title": title, "slug": notes.WithoutMD(filepath.Base(rel)), "path": rel})
	if e != nil {
		return e
	}
	if !utf8.ValidString(contents) {
		return errs.New(2, "new note content must be UTF-8; check --title")
	}
	if !noOpen {
		if _, e = exec.LookPath(c.Editor[0]); e != nil {
			return fmt.Errorf("editor %q: %w; no note was created", c.Editor[0], e)
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	p, e := notes.Safe(c.Root, rel, true)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		return e
	}
	actual, exists, e = notes.Collision(c.Root, rel)
	if e != nil {
		return e
	}
	if exists {
		return a.existing(ctx, c, actual, noOpen, authorized)
	}
	e = notes.Create(c.Root, rel, contents)
	if errors.Is(e, os.ErrExist) {
		actual, _, ce := notes.Collision(c.Root, rel)
		if ce != nil {
			return ce
		}
		return a.existing(ctx, c, actual, noOpen, authorized)
	}
	if e != nil {
		return e
	}
	fmt.Fprintf(a.Err, "Created %s\n", search.Visible(rel))
	if noOpen {
		return nil
	}
	return a.edit(ctx, c, search.Listing(rel))
}
func (a *app) browse(ctx context.Context, c config.Config) error {
	if !a.Interactive() {
		return errs.New(2, "note picker needs an interactive terminal; use note find QUERY --print or note recent --json")
	}
	all, e := search.Discover(ctx, c, a.Err)
	if e != nil {
		return e
	}
	return a.selectOpen(ctx, c, all, "list")
}
func (a *app) selectOpen(ctx context.Context, c config.Config, results []search.Result, kind string) error {
	if len(results) == 0 {
		return errs.New(1, "no notes found")
	}
	if !a.Interactive() {
		return errs.New(2, "selection needs an interactive terminal; use --print or --json")
	}
	r, e := picker.Pick(ctx, c, results, kind, a.Executable, a.Err)
	if e != nil {
		return e
	}
	return a.edit(ctx, c, r)
}
func (a *app) lookup(ctx context.Context, c config.Config, q string) error {
	if e := notes.Syntax(q, false); e != nil {
		return e
	}
	r, exists, e := exact(c.Root, q)
	if e != nil {
		return e
	}
	if exists {
		return a.edit(ctx, c, r)
	}
	all, e := search.Discover(ctx, c, a.Err)
	if e != nil {
		return e
	}
	matches := []search.Result{}
	if !strings.Contains(q, "/") {
		for _, r := range all {
			b := filepath.Base(r.Path)
			if search.Equal(b, q, c.Search.CaseSensitive) || search.Equal(notes.WithoutMD(b), q, c.Search.CaseSensitive) {
				matches = append(matches, r)
			}
		}
	}
	if len(matches) == 0 {
		matches = search.Find(all, q, c.Search.CaseSensitive)
	}
	if len(matches) == 1 {
		return a.edit(ctx, c, matches[0])
	}
	if len(matches) > 1 {
		return a.selectOpen(ctx, c, matches, "find")
	}
	if !a.Interactive() {
		return errs.New(1, "no matching note; create explicitly with note new PATH")
	}
	fmt.Fprint(a.Err, "Create a new note? [y/N] ")
	s, e := a.line(ctx)
	if e != nil {
		return e
	}
	s = strings.ToLower(strings.TrimSpace(s))
	if s != "y" && s != "yes" {
		return errs.New(130, "creation declined")
	}
	return a.create(ctx, c, q, "", "", false, false, false)
}
func (a *app) resultCommand(kind string) *cobra.Command {
	use := kind + " QUERY"
	min, max := 1, 1
	short := map[string]string{"find": "Search relative paths only", "grep": "Search note contents only", "search": "Search paths and contents, with each note once", "recent": "List recently modified notes"}[kind]
	if kind == "recent" {
		use = kind
		min, max = 0, 0
	}
	cmd := &cobra.Command{Use: use, Short: short, Args: args(min, max)}
	var print, json, sensitive bool
	var limit int
	cmd.Flags().BoolVar(&print, "print", false, "Print human-readable results")
	cmd.Flags().BoolVar(&json, "json", false, "Print a JSON array")
	if kind == "recent" {
		cmd.Flags().IntVar(&limit, "limit", 20, "Maximum notes (positive integer)")
	} else {
		cmd.Flags().BoolVar(&sensitive, "case-sensitive", false, "Use case-sensitive matching")
	}
	cmd.RunE = func(cmd *cobra.Command, v []string) error {
		if print && json {
			return errs.New(2, "--print and --json are mutually exclusive")
		}
		if kind == "recent" && limit <= 0 {
			return errs.New(2, "--limit must be positive")
		}
		if len(v) > 0 {
			if e := query(v[0]); e != nil {
				return e
			}
		}
		c, e := a.load()
		if e != nil {
			return e
		}
		if !cmd.Flags().Changed("case-sensitive") {
			sensitive = c.Search.CaseSensitive
		}
		all, e := search.Discover(cmd.Context(), c, a.Err)
		if e != nil {
			return e
		}
		var results []search.Result
		switch kind {
		case "recent":
			results = search.Recent(c.Root, all, limit, a.Err)
		case "find":
			results = search.Find(all, v[0], sensitive)
		case "grep":
			results, e = search.Grep(cmd.Context(), c, all, v[0], sensitive)
		case "search":
			var content []search.Result
			content, e = search.Grep(cmd.Context(), c, all, v[0], sensitive)
			results = search.Union(search.Find(all, v[0], sensitive), content)
		}
		if e != nil {
			return e
		}
		if print || json || !a.Interactive() {
			if e = search.Print(a.Out, kind, results, json); e != nil {
				return e
			}
			if len(results) == 0 {
				return errs.New(1, "no matches")
			}
			return nil
		}
		return a.selectOpen(cmd.Context(), c, results, kind)
	}
	return cmd
}
func (a *app) completePaths(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	directive := cobra.ShellCompDirectiveNoFileComp
	if len(args) > 0 {
		return nil, directive
	}
	c, e := a.paths(true).Load(a.configPath)
	if e != nil || config.CanonicalRoot(&c) != nil {
		return nil, directive
	}
	dir := filepath.ToSlash(filepath.Dir(prefix))
	if strings.HasSuffix(prefix, "/") {
		dir = strings.TrimSuffix(prefix, "/")
	}
	if dir == "." {
		dir = ""
	}
	if dir != "" {
		for _, part := range strings.Split(dir, "/") {
			if strings.HasPrefix(part, ".") {
				return nil, directive
			}
		}
		if notes.Syntax(dir, false) != nil {
			return nil, directive
		}
		if _, e = notes.Safe(c.Root, dir+"/__completion__", true); e != nil {
			return nil, directive
		}
	}
	entries, e := os.ReadDir(filepath.Join(c.Root, dir))
	if e != nil {
		return nil, directive
	}
	out := []string{}
	hasDir := false
	for _, i := range entries {
		if !i.IsDir() || strings.HasPrefix(i.Name(), ".") || i.Type()&os.ModeSymlink != 0 {
			continue
		}
		p := filepath.ToSlash(filepath.Join(dir, i.Name()))
		if strings.IndexFunc(p, unicode.IsControl) >= 0 {
			continue
		}
		if _, e = os.Lstat(filepath.Join(c.Root, p, ".git")); e == nil {
			continue
		}
		if strings.HasPrefix(p+"/", prefix) {
			out = append(out, p+"/")
			hasDir = true
		}
	}
	c.Tools.RG = append([]string{}, c.Tools.RG...)
	all, e := search.Discover(cmd.Context(), c, io.Discard)
	if e == nil {
		for _, r := range all {
			if strings.IndexFunc(r.Path, unicode.IsControl) >= 0 {
				continue
			}
			if filepath.ToSlash(filepath.Dir(r.Path)) == dir || dir == "" && filepath.Dir(r.Path) == "." {
				if strings.HasPrefix(r.Path, prefix) {
					out = append(out, r.Path)
				}
			}
		}
	}
	if hasDir {
		directive |= cobra.ShellCompDirectiveNoSpace
	}
	sort.Strings(out)
	return out, directive
}
func (a *app) completeTemplates(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
	dir := ""
	c, e := a.paths(true).Load(a.configPath)
	if e == nil {
		dir = c.TemplatesDir
	}
	list, e := templates.List(dir)
	if e != nil {
		list, _ = templates.List("")
	}
	out := []string{}
	for _, t := range list {
		if strings.HasPrefix(t.Name, prefix) {
			out = append(out, t.Name)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}
func (a *app) doctor(ctx context.Context) error {
	c, e := a.load()
	if e != nil {
		return e
	}
	fmt.Fprintf(a.Err, "Config: %s\nRoot: %s\n", search.Visible(c.File), search.Visible(c.Root))
	failed := false
	for _, tool := range []struct {
		name string
		argv []string
	}{{"editor", c.Editor}, {"ripgrep", c.Tools.RG}, {"fzf", c.Tools.FZF}, {"Git", c.Tools.Git}} {
		p, e := exec.LookPath(tool.argv[0])
		if e != nil {
			failed = true
			fmt.Fprintf(a.Err, "%s: missing %s\n", tool.name, search.Visible(tool.argv[0]))
		} else {
			fmt.Fprintf(a.Err, "%s: %s\n", tool.name, search.Visible(p))
		}
	}
	if state, e := syncgit.Check(ctx, c); e != nil {
		failed = true
		fmt.Fprintf(a.Err, "Git readiness: %v\n", e)
	} else {
		fmt.Fprintf(a.Err, "Git readiness: branch %s -> %s %s\n", state.Branch, state.Remote, state.Ref)
	}
	fmt.Fprintln(a.Err, "Completion: run note completion bash|zsh|fish; see docs/completion.md.")
	if failed {
		return errs.New(3, "some tools are missing; new --no-open needs no external tools")
	}
	return nil
}
