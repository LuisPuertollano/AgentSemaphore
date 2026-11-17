// Package icon draws the tray dot for each state at runtime (no image files).
package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

// Colours of the dot. Grey is mid-grey so it reads as "no data" on light and dark taskbars.
var colours = map[state.State]color.NRGBA{
	state.Red:    {220, 38, 38, 255},
	state.Orange: {245, 140, 20, 255},
	state.Green:  {34, 170, 80, 255},
	state.Grey:   {140, 140, 140, 255},
}

const size = 32

// Colour returns the fill colour of the dot for s (grey for unknown values).
// Tests use it to tell which state a tray icon shows.
func Colour(s state.State) color.NRGBA {
	if c, ok := colours[s]; ok {
		return c
	}
	return colours[state.Grey]
}

// PNG returns a 32×32 anti-aliased dot with a thin dark outline (the tray icon).
func PNG(s state.State) []byte { return PNGSize(s, size) }

// PNGSize draws the same dot at any size (used for documentation images).
func PNGSize(s state.State, size int) []byte {
	c, ok := colours[s]
	if !ok {
		c = colours[state.Grey]
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	r := float64(size) * 14 / 32 // radius of the fill
	edge := float64(size) * 1.5 / 32
	center := float64(size) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := math.Hypot(float64(x)+0.5-center, float64(y)+0.5-center)
			alpha := clamp(r + 0.5 - d) // anti-aliased disc
			if alpha <= 0 {
				continue
			}
			px := c
			if d > r-edge { // outline ring
				px = color.NRGBA{20, 20, 20, 255}
			}
			px.A = uint8(float64(px.A) * alpha)
			img.SetNRGBA(x, y, px)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// ICO wraps a PNG in a single-image .ico container (PNG-in-ICO, Windows Vista+),
// which is what the Windows tray API wants.
func ICO(pngData []byte) []byte {
	var buf bytes.Buffer
	w := func(v any) { _ = binary.Write(&buf, binary.LittleEndian, v) }
	w(uint16(0))            // reserved
	w(uint16(1))            // type: icon
	w(uint16(1))            // image count
	w(uint8(size))          // width
	w(uint8(size))          // height
	w(uint8(0))             // palette
	w(uint8(0))             // reserved
	w(uint16(1))            // colour planes
	w(uint16(32))           // bits per pixel
	w(uint32(len(pngData))) // data size
	w(uint32(6 + 16))       // data offset
	buf.Write(pngData)
	return buf.Bytes()
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
