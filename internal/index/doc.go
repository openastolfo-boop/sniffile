package index

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// File kinds. Kept as strings so the JSON stays readable if you open the
// index by hand (which is fine, it is just gzip + json).
const (
	KindImage   = "image"
	KindVideo   = "video"
	KindAudio   = "audio"
	KindText    = "text"
	KindCode    = "code"
	KindDoc     = "doc"
	KindArchive = "archive"
	KindOther   = "other"
)

// Kinds is used by the UI filter row.
var Kinds = []string{KindImage, KindVideo, KindAudio, KindText, KindCode, KindDoc, KindArchive, KindOther}

var extKind = map[string]string{
	// images
	"jpg": KindImage, "jpeg": KindImage, "png": KindImage, "gif": KindImage,
	"webp": KindImage, "bmp": KindImage, "tif": KindImage, "tiff": KindImage,
	"heic": KindImage, "heif": KindImage, "avif": KindImage, "ico": KindImage,
	"svg": KindImage, "jxl": KindImage, "raw": KindImage, "cr2": KindImage,
	"nef": KindImage, "arw": KindImage, "dng": KindImage,
	// video
	"mp4": KindVideo, "mkv": KindVideo, "mov": KindVideo, "avi": KindVideo,
	"webm": KindVideo, "m4v": KindVideo, "mpg": KindVideo, "mpeg": KindVideo,
	"wmv": KindVideo, "flv": KindVideo, "ogv": KindVideo,
	// audio
	"mp3": KindAudio, "flac": KindAudio, "wav": KindAudio, "ogg": KindAudio,
	"opus": KindAudio, "m4a": KindAudio, "aac": KindAudio, "wma": KindAudio,
	"aiff": KindAudio, "mid": KindAudio, "midi": KindAudio,
	// plain text
	"txt": KindText, "md": KindText, "markdown": KindText, "rst": KindText,
	"org": KindText, "log": KindText, "csv": KindText, "tsv": KindText,
	"json": KindText, "yaml": KindText, "yml": KindText, "toml": KindText,
	"ini": KindText, "conf": KindText, "cfg": KindText, "xml": KindText,
	// source code
	"go": KindCode, "js": KindCode, "mjs": KindCode, "cjs": KindCode,
	"ts": KindCode, "jsx": KindCode, "tsx": KindCode, "py": KindCode,
	"rb": KindCode, "rs": KindCode, "c": KindCode, "h": KindCode,
	"cpp": KindCode, "hpp": KindCode, "cc": KindCode, "java": KindCode,
	"kt": KindCode, "kts": KindCode, "swift": KindCode, "php": KindCode,
	"sh": KindCode, "bash": KindCode, "zsh": KindCode, "fish": KindCode,
	"lua": KindCode, "sql": KindCode, "css": KindCode, "scss": KindCode,
	"sass": KindCode, "less": KindCode, "html": KindCode, "htm": KindCode,
	"vue": KindCode, "svelte": KindCode, "nim": KindCode, "zig": KindCode,
	"hs": KindCode, "ex": KindCode, "exs": KindCode, "erl": KindCode,
	"clj": KindCode, "dart": KindCode, "r": KindCode, "pl": KindCode,
	"gd": KindCode, "cs": KindCode, "vb": KindCode, "asm": KindCode,
	// documents
	"pdf": KindDoc, "doc": KindDoc, "docx": KindDoc, "odt": KindDoc,
	"rtf": KindDoc, "epub": KindDoc, "ppt": KindDoc, "pptx": KindDoc,
	"xls": KindDoc, "xlsx": KindDoc, "ods": KindDoc, "djvu": KindDoc,
	// archives
	"zip": KindArchive, "tar": KindArchive, "gz": KindArchive, "bz2": KindArchive,
	"xz": KindArchive, "zst": KindArchive, "7z": KindArchive, "rar": KindArchive,
	"iso": KindArchive, "dmg": KindArchive, "apk": KindArchive, "deb": KindArchive,
	"rpm": KindArchive,
}

// KindOf guesses a kind from the file name. Unknown extensions are "other".
func KindOf(name string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if k, ok := extKind[ext]; ok {
		return k
	}
	return KindOther
}

// Extension returns the lowercase extension without a dot.
func Extension(name string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
}

// ImageSize reads just the header of a decodable image. Zeroes mean "no idea",
// which is a perfectly normal answer for heic/avif/raw files.
func ImageSize(path string) (int, int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// makeSnippet keeps a short readable preview of text files. It is used in the
// result list, so newlines become separated dots rather than layout chaos.
func makeSnippet(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, " ")
	}
	var b strings.Builder
	n := 0
	space := false
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			r = ' '
		}
		if unicode.IsControl(r) {
			continue
		}
		if r == ' ' || r == '\u00a0' {
			if space || b.Len() == 0 {
				continue
			}
			space = true
			b.WriteByte(' ')
			continue
		}
		space = false
		b.WriteRune(r)
		n++
		if n >= 160 {
			break
		}
	}
	out := strings.TrimSpace(b.String())
	if utf8.RuneCountInString(out) >= 160 {
		out += "…"
	}
	return out
}
