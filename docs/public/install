#!/bin/sh
set -eu

repo="${ORCH_REPO:-orchio/orch}"
version="${ORCH_VERSION:-latest}"
install_dir="${ORCH_INSTALL_DIR:-}"

say() {
  printf '%s\n' "$*"
}

fail() {
  say "orch install: $*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

default_install_dir() {
  if [ -n "$install_dir" ]; then
    printf '%s\n' "$install_dir"
    return
  fi

  if [ -n "${HOME:-}" ]; then
    printf '%s\n' "$HOME/.local/bin"
    return
  fi

  printf '%s\n' "/usr/local/bin"
}

normalize_os() {
  case "$(uname -s)" in
    Darwin) printf '%s\n' "Darwin" ;;
    Linux) printf '%s\n' "Linux" ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
  esac
}

normalize_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) printf '%s\n' "x86_64" ;;
    arm64 | aarch64) printf '%s\n' "arm64" ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

latest_version() {
  latest_url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")"
  tag="${latest_url##*/}"
  [ -n "$tag" ] && [ "$tag" != "latest" ] || fail "could not resolve latest release for $repo"
  printf '%s\n' "$tag"
}

checksum_command() {
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s\n' "sha256sum"
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    printf '%s\n' "shasum -a 256"
    return
  fi
  printf '%s\n' ""
}

verify_checksum() {
  checksum_file="$1"
  archive_file="$2"
  asset_name="$3"

  sum_cmd="$(checksum_command)"
  if [ -z "$sum_cmd" ]; then
    say "No SHA-256 tool found; skipping checksum verification."
    return
  fi

  expected="$(grep "  $asset_name\$" "$checksum_file" | awk '{print $1}' || true)"
  [ -n "$expected" ] || fail "checksum for $asset_name was not found"

  actual="$($sum_cmd "$archive_file" | awk '{print $1}')"
  [ "$actual" = "$expected" ] || fail "checksum verification failed for $asset_name"
}

need curl
need tar
need uname
need mktemp

os="$(normalize_os)"
arch="$(normalize_arch)"
suffix="${os}_${arch}"

if [ "$version" = "latest" ]; then
  version="$(latest_version)"
fi

asset="orch_${version}_${suffix}.tar.gz"
base_url="https://github.com/$repo/releases/download/$version"
archive_url="$base_url/$asset"
checksums_url="$base_url/checksums.txt"
target_dir="$(default_install_dir)"

tmp_dir="$(mktemp -d 2>/dev/null || mktemp -d -t orch-install)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

say "Installing orch $version for $suffix"
say "Downloading $archive_url"
curl -fsSL "$archive_url" -o "$tmp_dir/$asset"

if curl -fsSL "$checksums_url" -o "$tmp_dir/checksums.txt"; then
  verify_checksum "$tmp_dir/checksums.txt" "$tmp_dir/$asset" "$asset"
else
  say "No checksums.txt found for $version; skipping checksum verification."
fi

tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"
binary="$(find "$tmp_dir" -type f -name orch -perm -u+x | head -n 1)"
[ -n "$binary" ] || fail "release archive did not contain an executable orch binary"

mkdir -p "$target_dir"
[ -w "$target_dir" ] || fail "$target_dir is not writable; set ORCH_INSTALL_DIR to a writable directory"

cp "$binary" "$target_dir/orch"
chmod 0755 "$target_dir/orch"

say "orch installed to $target_dir/orch"
if ! command -v orch >/dev/null 2>&1; then
  say "Add $target_dir to PATH to run orch from any shell."
fi
