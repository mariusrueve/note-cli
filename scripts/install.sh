#!/bin/sh
# Install a checksum-verified release without Go, Pixi, or administrator access.
set -eu

note_release_base=https://github.com/mariusrueve/note-cli/releases
case "$(uname -s)" in
  Darwin) note_os=darwin ;;
  Linux) note_os=linux ;;
  *) printf 'note: only macOS and Linux are supported\n' >&2; exit 1 ;;
esac
case "$(uname -m)" in
  arm64|aarch64) note_arch=arm64 ;;
  x86_64|amd64) note_arch=amd64 ;;
  *) printf 'note: unsupported architecture\n' >&2; exit 1 ;;
esac

for note_tool in curl tar awk mktemp; do
  command -v "$note_tool" >/dev/null 2>&1 || { printf 'note: missing tool: %s\n' "$note_tool" >&2; exit 1; }
done
if command -v sha256sum >/dev/null 2>&1; then
  note_hash_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  note_hash_tool=shasum
else
  printf 'note: install sha256sum or shasum to verify the download\n' >&2
  exit 1
fi

note_install_dir=${NOTE_INSTALL_DIR:-${HOME:?HOME must be set}/.local/bin}
note_temp=$(mktemp -d)
trap 'rm -rf "$note_temp"' 0
trap 'exit 130' INT
trap 'exit 143' TERM

note_download() {
  curl -fLsS --proto '=https' --proto-redir '=https' \
    --connect-timeout 10 --max-time 120 -o "$2" "$1"
}

if [ -n "${NOTE_VERSION:-}" ]; then
  # Only a stable numeric version may become part of a download URL.
  printf '%s\n' "$NOTE_VERSION" | awk '
    NR!=1 {bad=1; exit 1}
    /^[0-9]+\.[0-9]+\.[0-9]+$/ {
      n=split($0,a,"."); for(i=1;i<=n;i++) if(length(a[i])>1 && substr(a[i],1,1)=="0") {bad=1; exit 1}
      ok=1
    }
    END {if(!ok || bad) exit 1}' || { printf 'note: invalid NOTE_VERSION\n' >&2; exit 1; }
  note_sums_url="$note_release_base/download/v$NOTE_VERSION/checksums.txt"
else
  note_sums_url="$note_release_base/latest/download/checksums.txt"
fi
note_download "$note_sums_url" "$note_temp/checksums.txt"
note_match=$(awk -v target="_${note_os}_${note_arch}[.]tar[.]gz" '
  NF==2 && $2 ~ ("^note_[0-9]+[.][0-9]+[.][0-9]+" target "$") {
    if(length($1)!=64 || $1 ~ /[^0-9a-fA-F]/) exit 1
    count++; match_line=$1 " " $2
  }
  END {if(count!=1) exit 1; print match_line}' "$note_temp/checksums.txt") || {
  printf 'note: release has no unique, valid checksum for this platform\n' >&2; exit 1;
}
# Fields contain only a validated hash and a fixed numeric archive name.
note_sum=${note_match%% *}
note_archive=${note_match##* }
note_version=${note_archive#note_}
note_version=${note_version%_${note_os}_${note_arch}.tar.gz}
if [ -n "${NOTE_VERSION:-}" ] && [ "$note_version" != "$NOTE_VERSION" ]; then
  printf 'note: checksum manifest does not match NOTE_VERSION\n' >&2; exit 1
fi
note_download "$note_release_base/download/v$note_version/$note_archive" "$note_temp/$note_archive"
(
  cd "$note_temp"
  printf '%s  %s\n' "$note_sum" "$note_archive" > selected-checksum.txt
  if [ "$note_hash_tool" = sha256sum ]; then
    sha256sum -c selected-checksum.txt
  else
    shasum -a 256 -c selected-checksum.txt
  fi
)
# Never extract archive paths to disk; only stream the expected binary.
test "$(tar tzf "$note_temp/$note_archive" | LC_ALL=C sort)" = "$(printf 'LICENSE\nREADME.md\nnote\n' | LC_ALL=C sort)" || {
  printf 'note: unexpected release archive contents\n' >&2; exit 1;
}
tar xOzf "$note_temp/$note_archive" note > "$note_temp/note"
chmod 755 "$note_temp/note"
case "$("$note_temp/note" --version)" in
  "note version $note_version ("*")") ;;
  *) printf 'note: downloaded executable has an unexpected version\n' >&2; exit 1 ;;
esac
"$note_temp/note" __install --directory "$note_install_dir"
