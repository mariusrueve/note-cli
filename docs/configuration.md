# Configuration

Both macOS and Linux use `$XDG_CONFIG_HOME/note/config.toml` and
`$XDG_CONFIG_HOME/note/templates/`, falling back to `~/.config/note/`.
An empty or relative XDG value uses the fallback; relative values produce a
warning outside completion. `note init --root ~/Knowledge --editor hx` creates
the root and a commented config exclusively. It never initializes Git.

`note --config /path/config.toml find docker` selects the config file only.
Templates still use XDG unless `templates_dir` is configured. Relative TOML paths
resolve against the config's directory; a leading `~` expands to the home
directory. Environment variables and shell syntax are not expanded. Ordinary
commands require the root to exist. The root itself may be a symlink, resolved
once; paths below it must not traverse symlinks.

See [examples/config.toml](../examples/config.toml) for all fields. `root` is
required, `schema_version` defaults to 1, and unknown keys/types or unsupported
versions are errors. Defaults are `hx`, template `default`, `fzf`, `rg`, `git`,
finder height `90%`, preview `right:60%:wrap`, case-insensitive search, and commit
message `notes: sync {timestamp}`. Explicit `false` overrides settings correctly.
Precedence is defaults, TOML, applicable CLI flags. `EDITOR` and `VISUAL` are not
used. Executable availability is checked only when needed by the command.

```toml
editor = ["code", "--wait", "--reuse-window"]
```

An editor string is one executable; use a list for arguments. Absolute executable
paths containing spaces are supported. Tool settings always use nonempty argv
lists. Processes run synchronously with direct arguments. GUI editors should use
their wait flag. Helix gets `absolute-path.md:LINE` for content hits, except when
the path contains a colon, when it gets the plain path.
