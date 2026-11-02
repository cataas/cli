package render

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"strings"
	"testing"
)

func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestBlocks(t *testing.T) {
	img := solid(8, 8, color.RGBA{255, 0, 0, 255})
	// bottom half blue
	for y := 4; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{0, 0, 255, 255})
		}
	}
	out := Blocks(img, 4)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%q", len(lines), out)
	}
	if n := strings.Count(lines[0], "▀"); n != 4 {
		t.Errorf("got %d cells, want 4", n)
	}
	if !strings.Contains(lines[0], "\x1b[38;2;255;0;0m") || !strings.Contains(lines[1], "\x1b[48;2;0;0;255m") {
		t.Errorf("unexpected colours: %q", out)
	}
	// Colours are only emitted when they change.
	if n := strings.Count(lines[0], "\x1b[38;2"); n != 1 {
		t.Errorf("foreground emitted %d times, want 1", n)
	}
}

func TestBlocksNeverUpscales(t *testing.T) {
	out := Blocks(solid(3, 2, color.White), 80)
	if n := strings.Count(out, "▀"); n != 3 {
		t.Errorf("got %d cells, want 3", n)
	}
}

func TestFitColumns(t *testing.T) {
	r := image.Rect(0, 0, 100, 200) // portrait
	if got := fitColumns(r, 80, 10); got != 10 {
		t.Errorf("fitColumns = %d, want 10", got)
	}
	if got := fitColumns(r, 80, 0); got != 80 {
		t.Errorf("fitColumns without limit = %d, want 80", got)
	}
}

func TestDetect(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cases := []struct {
		env  map[string]string
		want Mode
	}{
		{map[string]string{"TERM": "xterm-kitty"}, ModeKitty},
		{map[string]string{"TERM_PROGRAM": "iTerm.app"}, ModeITerm},
		{map[string]string{"TERM": "xterm-256color"}, ModeBlocks},
	}
	for _, c := range cases {
		if got := Detect(env(c.env)); got != c.want {
			t.Errorf("Detect(%v) = %s, want %s", c.env, got, c.want)
		}
	}
}

func TestRenderModes(t *testing.T) {
	var data bytes.Buffer
	png.Encode(&data, solid(4, 4, color.Black))
	ctx := context.Background()

	for mode, prefix := range map[Mode]string{
		ModeKitty:  "\x1b_Ga=T,f=100",
		ModeITerm:  "\x1b]1337;File=inline=1",
		ModeBlocks: "\x1b[38;2;0;0;0m",
	} {
		var out bytes.Buffer
		if err := Render(ctx, &out, data.Bytes(), Options{Mode: mode, Columns: 4}); err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if !strings.HasPrefix(out.String(), prefix) {
			t.Errorf("%s: output starts with %q", mode, out.String()[:min(20, out.Len())])
		}
	}

	if err := Render(ctx, &bytes.Buffer{}, []byte("not an image"), Options{Mode: ModeBlocks}); err == nil {
		t.Error("expected a decoding error")
	}
}

func TestAnimatedGIF(t *testing.T) {
	pal := color.Palette{color.Black, color.White}
	g := &gif.GIF{}
	for i := 0; i < 3; i++ {
		f := image.NewPaletted(image.Rect(0, 0, 4, 4), pal)
		f.SetColorIndex(0, 0, uint8(i%2))
		g.Image = append(g.Image, f)
		g.Delay = append(g.Delay, 1)
	}
	var data bytes.Buffer
	if err := gif.EncodeAll(&data, g); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Render(context.Background(), &out, data.Bytes(), Options{Mode: ModeBlocks, Columns: 4, Loops: 2}); err != nil {
		t.Fatal(err)
	}
	// 6 frames drawn, the 5 last ones move the cursor back up 2 lines.
	if n := strings.Count(out.String(), "\x1b[2F"); n != 5 {
		t.Errorf("cursor moved up %d times, want 5", n)
	}
}

func TestRenderFitsBox(t *testing.T) {
	var data bytes.Buffer
	png.Encode(&data, solid(400, 300, color.White)) // 4:3
	ctx := context.Background()

	var out bytes.Buffer
	Render(ctx, &out, data.Bytes(), Options{Mode: ModeBlocks, Columns: 40})
	if lines := strings.Count(out.String(), "\n"); lines != 15 {
		t.Errorf("width 40: got %d lines, want 15", lines)
	}

	out.Reset()
	Render(ctx, &out, data.Bytes(), Options{Mode: ModeBlocks, Columns: 100, Rows: 6})
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 6 || strings.Count(lines[0], "▀") != 16 {
		t.Errorf("box 100x6: got %d lines of %d cells, want 6 of 16", len(lines), strings.Count(lines[0], "▀"))
	}

	out.Reset()
	Render(ctx, &out, data.Bytes(), Options{Mode: ModeITerm, Columns: 30, Rows: 8})
	if !strings.Contains(out.String(), "width=30;height=8;preserveAspectRatio=1") {
		t.Errorf("iterm: %q", out.String()[:80])
	}

	out.Reset()
	Render(ctx, &out, data.Bytes(), Options{Mode: ModeKitty, Columns: 100, Rows: 6})
	if !strings.Contains(out.String(), ",c=16,") {
		t.Errorf("kitty: %q", out.String()[:40])
	}
}
