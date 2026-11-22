#!/usr/bin/env sh
# Build a Fedora/RHEL/openSUSE RPM. Uses nfpm when available, otherwise
# rpmbuild. Run on a machine with rpmbuild and the WebKitGTK build deps.
set -eu

cd "$(dirname "$0")/.."
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo 0.10)}
VERSION=${VERSION#v}
VERSION=${VERSION:-0.10}
ARCH=${ARCH:-$(go env GOARCH)}
OUT=${OUT:-dist}
mkdir -p "$OUT"

if [ ! -x bin/sniffile ]; then
	echo "bin/sniffile is missing; run make build first" >&2
	exit 1
fi

if command -v nfpm >/dev/null 2>&1; then
	cp bin/sniffile "$OUT/sniffile"
	VERSION=$VERSION GOARCH=$ARCH nfpm package -f packaging/nfpm.yaml -p rpm -t "$OUT"
	rm -f "$OUT/sniffile"
	echo "built $OUT/sniffile-${VERSION}.${ARCH}.rpm"
	exit 0
fi

if ! command -v rpmbuild >/dev/null 2>&1; then
	echo "rpmbuild or nfpm is required (dnf install rpm-build, or go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest)" >&2
	exit 1
fi

TOP=$(mktemp -d)
trap 'rm -rf "$TOP"' EXIT
mkdir -p "$TOP/SOURCES" "$TOP/BUILD" "$TOP/RPMS" "$TOP/SPECS" "$TOP/SRPMS"
cp bin/sniffile "$TOP/SOURCES/sniffile"
cp packaging/linux/sniffile.desktop "$TOP/SOURCES/"
cp packaging/linux/sniffile.metainfo.xml "$TOP/SOURCES/"
cp packaging/linux/sniffile.1 "$TOP/SOURCES/"
cp LICENSE "$TOP/SOURCES/"
cp -r packaging/icons "$TOP/SOURCES/icons"

rpmbuild -bb packaging/rpm/sniffile.spec \
	--define "_version $VERSION" \
	--define "_topdir $TOP" \
	--define "_sourcedir $TOP/SOURCES" \
	--define "debug_package %{nil}"
find "$TOP/RPMS" -name '*.rpm' -exec cp {} "$OUT/" \;
echo "built $(find "$OUT" -maxdepth 1 -name '*.rpm' | tail -1)"
