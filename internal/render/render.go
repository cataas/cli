// Package render draws images in a terminal.
//
// Three protocols are supported:
//   - kitty:  the Kitty graphics protocol (Kitty, Ghostty, WezTerm, Konsole...)
//   - iterm:  the iTerm2 inline images protocol (iTerm2, WezTerm, VS Code...)
//   - blocks: 24-bit ANSI colours with half-block characters, works almost everywhere
package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	_ "image/jpeg" // register decoders
	"image/png"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

// Mode is a rendering protocol.
type Mode string

const (
	ModeAuto   Mode = "auto"
	ModeKitty  Mode = "kitty"
	ModeITerm  Mode = "iterm"
	ModeBlocks Mode = "blocks"
)

// Modes lists the accepted values for Mode.
var Modes = []Mode{ModeAuto, ModeKitty, ModeITerm, ModeBlocks}

// ParseMode validates s.
func ParseMode(s string) (Mode, error) {
	for _, m := range Modes {
		if string(m) == s {
			return m, nil
		}
	}
	return "", fmt.Errorf("unknown render mode %q (expected one of auto, kitty, iterm, blocks)", s)
}

// Detect guesses the best protocol from the environment.
func Detect(getenv func(string) string) Mode {
	term := getenv("TERM")
	prog := getenv("TERM_PROGRAM")
	switch {
	case getenv("KITTY_WINDOW_ID") != "", term == "xterm-kitty", term == "xterm-ghostty", prog == "ghostty":
		return ModeKitty
	case prog == "iTerm.app", prog == "WezTerm", prog == "vscode", getenv("LC_TERMINAL") == "iTerm2":
		return ModeITerm
	}
	return ModeBlocks
}

// Options controls how an image is drawn.
type Options struct {
	Mode Mode
	// The picture is scaled, keeping its aspect ratio, to fit in a box of
	// Columns × Rows terminal cells. Rows == 0 means no height limit.
	Columns int
	Rows    int
	Loops   int // how many times animated GIFs are played (blocks mode); 0 means once
}

// Render decodes data and writes it to w.
func Render(ctx context.Context, w io.Writer, data []byte, opts Options) error {
	if opts.Columns <= 0 {
		opts.Columns = 80
	}
	mode := opts.Mode
	if mode == "" || mode == ModeAuto {
		mode = Detect(os.Getenv)
	}

	switch mode {
	case ModeITerm:
		// iTerm2 decodes (and animates) the original file itself.
		return writeITerm(w, data, opts.Columns, opts.Rows)
	case ModeKitty:
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("decoding image: %w", err)
		}
		return writeKitty(w, img, fitColumns(img.Bounds(), opts.Columns, opts.Rows))
	default:
		if isGIF(data) {
			g, err := gif.DecodeAll(bytes.NewReader(data))
			if err == nil && len(g.Image) > 1 {
				return animateBlocks(ctx, w, g, opts)
			}
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("decoding image: %w", err)
		}
		cols := fitColumns(img.Bounds(), opts.Columns, opts.Rows)
		_, err = io.WriteString(w, Blocks(img, cols))
		return err
	}
}

func isGIF(data []byte) bool {
	return bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))
}

// fitColumns reduces columns so that a picture of size b takes at most
// maxRows lines, assuming terminal cells are twice as tall as wide.
func fitColumns(b image.Rectangle, columns, maxRows int) int {
	if maxRows <= 0 || b.Dx() == 0 || b.Dy() == 0 {
		return columns
	}
	if limit := 2 * maxRows * b.Dx() / b.Dy(); limit < columns {
		return max(limit, 1)
	}
	return columns
}

// Blocks renders img with "▀" characters: each cell shows two vertical pixels,
// the upper one as foreground colour and the lower one as background colour.
func Blocks(img image.Image, columns int) string {
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return ""
	}
	if columns > b.Dx() {
		columns = b.Dx()
	}
	// Terminal cells are about twice as tall as wide, and each holds 2 pixels
	// vertically, so pixel rows map 1:1 to the horizontal scale.
	rows := (b.Dy()*columns/b.Dx() + 1) / 2 * 2
	if rows < 2 {
		rows = 2
	}
	px := scale(img, columns, rows)

	var sb strings.Builder
	sb.Grow(columns * rows / 2 * 40)
	for y := 0; y < rows; y += 2 {
		var prevTop, prevBot color.RGBA
		for x := 0; x < columns; x++ {
			top, bot := px[y*columns+x], px[(y+1)*columns+x]
			if x == 0 || top != prevTop {
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm", top.R, top.G, top.B)
			}
			if x == 0 || bot != prevBot {
				fmt.Fprintf(&sb, "\x1b[48;2;%d;%d;%dm", bot.R, bot.G, bot.B)
			}
			sb.WriteString("▀")
			prevTop, prevBot = top, bot
		}
		sb.WriteString("\x1b[0m\n")
	}
	return sb.String()
}

// scale resizes img to w×h with a box filter (averaging source pixels),
// compositing transparency over black.
func scale(img image.Image, w, h int) []color.RGBA {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	out := make([]color.RGBA, w*h)
	for y := 0; y < h; y++ {
		y0 := b.Min.Y + y*sh/h
		y1 := max(b.Min.Y+(y+1)*sh/h, y0+1)
		for x := 0; x < w; x++ {
			x0 := b.Min.X + x*sw/w
			x1 := max(b.Min.X+(x+1)*sw/w, x0+1)
			var r, g, bl, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					cr, cg, cb, _ := img.At(sx, sy).RGBA() // premultiplied: over black
					r, g, bl, n = r+uint64(cr), g+uint64(cg), bl+uint64(cb), n+1
				}
			}
			out[y*w+x] = color.RGBA{uint8(r / n >> 8), uint8(g / n >> 8), uint8(bl / n >> 8), 0xff}
		}
	}
	return out
}

func animateBlocks(ctx context.Context, w io.Writer, g *gif.GIF, opts Options) error {
	frames := composeGIF(g)
	cols := fitColumns(frames[0].Bounds(), opts.Columns, opts.Rows)
	loops := max(opts.Loops, 1)
	lines := 0
	io.WriteString(w, "\x1b[?25l")       // hide cursor
	defer io.WriteString(w, "\x1b[?25h") // show cursor
	for l := 0; l < loops; l++ {
		for i, frame := range frames {
			if lines > 0 {
				fmt.Fprintf(w, "\x1b[%dF", lines) // back to the first line of the picture
			}
			out := Blocks(frame, cols)
			lines = strings.Count(out, "\n")
			if _, err := io.WriteString(w, out); err != nil {
				return err
			}
			delay := time.Duration(g.Delay[i]) * 10 * time.Millisecond
			if delay <= 0 {
				delay = 100 * time.Millisecond
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}
		}
	}
	return nil
}

// composeGIF turns GIF frames (often partial) into full images, honouring
// the disposal methods.
func composeGIF(g *gif.GIF) []image.Image {
	bounds := image.Rect(0, 0, g.Config.Width, g.Config.Height)
	if bounds.Empty() {
		bounds = g.Image[0].Bounds()
	}
	canvas := image.NewRGBA(bounds)
	frames := make([]image.Image, 0, len(g.Image))
	for i, frame := range g.Image {
		var previous *image.RGBA
		disposal := byte(0)
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		if disposal == gif.DisposalPrevious {
			previous = image.NewRGBA(bounds)
			copy(previous.Pix, canvas.Pix)
		}
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		snapshot := image.NewRGBA(bounds)
		copy(snapshot.Pix, canvas.Pix)
		frames = append(frames, snapshot)

		switch disposal {
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			canvas = previous
		}
	}
	return frames
}

// writeITerm uses the iTerm2 inline images protocol.
// The terminal fits the picture in the columns × rows box, keeping its ratio.
func writeITerm(w io.Writer, data []byte, columns, rows int) error {
	height := "auto"
	if rows > 0 {
		height = strconv.Itoa(rows)
	}
	_, err := fmt.Fprintf(w, "\x1b]1337;File=inline=1;size=%d;width=%d;height=%s;preserveAspectRatio=1:%s\a\n",
		len(data), columns, height, base64.StdEncoding.EncodeToString(data))
	return err
}

// writeKitty uses the Kitty graphics protocol, sending a PNG in 4 KiB chunks.
func writeKitty(w io.Writer, img image.Image, columns int) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	payload := base64.StdEncoding.EncodeToString(buf.Bytes())
	const chunk = 4096
	for i := 0; i < len(payload); i += chunk {
		end := min(i+chunk, len(payload))
		more := 0
		if end < len(payload) {
			more = 1
		}
		var err error
		if i == 0 {
			_, err = fmt.Fprintf(w, "\x1b_Ga=T,f=100,q=2,c=%d,m=%d;%s\x1b\\", columns, more, payload[i:end])
		} else {
			_, err = fmt.Fprintf(w, "\x1b_Gm=%d;%s\x1b\\", more, payload[i:end])
		}
		if err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}
