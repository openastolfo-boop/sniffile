BIN     := sniffile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
VERSION := $(if $(strip $(VERSION)),$(VERSION),0.10)
LDFLAGS := -s -w -X main.version=$(VERSION)
TAGS    ?=
PREFIX  ?= /usr/local
DESTDIR ?=
GO      ?= go
DIST    := dist
ICONS   := 16 24 32 48 64 128 256 512

.PHONY: all build run test vet fmt fmt-check install uninstall clean \
        dist deb rpm apk arch windows release check-deps

all: build

# The desktop app always needs CGO: that is what puts a native window on screen.
build:
	CGO_ENABLED=1 $(GO) build -trimpath -tags "$(TAGS)" -ldflags "$(LDFLAGS)" -o bin/$(BIN) ./cmd/sniffile

# Build against the older WebKitGTK 4.0 ABI (Debian 11, Ubuntu 22.04, ...).
build-legacy:
	$(MAKE) build TAGS="webkit2_40"

run:
	CGO_ENABLED=1 $(GO) run ./cmd/sniffile

test:
	CGO_ENABLED=1 $(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*' -not -path './third_party/*')

fmt-check:
	@out=$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*' -not -path './third_party/*')); \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# Install as a normal desktop application (binary + menu entry + icons + man page).
install: build
	install -Dm755 bin/$(BIN) "$(DESTDIR)$(PREFIX)/bin/$(BIN)"
	install -Dm644 packaging/linux/sniffile.desktop "$(DESTDIR)$(PREFIX)/share/applications/sniffile.desktop"
	install -Dm644 packaging/linux/sniffile.metainfo.xml "$(DESTDIR)$(PREFIX)/share/metainfo/io.github.openastolfo-boop.sniffile.metainfo.xml"
	install -Dm644 packaging/linux/sniffile.1 "$(DESTDIR)$(PREFIX)/share/man/man1/sniffile.1"
	for s in $(ICONS); do \
		install -Dm644 packaging/icons/sniffile-$$s.png "$(DESTDIR)$(PREFIX)/share/icons/hicolor/$${s}x$${s}/apps/sniffile.png"; \
	done

uninstall:
	rm -f "$(DESTDIR)$(PREFIX)/bin/$(BIN)"
	rm -f "$(DESTDIR)$(PREFIX)/share/applications/sniffile.desktop"
	rm -f "$(DESTDIR)$(PREFIX)/share/metainfo/io.github.openastolfo-boop.sniffile.metainfo.xml"
	rm -f "$(DESTDIR)$(PREFIX)/share/man/man1/sniffile.1"
	for s in $(ICONS); do rm -f "$(DESTDIR)$(PREFIX)/share/icons/hicolor/$${s}x$${s}/apps/sniffile.png"; done

clean:
	rm -rf bin $(DIST)

# Portable Linux tarball: unpack anywhere, run ./sniffile.
dist: build
	mkdir -p $(DIST)/sniffile-$(VERSION)
	cp bin/$(BIN) $(DIST)/sniffile-$(VERSION)/
	cp LICENSE README.md README.ru.md $(DIST)/sniffile-$(VERSION)/
	cp packaging/linux/sniffile.desktop packaging/linux/sniffile.metainfo.xml packaging/linux/sniffile.1 $(DIST)/sniffile-$(VERSION)/
	cp -r packaging/icons $(DIST)/sniffile-$(VERSION)/
	tar -C $(DIST) -czf $(DIST)/sniffile-$(VERSION)-linux-$$(go env GOARCH).tar.gz sniffile-$(VERSION)
	rm -rf $(DIST)/sniffile-$(VERSION)

# Debian / Ubuntu / Mint / Pop!_OS / Kali / Raspberry Pi OS ...
deb: build
	./scripts/package-deb.sh

# Fedora / RHEL / CentOS / Rocky / Alma / openSUSE ...
rpm: build
	./scripts/package-rpm.sh

# Alpine
apk: build
	./scripts/package-apk.sh

# Arch / Manjaro / EndeavourOS / CachyOS ...
arch:
	./scripts/package-arch.sh

# Windows 10/11 (run on Windows, MSYS2, or with a mingw-w64 cross compiler).
windows:
	./scripts/build-windows.sh

# Portable artifacts that need no distro-specific tooling.
release: dist deb
	@mkdir -p $(DIST)
	@echo "artifacts in $(DIST)/ — run 'make rpm', 'make apk' or 'make arch' on the matching distro/CI"

check-deps:
	@printf 'go:      '; $(GO) version
	@printf 'gcc:     '; gcc --version | head -1
	@printf 'webkit:  '; pkg-config --modversion webkit2gtk-4.1 2>/dev/null || pkg-config --modversion webkit2gtk-4.0 2>/dev/null || echo "MISSING (install libwebkit2gtk-4.1-dev / webkit2gtk-4.1)"
	@printf 'gtk3:    '; pkg-config --modversion gtk+-3.0 2>/dev/null || echo "MISSING (install libgtk-3-dev / gtk3)"
