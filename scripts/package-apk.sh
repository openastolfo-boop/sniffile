#!/usr/bin/env sh
# Build an Alpine .apk with nfpm.
set -eu

cd "$(dirname "$0")/.."
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo 0.10)}
VERSION=${VERSION#v}
VERSION=${VERSION:-0.10}
ARCH=${ARCH:-$(go env GOARCH)}
OUT=${OUT:-dist}

if ! command -v nfpm >/dev/null 2>&1; then
	echo "nfpm is required: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest" >&2
	exit 1
fi
# Alpine uses musl; a glibc binary will not start there. Build inside Alpine.
if ! (ldd --version 2>&1 | grep -qi musl) && [ "${ALLOW_GLIBC_APK:-0}" != "1" ]; then
	echo "build the Alpine package inside Alpine (or set ALLOW_GLIBC_APK=1 if you really know why)" >&2
	exit 1
fi
if [ ! -x bin/sniffile ]; then
	echo "bin/sniffile is missing; run make build first" >&2
	exit 1
fi

mkdir -p "$OUT"
cp bin/sniffile "$OUT/sniffile"
VERSION=$VERSION GOARCH=$ARCH nfpm package -f packaging/nfpm.yaml -p apk -t "$OUT"
rm -f "$OUT/sniffile"
echo "built apk in $OUT"
