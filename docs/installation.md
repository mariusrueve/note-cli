# Installation and updates

Published releases provide macOS and Linux binaries for ARM64 and AMD64. They
need no Go, Pixi, or checkout. Install your chosen editor, ripgrep (`rg`), `fzf`,
and Git for the commands that use them; `note doctor` checks availability after
initialization. `note new --no-open` works without those tools.

## Release installer

```sh
curl -fsSL https://github.com/mariusrueve/note-cli/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
note --version
note init --root ~/Knowledge
```

Put the PATH export in `~/.zshrc`, `~/.bashrc`, or the equivalent shell setup.
The installer needs `curl`, `tar`, standard Unix tools, and `sha256sum` or
`shasum`. It downloads the latest release's checksum manifest, chooses the native
archive, then downloads that exact version and verifies its SHA256. It streams
only the binary from the archive and installs it atomically in `~/.local/bin`.
The installer requires no administrator access and leaves shell configuration
for you to edit. The script is available as a versioned release asset:

```sh
curl -fsSL https://github.com/mariusrueve/note-cli/releases/latest/download/install.sh -o install-note.sh
# Read install-note.sh, then choose the version and directory:
NOTE_VERSION=0.1.0 NOTE_INSTALL_DIR="$HOME/.local/bin" sh install-note.sh
```

The directory contains `note`, a private `.note-install.json` receipt, and a
stable `.note-update.lock` file. Existing binaries require a matching receipt
and binary hash; the installer refuses unrelated installations, symlinks,
modified executables, and downgrades. To replace an installation made through
Go or a package manager, uninstall it through that mechanism first, or choose
a different directory. Check `command -v note` if PATH selects another copy.

The script and downloads use HTTPS from this project's GitHub releases.
Checksums detect corruption and mismatched assets; they are served by the same
release source and are not an independent signature. The source and releases
must be publicly accessible for unauthenticated installation and self-update.

## Updates

```sh
note self-update --check
note self-update
```

Neither command needs a config file, knowledge root, Go, or external tools.
`--check` reports the latest stable release and changes no executable; it also
works for source builds. Availability/status goes to stderr and success returns
0 whether an update exists or the version is current. Manual network failures
return 3, and cancellation returns 130.

Replacement is supported only for release builds installed by this script.
The updater selects the latest published stable release, downloads its archive
and checksum, rejects unexpected/nonregular/duplicate archive entries, and
verifies the candidate version. It stages the executable beside the destination
and atomically renames it after validation, retaining executable permissions.
An installation lock prevents overlapping installs/updates; the receipt/hash is
rechecked before replacement. Failed downloads, validation, or cancellation
leave the previous executable intact. There are no automatic installs or
downgrades. Source and package builds use their own installation mechanism.

Successful interactive note commands may show a newer-version hint after the
operation. The network check is limited to one second and attempted at most once
per 24 hours; failures are silent. Only release builds check automatically.
Help, version, completion, internal commands, unsuccessful operations, headless
commands, and explicit `--json`/`--print` modes make no automatic requests.
The checker requests public release metadata only; no note content, paths,
configuration, or credentials are sent. Normal HTTPS connection metadata is
visible to GitHub.

Set `NOTE_NO_UPDATE_CHECK=1` to disable automatic checks and hints; explicit
`note self-update --check` remains available. Cache state is stored at
`$XDG_CACHE_HOME/note/update.json` when that base is absolute, otherwise
`~/.cache/note/update.json`. An unwritable cache never fails a note operation.
Unavailable caches and busy cache locks skip the automatic request.

## Source installation and removal

In the checkout, run `pixi run --locked install` to install into `GOBIN` or
`$(go env GOPATH)/bin`. Add that directory to PATH. Update by pulling/editing
source, running the locked checks, and repeating installation. These builds do
not check for updates automatically and self-update does not replace them.

To remove a release installation, delete its `note` executable and matching
`.note-install.json` receipt. Remove `.note-update.lock` only when no installation
or update process is active. The cache can also be removed. Configuration,
templates, and knowledge files are preserved.
