# Build with packaging/scripts/package-rpm.sh, or:
#   rpmbuild -bb packaging/rpm/sniffile.spec \
#     --define "_version 0.3.0" --define "_sourcedir $PWD/bin"
Name:           sniffile
Version:        %{_version}
Release:        1%{?dist}
Summary:        Fast local file search that never leaves your machine
License:        MIT
URL:            https://github.com/openastolfo-boop/sniffile
Source0:        sniffile
Source1:        sniffile.desktop
Source2:        sniffile.metainfo.xml
Source3:        sniffile.1
BuildArch:      x86_64
Requires:       gtk3
Requires:       webkit2gtk4.1

%description
Sniffile indexes folders you choose and finds files by name, folder, date,
extension, image orientation and text inside plain-text files. It understands
Russian and English queries, survives typos, and runs in a native desktop
window with no cloud dependency.

%prep

%build

%install
install -Dm755 %{SOURCE0} %{buildroot}%{_bindir}/sniffile
install -Dm644 %{SOURCE1} %{buildroot}%{_datadir}/applications/sniffile.desktop
install -Dm644 %{SOURCE2} %{buildroot}%{_datadir}/metainfo/io.github.openastolfo-boop.sniffile.metainfo.xml
install -Dm644 %{SOURCE3} %{buildroot}%{_mandir}/man1/sniffile.1
for s in 16 24 32 48 64 128 256 512; do
  install -Dm644 %{_sourcedir}/icons/sniffile-$s.png %{buildroot}%{_datadir}/icons/hicolor/${s}x${s}/apps/sniffile.png
done
install -Dm644 %{_sourcedir}/LICENSE %{buildroot}%{_licensedir}/sniffile/LICENSE

%files
%{_bindir}/sniffile
%{_datadir}/applications/sniffile.desktop
%{_datadir}/metainfo/io.github.openastolfo-boop.sniffile.metainfo.xml
%{_mandir}/man1/sniffile.1*
%{_datadir}/icons/hicolor/*/apps/sniffile.png
%{_licensedir}/sniffile/LICENSE

%changelog
* Sun Nov 15 2026 openastolfo-boop <openastolfo-boop@users.noreply.github.com> - 0.3.0
- Native desktop window, Wayland crash fix, Linux and Windows packaging.
