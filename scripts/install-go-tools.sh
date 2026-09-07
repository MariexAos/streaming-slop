#!/bin/sh
set -eu
version=$1
bin_dir=$2
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case $(uname -m) in
  arm64|aarch64) arch=arm64 ;;
  x86_64) arch=amd64 ;;
  *) echo 'Unsupported architecture' >&2; exit 1 ;;
esac
archive="golangci-lint-${version#v}-${os}-${arch}.tar.gz"
url="https://github.com/golangci/golangci-lint/releases/download/$version"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
curl --fail --silent --show-error --location "$url/$archive" -o "$work/$archive"
curl --fail --silent --show-error --location "$url/golangci-lint-${version#v}-checksums.txt" -o "$work/checksums.txt"
(cd "$work" && awk -v archive="$archive" '$2 == archive { print; found=1 } END { if (!found) exit 1 }' checksums.txt > checksum)
(cd "$work" && shasum -a 256 -c checksum)
tar -xzf "$work/$archive" -C "$work"
mkdir -p "$bin_dir"
cp "$work/golangci-lint-${version#v}-${os}-${arch}/golangci-lint" "$bin_dir/golangci-lint-$version"
