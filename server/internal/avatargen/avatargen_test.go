package avatargen

import (
	"image"
	"image/color"
	"testing"
)

// makeSkin builds a 64×64 test skin: solid per-face colors so renders are
// assertable by dominant color; the head front face is magenta.
func makeSkin() *image.NRGBA {
	s := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	fill := func(x0, y0, w, h int, c color.NRGBA) {
		for y := y0; y < y0+h; y++ {
			for x := x0; x < x0+w; x++ {
				s.SetNRGBA(x, y, c)
			}
		}
	}
	// Head faces.
	fill(8, 8, 8, 8, color.NRGBA{255, 0, 255, 255})   // front
	fill(0, 8, 8, 8, color.NRGBA{255, 0, 0, 255})    // right side
	fill(16, 8, 8, 8, color.NRGBA{0, 255, 0, 255})   // left side
	fill(24, 8, 8, 8, color.NRGBA{0, 0, 255, 255})   // back
	fill(8, 0, 8, 8, color.NRGBA{255, 255, 0, 255})  // top
	fill(16, 0, 8, 8, color.NRGBA{0, 255, 255, 255}) // bottom
	// Body + limbs (opaque so full-body renders have content everywhere).
	fill(16, 16, 24, 16, color.NRGBA{128, 128, 128, 255})
	fill(0, 16, 16, 16, color.NRGBA{96, 96, 96, 255})
	fill(40, 16, 16, 16, color.NRGBA{96, 96, 96, 255})
	// Left-limb net (v≥48) so 64×64 path is covered.
	fill(16, 48, 16, 16, color.NRGBA{64, 64, 64, 255})
	fill(32, 48, 16, 16, color.NRGBA{64, 64, 64, 255})
	return s
}

func TestRenderAvatar2DFrontFaceDominant(t *testing.T) {
	skin := makeSkin()
	out := RenderAvatar(skin, 64, false)
	if out.Bounds().Dx() != 64 || out.Bounds().Dy() != 64 {
		t.Fatalf("avatar size = %v, want 64×64", out.Bounds())
	}
	// Center pixel must be the magenta front face (no side bleed at rot 0).
	c := out.NRGBAAt(32, 32)
	if c.R < 200 || c.B < 200 || c.G > 50 {
		t.Fatalf("center pixel = %v, want magenta front face", c)
	}
}

func TestRenderAvatar3DHasSideAndTop(t *testing.T) {
	skin := makeSkin()
	out := RenderAvatar(skin, 96, true)
	if out.Bounds().Dx() != 96 || out.Bounds().Dy() != 96 {
		t.Fatalf("3d avatar size = %v, want 96×96", out.Bounds())
	}
	// At -45° yaw the render shows front (UV 8,8 magenta) on the left and
	// the head's +x face (UV 16,8 green) on the right, plus the top
	// (UV 8,0 yellow) — matching the live legacy probe orientation.
	var magenta, green, yellow int
	for y := 0; y < 96; y++ {
		for x := 0; x < 96; x++ {
			c := out.NRGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			switch {
			case c.R > 200 && c.B > 200 && c.G < 50:
				magenta++
			case c.G > 200 && c.R < 50 && c.B < 50:
				green++
			case c.R > 200 && c.G > 200 && c.B < 50:
				yellow++
			}
		}
	}
	if magenta == 0 || green == 0 || yellow == 0 {
		t.Fatalf("3d render faces: front=%d side=%d top=%d, all must be >0", magenta, green, yellow)
	}
}

func TestRenderAvatarLegacy64x32(t *testing.T) {
	s := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			s.SetNRGBA(x, y, color.NRGBA{200, 100, 50, 255})
		}
	}
	for y := 8; y < 16; y++ {
		for x := 8; x < 16; x++ {
			s.SetNRGBA(x, y, color.NRGBA{255, 0, 255, 255})
		}
	}
	out := RenderAvatar(s, 48, true)
	if out.Bounds().Dx() != 48 {
		t.Fatalf("legacy avatar width = %d, want 48", out.Bounds().Dx())
	}
	// Must not panic on missing layer-2 region and must have opaque content.
	opaque := 0
	for y := 0; y < 48; y++ {
		for x := 0; x < 48; x++ {
			if out.NRGBAAt(x, y).A == 255 {
				opaque++
			}
		}
	}
	if opaque < 100 {
		t.Fatalf("legacy render too empty: %d opaque px", opaque)
	}
}

func TestRenderPreviewDimensions(t *testing.T) {
	skin := makeSkin()
	out := RenderPreview(skin, false)
	b := out.Bounds()
	// Fixed-ratio output (legacy ignores ?height; probe 100/400 → 430×437).
	if b.Dy() < 300 || b.Dy() > 500 {
		t.Fatalf("preview height = %d, want ≈430 (300..500)", b.Dy())
	}
	if b.Dx() < b.Dy()/2 {
		t.Fatalf("preview width %d too small for two side-by-side views", b.Dx())
	}
	// Both views must have opaque pixels (back left half, front right half).
	left, right := 0, 0
	mid := b.Dx() / 2
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			if out.NRGBAAt(x, y).A != 0 {
				if x < mid {
					left++
				} else {
					right++
				}
			}
		}
	}
	if left == 0 || right == 0 {
		t.Fatalf("preview views missing: left=%d right=%d", left, right)
	}
}

func TestRenderPreviewSlim(t *testing.T) {
	skin := makeSkin()
	// Slim path must not panic and must produce content.
	out := RenderPreview(skin, true)
	if out.Bounds().Dx() == 0 {
		t.Fatal("slim preview empty canvas")
	}
}

func TestDefaultAvatar(t *testing.T) {
	for _, threeD := range []bool{false, true} {
		img, err := DefaultAvatar(64, threeD)
		if err != nil {
			t.Fatalf("DefaultAvatar(3d=%v): %v", threeD, err)
		}
		if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
			t.Fatalf("default avatar size = %v", img.Bounds())
		}
	}
}

func TestRenderAvatarEmptySkinNoPanic(t *testing.T) {
	// Fully transparent 64×64: must not panic; output stays sized.
	s := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	out := RenderAvatar(s, 32, true)
	if out.Bounds().Dx() != 32 {
		t.Fatalf("empty-skin avatar width = %d", out.Bounds().Dx())
	}
}
