#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
note_dist_dir="${1:?usage: release-smoke.sh DIST_DIRECTORY VERSION}"
note_version="${2:?expected version required}"
note_dist_dir="$(cd "$note_dist_dir" && pwd)"
(
  cd "$note_dist_dir"
  if command -v sha256sum > /dev/null; then
    sha256sum -c checksums.txt
  else
    shasum -a 256 -c checksums.txt
  fi
  for archive in *.tar.gz; do
    test "$(tar tzf "$archive" | sort)" = "$(printf 'LICENSE\nREADME.md\nnote\n' | sort)"
  done
)
case "$(uname -s)" in Darwin) note_os=darwin ;; Linux) note_os=linux ;; *) exit 1 ;; esac
case "$(uname -m)" in arm64|aarch64) note_arch=arm64 ;; x86_64) note_arch=amd64 ;; *) exit 1 ;; esac
note_temp="$(mktemp -d)"
trap 'rm -rf "$note_temp"' EXIT
tar xzf "$note_dist_dir/note_${note_version}_${note_os}_${note_arch}.tar.gz" -C "$note_temp"
note_binary="$note_temp/note"
note_commit="$(git rev-parse HEAD)"
test "$("$note_binary" --version)" = "note version $note_version ($note_commit)"
# These installed-binary checks run without Go, Git, rg, fzf or an editor on PATH.
(
  cd "$note_temp"
  export PATH=/no-runtime-tools
  export XDG_CONFIG_HOME="$note_temp/config"
  "$note_binary" --help > /dev/null
  "$note_binary" init --root "$note_temp/knowledge"
  "$note_binary" templates > /dev/null
  "$note_binary" new "release smoke" --no-open
)
test "$(cat "$note_temp/knowledge/release-smoke.md")" = '# Release smoke'
# Exercise the actual archive binary in a real terminal on each native CI target.
NOTE_TEST_BINARY="$note_binary" go test -mod=readonly -count=1 ./cmd/note
