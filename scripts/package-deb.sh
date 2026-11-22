#!/usr/bin/env sh
# Build a Debian binary package without debhelper or dpkg-deb.
# Works on Arch, Fedora, Debian, Ubuntu, and anywhere with ar + tar + xz.
set -eu

cd "$(dirname "$0")/.."
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo 0.10)}
VERSION=${VERSION#v}
VERSION=${VERSION:-0.10}
ARCH=${ARCH:-$(go env GOARCH)}
WEBKIT=${WEBKIT:-4.1}
case "$ARCH" in
	amd64) DEB_ARCH=amd64 ;;
	arm64) DEB_ARCH=arm64 ;;
	386)   DEB_ARCH=i386 ;;
	*)     DEB_ARCH=$ARCH ;;
esac
OUT=${OUT:-dist}
PKG=sniffile_${VERSION}_${DEB_ARCH}
ROOT=$(mktemp -d)
trap 'rm -rf "$ROOT"' EXIT

if [ ! -x bin/sniffile ]; then
	echo "bin/sniffile is missing; run make build first" >&2
	exit 1
fi

mkdir -p "$ROOT/DEBIAN" "$ROOT/usr/bin" "$ROOT/usr/share/applications" \
	"$ROOT/usr/share/metainfo" "$ROOT/usr/share/man/man1" "$ROOT/usr/share/doc/sniffile"
install -m755 bin/sniffile "$ROOT/usr/bin/sniffile"
install -m644 packaging/linux/sniffile.desktop "$ROOT/usr/share/applications/sniffile.desktop"
install -m644 packaging/linux/sniffile.metainfo.xml "$ROOT/usr/share/metainfo/io.github.openastolfo-boop.sniffile.metainfo.xml"
install -m644 packaging/linux/sniffile.1 "$ROOT/usr/share/man/man1/sniffile.1"
install -m644 LICENSE "$ROOT/usr/share/doc/sniffile/copyright"
for s in 16 24 32 48 64 128 256 512; do
	mkdir -p "$ROOT/usr/share/icons/hicolor/${s}x${s}/apps"
	install -m644 "packaging/icons/sniffile-$s.png" "$ROOT/usr/share/icons/hicolor/${s}x${s}/apps/sniffile.png"
done
if command -v gzip >/dev/null 2>&1; then
	gzip -9n "$ROOT/usr/share/man/man1/sniffile.1"
fi

INSTALLED_SIZE=$(du -sk "$ROOT/usr" | cut -f1)
cat > "$ROOT/DEBIAN/control" <<EOF
Package: sniffile
Version: $VERSION
Section: utils
Priority: optional
Architecture: $DEB_ARCH
Installed-Size: $INSTALLED_SIZE
Maintainer: openastolfo-boop <openastolfo-boop@users.noreply.github.com>
Homepage: https://github.com/openastolfo-boop/sniffile
Description: Fast local file search that never leaves your machine
 Sniffile indexes folders you choose and finds files by name, folder,
 date, extension, image orientation and text inside plain-text files.
 It understands Russian and English queries, survives typos, and runs
 in a native desktop window with no cloud dependency.
Depends: libgtk-3-0, libwebkit2gtk-${WEBKIT}-0 (>= 2.40)
EOF

cat > "$ROOT/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database -q /usr/share/applications || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
	gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
fi
exit 0
EOF
cat > "$ROOT/DEBIAN/postrm" <<'EOF'
#!/bin/sh
set -e
if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database -q /usr/share/applications || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
	gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
fi
exit 0
EOF
chmod 755 "$ROOT/DEBIAN/postinst" "$ROOT/DEBIAN/postrm"

mkdir -p "$OUT"
(cd "$ROOT/DEBIAN" && tar --owner=0 --group=0 --numeric-owner -cJf "$ROOT/control.tar.xz" .)
(cd "$ROOT" && tar --owner=0 --group=0 --numeric-owner -cJf data.tar.xz ./usr)
printf '2.0\n' > "$ROOT/debian-binary"
(cd "$ROOT" && ar rcs "$PKG.deb" debian-binary control.tar.xz data.tar.xz)
mv "$ROOT/$PKG.deb" "$OUT/$PKG.deb"
echo "built $OUT/$PKG.deb"
