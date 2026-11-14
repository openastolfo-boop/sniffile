package index

import "testing"

func TestTokenizeRussianAndDigits(t *testing.T) {
	got := Tokenize("Фотка с красивым мопсом 2024")
	want := []string{"фотк", "красив", "мопс", "2024"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %q, want %q (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestStem(t *testing.T) {
	cases := map[string]string{
		"мопсом":     "мопс",
		"красивым":   "красив",
		"фотографии": "фотограф",
		"photos":     "photo",
		"cities":     "city",
		"news":       "new",
		"2024":       "2024",
	}
	for in, want := range cases {
		if got := Stem(in); got != want {
			t.Errorf("Stem(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeCamelCase(t *testing.T) {
	got := Normalize("getUserAvatar2.png")
	want := []string{"get", "user", "avatar2", "png"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestKindOf(t *testing.T) {
	cases := map[string]string{
		"mops.JPG":        KindImage,
		"clip.mkv":        KindVideo,
		"podcast.opus":    KindAudio,
		"notes.md":        KindText,
		"main.go":         KindCode,
		"contract.pdf":    KindDoc,
		"backup.tar.gz":   KindArchive,
		"something.weird": KindOther,
	}
	for name, want := range cases {
		if got := KindOf(name); got != want {
			t.Errorf("KindOf(%q) = %q, want %q", name, got, want)
		}
	}
}
