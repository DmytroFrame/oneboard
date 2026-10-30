// Package imagethumb makes small JPEG previews of uploaded images.
package imagethumb

import (
	"bufio"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"

	_ "image/gif" // register decoders
	_ "image/png"

	"go.uber.org/fx"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"clodapp/backend/internal/usecase/attachment"
)

var Module = fx.Module("thumbnails", fx.Provide(func() attachment.Thumbnailer { return Thumbnailer{} }))

const (
	maxSide = 480 // card covers are ~240 CSS px wide; 2x for retina
	// maxPixels guards memory: decoding is ~4 bytes per pixel, so 40 MP ≈ 160 MB.
	maxPixels = 40_000_000
)

var ErrTooLarge = errors.New("image too large for a thumbnail")

type Thumbnailer struct{}

func (Thumbnailer) Thumbnail(r io.Reader) ([]byte, error) {
	br := bufio.NewReaderSize(r, 64<<10)
	// Peek at the header first, so a 20000×20000 PNG is rejected before it is decoded.
	header, err := br.Peek(64 << 10)
	if err != nil && err != io.EOF && err != bufio.ErrBufferFull {
		return nil, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(header))
	if err != nil {
		return nil, err
	}
	if cfg.Width*cfg.Height > maxPixels {
		return nil, ErrTooLarge
	}
	src, _, err := image.Decode(br)
	if err != nil {
		return nil, err
	}

	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > maxSide || h > maxSide {
		if w >= h {
			w, h = maxSide, max(1, h*maxSide/w)
		} else {
			w, h = max(1, w*maxSide/h), maxSide
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	// JPEG has no alpha: put transparent images on white rather than black.
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
