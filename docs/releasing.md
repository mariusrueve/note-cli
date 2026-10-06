# Releases

The first release is 0.1.0. Shipped binaries use the locked Go 1.27 toolchain with
CGO disabled and require no Go runtime, module cache, checkout, or Pixi. Runtime
commands still need their configured editor, rg, fzf, or Git as documented.
Go 1.26 compatibility and native race tests are checked separately.

```sh
pixi install --locked
pixi run --locked check
pixi run --locked security
pixi run --locked snapshot
pixi run --locked verify-release work/releases/snapshot snapshot
```

The builder creates darwin/arm64, darwin/amd64, linux/arm64, and linux/amd64
`.tar.gz` archives containing executable `note`, README.md, and the MIT license.
It writes `checksums.txt` and `build-info.json`, recording the actual commit,
whether source is dirty, binary hashes, Go version/settings, and linked module
versions/checksums. Archive metadata is deterministic. Snapshots can contain
uncommitted changes; versioned builds require a clean checkout. The builder
publishes nothing. To build a clean version locally:

```sh
pixi run --locked release --version 0.1.0
pixi run --locked verify-release work/releases/0.1.0 0.1.0
```

Every main push and pull request runs formatting, module verification/tidy, vet,
coverage, real Git/rg/fzf/PTY integration, native race tests, completions, and
ordinary installation on native macOS ARM64/Intel and Linux ARM64/AMD64 with both
Go lines. Workflow lint and a reachable-vulnerability scan must also pass. The
pipeline then cross-builds and verifies each archive on its native runner,
including standalone creation with no runtime tools on PATH and interactive
picker/editor/cancellation tests of the actual archived binary. Cross-compilation
alone is never treated as native execution proof.

To rehearse the release pipeline, manually dispatch **Release** with version
`0.1.0`. It repeats all gates and retains verified distribution artifacts without
publishing. The only publication trigger is a `vX.Y.Z` tag. Before tagging:

1. Set `workspace.version` in pixi.toml and finalize that version's dated
   CHANGELOG heading (`## [0.1.0] - YYYY-MM-DD`). Commit and push to main.
2. Wait for the aggregate `CI` check to pass on that commit. Review the archive
   and build record, and keep the verification record current.
3. Create and push an annotated tag for that exact commit:
   `git tag -a v0.1.0 -m 'note 0.1.0'` then `git push origin v0.1.0`.
4. The tag workflow checks version consistency and ancestry from main, repeats
   the full validation, and publishes the verified assets with generated notes.
   Only the publication job has repository write permission. Failed validation
   leaves the release unpublished. Never move an existing release tag or replace
   its assets; fix the issue in a new patch version.

GitHub artifacts are retained for seven days. Published release assets persist
on the repository's [Releases page](https://github.com/mariusrueve/note-cli/releases).
The repository's access controls also apply to downloads. The build record and
checksums document contents; they are not a cryptographic signature or attestation.

Download your OS/architecture archive and `checksums.txt` from the same release.
Verify the matching archive's checksum before extracting and installing it:

```sh
# Download the two files to an empty directory first.
# Check the matching checksum entry (the full file also lists the other targets).
grep '  note_0.1.0_darwin_arm64.tar.gz$' checksums.txt | shasum -a 256 -c -
# Linux: use sha256sum -c - and your linux archive name.
tar xzf note_0.1.0_darwin_arm64.tar.gz
mkdir -p ~/.local/bin
cp note ~/.local/bin/note
export PATH="$HOME/.local/bin:$PATH"
note --version
```

For authenticated downloads, `gh release download v0.1.0 --repo
mariusrueve/note-cli --pattern 'note_0.1.0_darwin_arm64.tar.gz' --pattern
checksums.txt` fetches the archive and checksums. Substitute your target.
Update by verifying and replacing the executable. Uninstall by removing that
executable; config, templates, and knowledge files remain. There is no self-updater
or distribution-package dependency.
