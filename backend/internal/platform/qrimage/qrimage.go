// Package qrimage turns the SVG Wompi returns for a Bancolombia QR into a PNG.
//
// A general SVG library fails on it: Wompi's logo and footer use percentage sizes. The QR itself
// is a path of one-module squares ("M{x},{y}h1v1h-1z"), so the PNG is drawn from those squares
// with the standard library. Wompi's logo is left out, which a QR reader does not need.
package qrimage

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"regexp"
	"strconv"
)

const (
	// pixelsPerModule keeps the image small (a few KB) and sharp enough for a phone's bank app.
	pixelsPerModule = 8
	maxModules      = 400
)

var (
	viewBox  = regexp.MustCompile(`viewBox="0 0 (\d+) (\d+)"`)
	qrPath   = regexp.MustCompile(`<path d="([^"]+)" fill="#000000"`)
	moduleAt = regexp.MustCompile(`M(\d+),(\d+)h1v1h-1z`)
)

// ErrUnreadableQR means the SVG is not the shape Wompi's QR has.
var ErrUnreadableQR = errors.New("the QR is not in the shape Wompi returns it")

// RenderPNG draws the QR's modules black on white.
func RenderPNG(svg string) ([]byte, error) {
	size := viewBox.FindStringSubmatch(svg)
	path := qrPath.FindStringSubmatch(svg)
	if size == nil || path == nil {
		return nil, ErrUnreadableQR
	}
	width, _ := strconv.Atoi(size[1])
	height, _ := strconv.Atoi(size[2])
	if width < 1 || height < 1 || width > maxModules || height > maxModules {
		return nil, ErrUnreadableQR
	}
	modules := moduleAt.FindAllStringSubmatch(path[1], -1)
	if len(modules) == 0 {
		return nil, ErrUnreadableQR
	}

	img := image.NewGray(image.Rect(0, 0, width*pixelsPerModule, height*pixelsPerModule))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	black := &image.Uniform{C: color.Black}
	for _, m := range modules {
		x, _ := strconv.Atoi(m[1])
		y, _ := strconv.Atoi(m[2])
		if x >= width || y >= height {
			return nil, ErrUnreadableQR
		}
		square := image.Rect(x*pixelsPerModule, y*pixelsPerModule, (x+1)*pixelsPerModule, (y+1)*pixelsPerModule)
		draw.Draw(img, square, black, image.Point{}, draw.Src)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
