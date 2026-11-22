package server

import (
	"encoding/binary"
	"io"
)

// jpegOrientation reads the EXIF orientation tag from a JPEG stream and
// returns 1..8 (1 is "upright"). Anything it cannot understand is 1, because a
// missing tag is far more common than a corrupt one.
//
// The decoder in image/jpeg ignores EXIF, so phone photos taken in portrait
// would otherwise be rendered sideways. We only need one tag, so this is a
// small APP1/TIFF reader instead of an EXIF dependency.
func jpegOrientation(r io.Reader) int {
	var soi [2]byte
	if _, err := io.ReadFull(r, soi[:]); err != nil || soi != [2]byte{0xFF, 0xD8} {
		return 1
	}
	for {
		var marker [2]byte
		if _, err := io.ReadFull(r, marker[:]); err != nil {
			return 1
		}
		if marker[0] != 0xFF {
			return 1 // not a marker: the file is not shaped like a JPEG
		}
		for marker[1] == 0xFF { // fill bytes are legal between segments
			if _, err := io.ReadFull(r, marker[1:2]); err != nil {
				return 1
			}
		}
		switch {
		case marker[1] == 0xD8 || marker[1] == 0xD9 || marker[1] == 0x01 ||
			(marker[1] >= 0xD0 && marker[1] <= 0xD7):
			continue // standalone marker, no length field
		case marker[1] == 0xDA: // start of scan: headers are over
			return 1
		}
		var lenBuf [2]byte
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			return 1
		}
		segLen := int(binary.BigEndian.Uint16(lenBuf[:]))
		if segLen < 2 {
			return 1
		}
		// APP1 carries EXIF; other APPn segments are skipped in one read.
		if marker[1] != 0xE1 {
			if _, err := io.CopyN(io.Discard, r, int64(segLen-2)); err != nil {
				return 1
			}
			continue
		}
		seg := make([]byte, segLen-2)
		if _, err := io.ReadFull(r, seg); err != nil {
			return 1
		}
		if len(seg) < 6 || string(seg[:6]) != "Exif\x00\x00" {
			continue // XMP or some other APP1 payload
		}
		if o := tiffOrientation(seg[6:]); o != 1 {
			return o
		}
	}
}

// tiffOrientation reads IFD0 of a TIFF/EXIF block and returns tag 0x0112.
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(t[2:4]) != 42 {
		return 1
	}
	off := int(order.Uint32(t[4:8]))
	if off < 8 || off+2 > len(t) {
		return 1
	}
	n := int(order.Uint16(t[off : off+2]))
	off += 2
	for i := 0; i < n; i++ {
		e := off + i*12
		if e+12 > len(t) {
			return 1
		}
		tag := order.Uint16(t[e : e+2])
		if tag != 0x0112 {
			continue
		}
		typ := order.Uint16(t[e+2 : e+4])
		count := order.Uint32(t[e+4 : e+8])
		if typ != 3 || count < 1 { // SHORT
			return 1
		}
		// A single SHORT fits in the value/offset field itself.
		o := int(order.Uint16(t[e+8 : e+10]))
		if o < 1 || o > 8 {
			return 1
		}
		return o
	}
	return 1
}
