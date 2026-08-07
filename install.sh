#!/bin/sh

set -eu

repository="siliconwitchery/superstack-cli"

platform=$(uname -s)

case "$platform" in
    Linux) platform="linux" ;;
    Darwin) platform="darwin" ;;
    *) echo "Unsupported operating system: $platform" >&2; exit 1 ;;
esac

architecture=$(uname -m)

case "$architecture" in
    x86_64) architecture="amd64" ;;
    aarch64 | arm64) architecture="arm64" ;;
    *) echo "Unsupported architecture: $architecture" >&2; exit 1 ;;
esac

tag="${VERSION:-}"

if [ -z "$tag" ]; then
    latest=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repository/releases/latest")

    case "$latest" in
        */releases/tag/*) tag="${latest##*/}" ;;
        *) echo "No published release found for $repository" >&2; exit 1 ;;
    esac
fi

case "$tag" in
    v*) ;;
    *) tag="v$tag" ;;
esac

version="${tag#v}"

archive="superstack_${version}_${platform}_${architecture}.tar.gz"
download="https://github.com/$repository/releases/download/$tag"

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
trap 'exit 1' INT TERM HUP

curl -fsSL "$download/$archive" -o "$workdir/$archive"
curl -fsSL "$download/checksums.txt" -o "$workdir/checksums.txt"

cd "$workdir"

awk -v name="$archive" '$2 == name' checksums.txt > expected.txt

if command -v sha256sum > /dev/null; then
    sha256sum -c expected.txt > /dev/null
else
    shasum -a 256 -c expected.txt > /dev/null
fi

tar -xzf "$archive" superstack

if [ -w /usr/local/bin ]; then
    install -m 755 superstack /usr/local/bin/superstack
else
    sudo mkdir -p /usr/local/bin
    sudo install -m 755 superstack /usr/local/bin/superstack
fi

echo "Installed superstack $version to /usr/local/bin/superstack"
