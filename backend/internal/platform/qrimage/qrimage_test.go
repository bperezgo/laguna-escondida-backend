package qrimage

import (
	"bytes"
	"image/png"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderPNG_ARealSandboxQR(t *testing.T) {
	svg, err := os.ReadFile("testdata/wompi_sandbox_qr.svg")
	require.NoError(t, err)

	out, err := RenderPNG(string(svg))

	require.NoError(t, err)
	img, err := png.Decode(bytes.NewReader(out))
	require.NoError(t, err)
	assert.Equal(t, 97*pixelsPerModule, img.Bounds().Dx())
	assert.Equal(t, 101*pixelsPerModule, img.Bounds().Dy())
	assert.Less(t, len(out), 20<<10, "a few KB, not the 111 KB of the SVG")

	modules := len(regexp.MustCompile(`M\d+,\d+h1v1h-1z`).FindAllString(string(svg), -1))
	dark := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if r, _, _, _ := img.At(x, y).RGBA(); r == 0 {
				dark++
			}
		}
	}
	assert.Equal(t, modules*pixelsPerModule*pixelsPerModule, dark, "every module is drawn, nothing else")
}

func TestRenderPNG_ATinyQR(t *testing.T) {
	svg := `<svg viewBox="0 0 3 3"><rect width="100%" height="100%" fill="#FFFFFF"/><path d="M0,0h1v1h-1z M2,2h1v1h-1z" fill="#000000"/></svg>`

	out, err := RenderPNG(svg)

	require.NoError(t, err)
	img, err := png.Decode(bytes.NewReader(out))
	require.NoError(t, err)
	black := func(x, y int) bool { r, _, _, _ := img.At(x, y).RGBA(); return r == 0 }
	assert.True(t, black(0, 0))
	assert.True(t, black(2*pixelsPerModule, 2*pixelsPerModule))
	assert.False(t, black(pixelsPerModule, pixelsPerModule))
}

func TestRenderPNG_Unreadable(t *testing.T) {
	for name, svg := range map[string]string{
		"not an SVG":         "hello",
		"no viewBox":         `<svg><path d="M0,0h1v1h-1z" fill="#000000"/></svg>`,
		"no QR path":         `<svg viewBox="0 0 3 3"></svg>`,
		"no modules":         `<svg viewBox="0 0 3 3"><path d="M0,0L3,3" fill="#000000"/></svg>`,
		"a module outside":   `<svg viewBox="0 0 3 3"><path d="M5,5h1v1h-1z" fill="#000000"/></svg>`,
		"an enormous canvas": `<svg viewBox="0 0 90000 90000"><path d="M0,0h1v1h-1z" fill="#000000"/></svg>`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := RenderPNG(svg)

			require.ErrorIs(t, err, ErrUnreadableQR)
		})
	}
}
