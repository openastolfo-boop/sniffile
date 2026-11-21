# Packaging

Everything here produces a **native desktop app**, not a website launcher.
The binary is built with CGO and links the system GTK 3 + WebKitGTK (or
WebView2 on Windows).

## One command per target

| target | command | output |
|---|---|---|
| portable Linux | `make dist` | `dist/sniffile-<ver>-linux-<arch>.tar.gz` |
| Debian/Ubuntu | `make deb` | `dist/sniffile_<ver>_<arch>.deb` |
| Fedora/openSUSE | `make rpm` | `dist/sniffile-<ver>.<arch>.rpm` |
| Alpine | `make apk` (on Alpine) | `dist/sniffile-<ver>.<arch>.apk` |
| Arch | `make arch` | `dist/sniffile-<ver>-1-<arch>.pkg.tar.zst` |
| Windows | `make windows` | `.exe`, portable `.zip`, NSIS `setup.exe` |
| system install | `sudo make install PREFIX=/usr` | binary, menu entry, icons, man page |

## Build dependencies

- Arch: `base-devel go gtk3 webkit2gtk-4.1 zip`
- Debian/Ubuntu: `build-essential pkg-config golang-go libgtk-3-dev libwebkit2gtk-4.1-dev`
- Ubuntu 22.04 / Debian 11: use `libwebkit2gtk-4.0-dev` and `make build-legacy`
- Fedora: `gcc make pkgconf-pkg-config golang gtk3-devel webkit2gtk4.1-devel`
- Alpine: `build-base go pkgconf gtk+3.0-dev webkit2gtk-4.1-dev`
- Windows: MSYS2 with `mingw-w64-x86_64-gcc`, `mingw-w64-x86_64-nsis`, `zip`

## Notes

- `scripts/package-deb.sh` builds a valid `.deb` with `ar` and `tar`, so it also
  works on Arch or Fedora where `dpkg-deb` is absent.
- `debian/` is a normal debhelper source tree for `dpkg-buildpackage`.
- `packaging/nfpm.yaml` is used by CI for `.deb`, `.rpm` and `.apk`.
- `packaging/arch/PKGBUILD` can be submitted to the AUR; `make arch` stages a
  source tarball from the working tree so uncommitted fixes are included.
- The Windows installer checks for WebView2 and offers the official download
  when it is missing.
