#!/usr/bin/env sh
# Build an Arch package (.pkg.tar.zst) from the current working tree.
# Produces a package that installs on Arch, Manjaro, EndeavourOS, CachyOS,
# Garuda and any other pacman-based distribution with webkit2gtk-4.1.
set -eu

cd "$(dirname "$0")/.."
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo 0.10)}
VERSION=${VERSION#v}
VERSION=${VERSION:-0.10}
OUT=$(pwd)/dist
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

if ! command -v makepkg >/dev/null 2>&1; then
	echo "makepkg is required (pacman -S base-devel)" >&2
	exit 1
fi

mkdir -p "$OUT" "$WORK/src/sniffile-$VERSION"
# Stage the working tree, not just the last commit: local packaging should
# include uncommitted fixes without forcing a commit.
tar --exclude=.git --exclude=bin --exclude=dist --exclude='*.pkg.tar.*' \
	-cf - . | tar -C "$WORK/src/sniffile-$VERSION" -xf -

sed "s/^pkgver=.*/pkgver=$VERSION/" packaging/arch/PKGBUILD > "$WORK/PKGBUILD"
tar -C "$WORK/src" -czf "$WORK/sniffile-$VERSION.tar.gz" "sniffile-$VERSION"

(cd "$WORK" && makepkg -f --nodeps --noconfirm)
find "$WORK" -maxdepth 1 -name 'sniffile-[0-9]*.pkg.tar.zst' -exec cp {} "$OUT/" \;
echo "built $(find "$OUT" -maxdepth 1 -name 'sniffile-[0-9]*.pkg.tar.zst' | tail -1)"
