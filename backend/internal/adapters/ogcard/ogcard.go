// Package ogcard draws the image shown when a link is shared on WhatsApp and
// other apps, which read it from the page instead of running the site.
package ogcard

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/unicode/norm"
)

const (
	Width  = 1200
	Height = 630
	margin = 72
	// Text stops short of the stripes on the right.
	textWidth = 820
)

var (
	//go:embed display.ttf
	displayTTF []byte
	//go:embed body.ttf
	bodyTTF []byte
	//go:embed logo.png
	logoPNG []byte

	displayFont = mustFont(displayTTF)
	bodyFont    = mustFont(bodyTTF)
	logo        = mustImage(logoPNG)

	ink   = color.RGBA{0x0B, 0x12, 0x0E, 0xFF}
	white = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	muted = color.RGBA{0xB9, 0xC3, 0xBD, 0xFF}
	lime  = color.RGBA{0x8E, 0xE0, 0x7F, 0xFF}
	green = color.RGBA{0x0A, 0x82, 0x05, 0xFF}
	red   = color.RGBA{0xFF, 0x0F, 0x15, 0xFF}
)

// Card is the content of one image. Anything left empty is left out.
type Card struct {
	Kicker string // the small line above the title
	Title  string
	Body   string
	Big    string // the amount
	Foot   string
}

func mustFont(b []byte) *opentype.Font {
	f, err := opentype.Parse(b)
	if err != nil {
		panic(err)
	}
	return f
}

func mustImage(b []byte) image.Image {
	i, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	return i
}

func face(f *opentype.Font, size float64) font.Face {
	x, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err)
	}
	return x
}

// Render draws a card as a PNG.
func Render(c Card) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	draw.Draw(img, img.Bounds(), image.NewUniform(ink), image.Point{}, draw.Src)
	stripe(img, 900, 90, green)
	stripe(img, 1040, 90, red)

	draw.Draw(img, image.Rect(margin, 56, margin+76, 132), logo, logo.Bounds().Min, draw.Over)
	y := 132 // the bottom of what is drawn so far
	if c.Kicker != "" {
		y = text(img, bodyFont, 36, muted, c.Kicker, y+58, 1, 1.25)
	}
	if c.Title != "" {
		size, lines := fit(displayFont, c.Title, []float64{84, 72, 60, 50})
		y = textLines(img, displayFont, size, white, lines, y+int(size*1.05), 1.08)
	}
	if c.Body != "" {
		y = text(img, bodyFont, 34, muted, c.Body, y+58, 2, 1.28)
	}
	if c.Big != "" {
		text(img, displayFont, 96, lime, c.Big, Height-124, 1, 1)
	}
	if c.Foot != "" {
		text(img, bodyFont, 30, muted, c.Foot, Height-56, 1, 1)
	}
	var out bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func stripe(img *image.RGBA, x, w int, c color.RGBA) {
	for y := 0; y < Height; y++ {
		left := x + (Height-y)*2/5 - 120
		draw.Draw(img, image.Rect(left, y, left+w, y+1), image.NewUniform(c), image.Point{}, draw.Src)
	}
}

func fit(f *opentype.Font, s string, sizes []float64) (float64, []string) {
	s = clean(f, s)
	for _, size := range sizes {
		if lines := wrap(face(f, size), s, textWidth); len(lines) <= 2 {
			return size, lines
		}
	}
	size := sizes[len(sizes)-1]
	return size, limit(face(f, size), wrap(face(f, size), s, textWidth), 2)
}

func text(img *image.RGBA, f *opentype.Font, size float64, c color.RGBA, s string, y, maxLines int, leading float64) int {
	x := face(f, size)
	lines := limit(x, wrap(x, clean(f, s), textWidth), maxLines)
	return textLines(img, f, size, c, lines, y, leading)
}

func textLines(img *image.RGBA, f *opentype.Font, size float64, c color.RGBA, lines []string, y int, leading float64) int {
	x := face(f, size)
	d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: x}
	for i, l := range lines {
		if i > 0 {
			y += int(size * leading)
		}
		d.Dot = fixed.P(margin, y)
		d.DrawString(l)
	}
	return y
}

func wrap(x font.Face, s string, width int) []string {
	var lines []string
	cur := ""
	for _, word := range strings.Fields(s) {
		next := strings.TrimSpace(cur + " " + word)
		if font.MeasureString(x, next).Ceil() <= width || cur == "" {
			cur = next
			continue
		}
		lines = append(lines, cur)
		cur = word
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	for i, l := range lines {
		lines[i] = shorten(x, l, width)
	}
	return lines
}

func limit(x font.Face, lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	lines = append([]string{}, lines[:n]...)
	lines[n-1] = shorten(x, lines[n-1]+"…", textWidth)
	return lines
}

// shorten cuts a line that is too long, such as one very long word.
func shorten(x font.Face, l string, width int) string {
	if font.MeasureString(x, l).Ceil() <= width {
		return l
	}
	r := []rune(strings.TrimSuffix(l, "…"))
	for len(r) > 1 && font.MeasureString(x, string(r)+"…").Ceil() > width {
		r = r[:len(r)-1]
	}
	return strings.TrimSpace(string(r)) + "…"
}

// clean keeps what the font can draw. An unknown letter loses its accent if that
// helps, and anything else, such as an emoji, is left out instead of drawn as a box.
func clean(f *opentype.Font, s string) string {
	var buf sfnt.Buffer
	has := func(r rune) bool {
		i, err := f.GlyphIndex(&buf, r)
		return err == nil && i != 0
	}
	var b strings.Builder
	for _, r := range strings.Join(strings.Fields(s), " ") {
		switch {
		case has(r):
			b.WriteRune(r)
		default:
			for _, base := range norm.NFD.String(string(r)) {
				if !unicode.Is(unicode.Mn, base) && has(base) {
					b.WriteRune(base)
				}
			}
		}
	}
	return strings.TrimSpace(b.String())
}
