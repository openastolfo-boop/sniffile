package server

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// jpegWithOrientation returns a tiny valid JPEG with an EXIF APP1 block whose
// orientation tag is set to o.
func jpegWithOrientation(t *testing.T, o int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 6), G: uint8(y * 12), B: 200, A: 255})
		}
	}
	var raw bytes.Buffer
	if err := jpeg.Encode(&raw, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	tiff := []byte{
		'I', 'I', 42, 0, 8, 0, 0, 0, // little-endian TIFF header, IFD at 8
		1, 0, // one entry
		0x12, 0x01, 3, 0, 1, 0, 0, 0, byte(o), 0, 0, 0, // tag 0x0112, SHORT
		0, 0, 0, 0, // next IFD
	}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	app1 := make([]byte, 4+len(payload))
	app1[0], app1[1] = 0xFF, 0xE1
	binary.BigEndian.PutUint16(app1[2:4], uint16(len(payload)+2))
	copy(app1[4:], payload)

	src := raw.Bytes()
	out := make([]byte, 0, len(src)+len(app1))
	out = append(out, src[:2]...) // SOI
	out = append(out, app1...)    // EXIF
	out = append(out, src[2:]...)
	return out
}

func TestJPEGOrientation(t *testing.T) {
	for want := 1; want <= 8; want++ {
		data := jpegWithOrientation(t, want)
		if got := jpegOrientation(bytes.NewReader(data)); got != want {
			t.Errorf("orientation = %d, want %d", got, want)
		}
	}
	if got := jpegOrientation(bytes.NewReader([]byte("not a jpeg"))); got != 1 {
		t.Errorf("non-JPEG orientation = %d, want 1", got)
	}
	// Truncated APP1 must not panic or invent a rotation.
	data := jpegWithOrientation(t, 6)
	if got := jpegOrientation(bytes.NewReader(data[:20])); got != 1 {
		t.Errorf("truncated orientation = %d, want 1", got)
	}
}

func TestRenderThumbRespectsEXIFOrientation(t *testing.T) {
	for _, tc := range []struct {
		orientation int
		w, h        int
	}{
		{1, 40, 20},
		{3, 40, 20},
		{6, 20, 40},
		{8, 20, 40},
	} {
		path := filepath.Join(t.TempDir(), "phone.jpg")
		if err := os.WriteFile(path, jpegWithOrientation(t, tc.orientation), 0o644); err != nil {
			t.Fatal(err)
		}
		thumb, err := renderThumb(path)
		if err != nil {
			t.Fatalf("orientation %d: %v", tc.orientation, err)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(thumb))
		if err != nil {
			t.Fatalf("orientation %d: %v", tc.orientation, err)
		}
		if cfg.Width != tc.w || cfg.Height != tc.h {
			t.Errorf("orientation %d: thumbnail %dx%d, want %dx%d",
				tc.orientation, cfg.Width, cfg.Height, tc.w, tc.h)
		}
	}
}

func TestApplyOrientationMapsPixels(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			src.Set(x, y, color.RGBA{R: uint8(x*10 + y), A: 255})
		}
	}
	// Orientation 6 rotates 90 degrees clockwise: top-left becomes top-right.
	dst := applyOrientation(src, 6)
	if dst.Bounds().Dx() != 3 || dst.Bounds().Dy() != 2 {
		t.Fatalf("bounds = %v, want 3x2", dst.Bounds())
	}
	got := color.RGBAModel.Convert(dst.At(2, 0)).(color.RGBA)
	want := color.RGBAModel.Convert(src.At(0, 0)).(color.RGBA)
	if got != want {
		t.Errorf("rotated pixel = %v, want %v", got, want)
	}
	if applyOrientation(src, 1) != src {
		t.Error("orientation 1 should return the source image")
	}
}
