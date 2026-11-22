package server

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math"
	"os"
	"sync"
	"time"
)

var errNoPreview = errors.New("cannot decode image")

type thumbItem struct {
	data  []byte
	mtime int64
	size  int64
	at    time.Time
}

// thumbCache is a deliberately dumb size-bounded map. A personal search tool
// does not need an LRU with a linked list, it needs to not eat all the RAM.
type thumbCache struct {
	mu    sync.Mutex
	items map[string]*thumbItem
	max   int
}

func newThumbCache(max int) *thumbCache {
	if max <= 0 {
		max = 128
	}
	return &thumbCache{items: make(map[string]*thumbItem, max), max: max}
}

func (c *thumbCache) get(path string, mtime, size int64) ([]byte, error) {
	c.mu.Lock()
	if it, ok := c.items[path]; ok && it.mtime == mtime && it.size == size {
		data := it.data
		c.mu.Unlock()
		return data, nil
	}
	c.mu.Unlock()

	data, err := renderThumb(path)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	if len(c.items) >= c.max {
		drop := c.max/4 + 1
		for k := range c.items {
			delete(c.items, k)
			drop--
			if drop <= 0 {
				break
			}
		}
	}
	c.items[path] = &thumbItem{data: data, mtime: mtime, size: size, at: time.Now()}
	c.mu.Unlock()
	return data, nil
}

// renderThumb produces a grayscale JPEG. Grayscale is a style decision (the
// whole app is black and white) and also a size decision: ~8x smaller.
func renderThumb(path string) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.Size() > 48<<20 {
		return nil, errNoPreview
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Phone cameras store the sensor image in landscape and describe the real
	// orientation in EXIF. Resizing drops all metadata, so bake the rotation in
	// before the JPEG is written. Read it first, then rewind for the decoder.
	orientation := 1
	if _, err := f.Seek(0, io.SeekStart); err == nil {
		orientation = jpegOrientation(f)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, errNoPreview
	}

	src, _, err := image.Decode(f)
	if err != nil {
		return nil, errNoPreview
	}
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 || b.Dx()*b.Dy() > 80_000_000 {
		return nil, errNoPreview
	}
	small := boxScale(src, 360)
	if small == nil {
		return nil, errNoPreview
	}
	return encodeThumb(applyOrientation(small, orientation))
}

// encodeThumb writes the grayscale JPEG served by /api/thumb.
func encodeThumb(src image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, toGray(src), &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// applyOrientation maps the eight EXIF orientations onto pixel coordinates.
// It runs after downscaling, so the extra image is tiny rather than a second
// full-resolution copy of the photo.
func applyOrientation(src image.Image, orientation int) image.Image {
	if orientation < 2 || orientation > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if orientation >= 5 { // 5..8 transpose the axes
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch orientation {
			case 2: // mirror horizontal
				dx, dy = w-1-x, y
			case 3: // rotate 180
				dx, dy = w-1-x, h-1-y
			case 4: // mirror vertical
				dx, dy = x, h-1-y
			case 5: // transpose
				dx, dy = y, x
			case 6: // rotate 90 clockwise
				dx, dy = h-1-y, x
			case 7: // transverse
				dx, dy = h-1-y, w-1-x
			case 8: // rotate 270 clockwise
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// boxScale is a small area-average resizer. Bilinear would be prettier; this
// is 40 lines and looks fine at 360px.
func boxScale(src image.Image, maxSide int) image.Image {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= maxSide && sh <= maxSide {
		return src
	}
	scale := math.Min(float64(maxSide)/float64(sw), float64(maxSide)/float64(sh))
	dw, dh := int(math.Round(float64(sw)*scale)), int(math.Round(float64(sh)*scale))
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		sy0 := b.Min.Y + y*sh/dh
		sy1 := b.Min.Y + (y+1)*sh/dh
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for x := 0; x < dw; x++ {
			sx0 := b.Min.X + x*sw/dw
			sx1 := b.Min.X + (x+1)*sw/dw
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var r, g, bl, a, n uint64
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					cr, cg, cb, ca := src.At(sx, sy).RGBA()
					r += uint64(cr)
					g += uint64(cg)
					bl += uint64(cb)
					a += uint64(ca)
					n++
				}
			}
			if n == 0 {
				continue
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8((r / n) >> 8),
				G: uint8((g / n) >> 8),
				B: uint8((bl / n) >> 8),
				A: uint8((a / n) >> 8),
			})
		}
	}
	return dst
}

func toGray(src image.Image) *image.Gray {
	b := src.Bounds()
	dst := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := src.At(x, y).RGBA()
			l := float64(((299*r+587*g+114*bl)/1000)>>8) / 255
			l = (l-0.5)*1.06 + 0.5
			switch {
			case l < 0:
				l = 0
			case l > 1:
				l = 1
			}
			dst.SetGray(x-b.Min.X, y-b.Min.Y, color.Gray{Y: uint8(l * 255)})
		}
	}
	return dst
}
