package icon

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

func TestPNGDecodesWithRightColourInTheMiddle(t *testing.T) {
	for _, s := range []state.State{state.Red, state.Orange, state.Green, state.Grey} {
		img, err := png.Decode(bytes.NewReader(PNG(s)))
		if err != nil {
			t.Fatalf("%v: %v", s, err)
		}
		r, g, b, a := img.At(16, 16).RGBA()
		want := colours[s]
		if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B || a>>8 != 255 {
			t.Errorf("%v: centre pixel %d,%d,%d,%d", s, r>>8, g>>8, b>>8, a>>8)
		}
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Errorf("%v: corner should be transparent", s)
		}
	}
}

func TestICOHeader(t *testing.T) {
	p := PNG(state.Red)
	ico := ICO(p)
	if !bytes.Equal(ico[:6], []byte{0, 0, 1, 0, 1, 0}) {
		t.Fatalf("bad ICO header % x", ico[:6])
	}
	if !bytes.Equal(ico[22:], p) {
		t.Fatal("PNG payload not at offset 22")
	}
}
