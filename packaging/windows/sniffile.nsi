; Sniffile installer for Windows 10/11 (NSIS 3, Unicode).
; Build: makensis -DVERSION=0.10 -DOUTDIR=dist packaging/windows/sniffile.nsi
Unicode true

!ifndef VERSION
  !define VERSION "0.10"
!endif
!ifndef OUTDIR
  !define OUTDIR "dist"
!endif
!ifndef ARCH
  !define ARCH "amd64"
!endif

!define APPNAME "Sniffile"
!define COMPANY "openastolfo-boop"
!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Sniffile"

Name "${APPNAME}"
OutFile "${OUTDIR}\sniffile-${VERSION}-windows-${ARCH}-setup.exe"
InstallDir "$PROGRAMFILES64\Sniffile"
InstallDirRegKey HKLM "${UNINSTKEY}" "InstallLocation"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
BrandingText "Sniffile — local file search"

!include "MUI2.nsh"
!include "x64.nsh"

!define MUI_ABORTWARNING
!define MUI_ICON "packaging\icons\sniffile.ico"
!define MUI_UNICON "packaging\icons\sniffile.ico"
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "Russian"

; WebView2 runtime is present on current Windows 10/11 by default, but a
; stripped or LTSC install may miss it. Warn with a clickable download link.
Function .onInit
  ReadRegStr $0 HKLM "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  StrCmp $0 "" 0 done
  ReadRegStr $0 HKCU "Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  StrCmp $0 "" missing done
  missing:
    MessageBox MB_YESNO|MB_ICONQUESTION \
      "Microsoft Edge WebView2 is not installed. Sniffile needs it to draw its window.$\r$\n$\r$\nOpen the official download page now?" \
      IDNO done
    ExecShell "open" "https://developer.microsoft.com/microsoft-edge/webview2/"
  done:
FunctionEnd

Section "Sniffile" SecMain
  SectionIn RO
  SetOutPath "$INSTDIR"
  File "${OUTDIR}\sniffile.exe"
  File "LICENSE"
  File "README.md"

  CreateShortcut "$SMPROGRAMS\Sniffile.lnk" "$INSTDIR\sniffile.exe" "" "$INSTDIR\sniffile.exe" 0
  CreateShortcut "$DESKTOP\Sniffile.lnk" "$INSTDIR\sniffile.exe" "" "$INSTDIR\sniffile.exe" 0

  WriteUninstaller "$INSTDIR\uninstall.exe"
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayName" "${APPNAME}"
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "${UNINSTKEY}" "Publisher" "${COMPANY}"
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayIcon" "$INSTDIR\sniffile.exe"
  WriteRegStr HKLM "${UNINSTKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${UNINSTKEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegDWORD HKLM "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINSTKEY}" "NoRepair" 1
SectionEnd

Section "Uninstall"
  Delete "$INSTDIR\sniffile.exe"
  Delete "$INSTDIR\LICENSE"
  Delete "$INSTDIR\README.md"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\Sniffile.lnk"
  Delete "$DESKTOP\Sniffile.lnk"
  DeleteRegKey HKLM "${UNINSTKEY}"
SectionEnd
