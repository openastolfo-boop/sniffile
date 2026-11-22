#!/usr/bin/env sh
# Build the Windows 10/11 desktop app.
#
# On Windows use MSYS2 (mingw-w64) or Git Bash + MinGW. From Linux/macOS
# install mingw-w64 and this script cross-compiles, provided the C toolchain
# can see the WebView2 headers (they are vendored in third_party).
set -eu

cd "$(dirname "$0")/.."
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo 0.10)}
VERSION=${VERSION#v}
VERSION=${VERSION:-0.10}
ARCH=${ARCH:-amd64}
OUT=${OUT:-dist}
mkdir -p "$OUT"

CC_BIN=${CC:-}
if [ -z "$CC_BIN" ]; then
	case "$(uname -s)" in
		MINGW*|MSYS*|CYGWIN*) CC_BIN=gcc ;;
		*)
			if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
				CC_BIN=x86_64-w64-mingw32-gcc
			else
				echo "No mingw-w64 compiler found." >&2
				echo "Windows: install MSYS2 and run this from the MINGW64 shell." >&2
				echo "Linux:   pacman -S mingw-w64-gcc (Arch) / apt install gcc-mingw-w64-x86-64" >&2
				exit 1
			fi
			;;
	esac
fi

# cgo compiles the WebView2 glue as C++, so the matching g++ matters.
CXX_BIN=${CXX:-}
if [ -z "$CXX_BIN" ]; then
	case "$CC_BIN" in
		*gcc) CXX_BIN="${CC_BIN%gcc}g++" ;;
		*)    CXX_BIN="$CC_BIN" ;;
	esac
fi

echo "building Windows $ARCH with $CC_BIN / $CXX_BIN"
CGO_ENABLED=1 GOOS=windows GOARCH="$ARCH" CC="$CC_BIN" CXX="$CXX_BIN" \
	${GO:-go} build -trimpath \
	-ldflags "-s -w -H windowsgui -X main.version=$VERSION" \
	-o "$OUT/sniffile.exe" ./cmd/sniffile

cp README.md LICENSE "$OUT/" 2>/dev/null || true
mkdir -p "$OUT/windows"
cp "$OUT/sniffile.exe" "$OUT/windows/"
cp README.md LICENSE packaging/icons/sniffile.ico "$OUT/windows/" 2>/dev/null || true
(cd "$OUT" && zip -qr "sniffile-${VERSION}-windows-${ARCH}.zip" windows)
rm -rf "$OUT/windows"

if command -v makensis >/dev/null 2>&1; then
	makensis -DVERSION="$VERSION" -DOUTDIR="$OUT" -DARCH="$ARCH" packaging/windows/sniffile.nsi
else
	echo "makensis not found: skipped the .exe installer (portable zip is ready)"
fi
echo "built Windows artifacts in $OUT"
