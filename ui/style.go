package ui

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"mvdan.cc/xurls/v2"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

func clusterWidth(cluster []rune) int {
	width := 0
	for _, r := range cluster {
		width += runewidth.RuneWidth(r)
	}
	return width
}

func stringWidth(s string) int {
	width := 0
	for _, r := range s {
		width += runewidth.RuneWidth(r)
	}
	return width
}

func truncate(s string, w int, tail string) string {
	width := stringWidth(s)
	if width <= w {
		return s
	}
	w -= stringWidth(tail)
	end := len(s)
	for i, r := range s {
		if w < 0 {
			end = i
			break
		}
		w -= runewidth.RuneWidth(r)
	}
	return s[:end] + tail
}

func clusterSize(g *uniseg.Graphemes) int {
	start, end := g.Positions()
	return end - start
}

// Taken from <https://modern.ircdocs.horse/formatting.html>

var baseCodes = []tcell.Color{
	tcell.ColorWhite, tcell.ColorBlack, tcell.ColorBlue, tcell.ColorGreen,
	tcell.ColorRed, tcell.ColorBrown, tcell.ColorPurple, tcell.ColorOrange,
	tcell.ColorYellow, tcell.ColorLightGreen, tcell.ColorTeal, tcell.ColorLightCyan,
	tcell.ColorLightBlue, tcell.ColorPink, tcell.ColorGrey, tcell.ColorLightGrey,
}

// unused
var ansiCodes = []uint64{
	/* 16-27 */ 52, 94, 100, 58, 22, 29, 23, 24, 17, 54, 53, 89,
	/* 28-39 */ 88, 130, 142, 64, 28, 35, 30, 25, 18, 91, 90, 125,
	/* 40-51 */ 124, 166, 184, 106, 34, 49, 37, 33, 19, 129, 127, 161,
	/* 52-63 */ 196, 208, 226, 154, 46, 86, 51, 75, 21, 171, 201, 198,
	/* 64-75 */ 203, 215, 227, 191, 83, 122, 87, 111, 63, 177, 207, 205,
	/* 76-87 */ 217, 223, 229, 193, 157, 158, 159, 153, 147, 183, 219, 212,
	/* 88-98 */ 16, 233, 235, 237, 239, 241, 244, 247, 250, 254, 231,
}

var hexCodes = []int32{
	0x470000, 0x472100, 0x474700, 0x324700, 0x004700, 0x00472c, 0x004747, 0x002747, 0x000047, 0x2e0047, 0x470047, 0x47002a,
	0x740000, 0x743a00, 0x747400, 0x517400, 0x007400, 0x007449, 0x007474, 0x004074, 0x000074, 0x4b0074, 0x740074, 0x740045,
	0xb50000, 0xb56300, 0xb5b500, 0x7db500, 0x00b500, 0x00b571, 0x00b5b5, 0x0063b5, 0x0000b5, 0x7500b5, 0xb500b5, 0xb5006b,
	0xff0000, 0xff8c00, 0xffff00, 0xb2ff00, 0x00ff00, 0x00ffa0, 0x00ffff, 0x008cff, 0x0000ff, 0xa500ff, 0xff00ff, 0xff0098,
	0xff5959, 0xffb459, 0xffff71, 0xcfff60, 0x6fff6f, 0x65ffc9, 0x6dffff, 0x59b4ff, 0x5959ff, 0xc459ff, 0xff66ff, 0xff59bc,
	0xff9c9c, 0xffd39c, 0xffff9c, 0xe2ff9c, 0x9cff9c, 0x9cffdb, 0x9cffff, 0x9cd3ff, 0x9c9cff, 0xdc9cff, 0xff9cff, 0xff94d3,
	0x000000, 0x131313, 0x282828, 0x363636, 0x4d4d4d, 0x656565, 0x818181, 0x9f9f9f, 0xbcbcbc, 0xe2e2e2, 0xffffff,
}

func colorFromCode(code int) (color tcell.Color) {
	if code < 0 || 99 <= code {
		color = tcell.ColorDefault
	} else if code < 16 {
		color = baseCodes[code]
	} else {
		color = tcell.NewHexColor(hexCodes[code-16])
	}
	return
}

type rangedStyle struct {
	Start int // byte index at which Style is effective
	Style tcell.Style
}

type StyledString struct {
	string
	styles []rangedStyle // sorted, elements cannot have the same Start value
}

func PlainString(s string) StyledString {
	return StyledString{string: s}
}

func PlainSprintf(format string, a ...interface{}) StyledString {
	return PlainString(fmt.Sprintf(format, a...))
}

func Styled(s string, style tcell.Style) StyledString {
	rStyle := rangedStyle{
		Start: 0,
		Style: style,
	}
	return StyledString{
		string: s,
		styles: []rangedStyle{rStyle},
	}
}

func (s StyledString) String() string {
	return s.string
}

func (s StyledString) Truncate(w int, tail StyledString) StyledString {
	if stringWidth(s.string) < w {
		return s
	}
	var sb StyledStringBuilder
	truncated := truncate(s.string, w-stringWidth(tail.string), "")
	sb.WriteString(truncated)
	for _, style := range s.styles {
		if len(truncated) <= style.Start {
			break
		}
		sb.styles = append(sb.styles, style)
	}
	sb.WriteStyledString(tail)
	return sb.StyledString()
}

var urlRegex = xurls.Relaxed()

func (s StyledString) ParseURLs() StyledString {
	styles := make([]rangedStyle, 0, len(s.styles))

	urls := urlRegex.FindAllStringIndex(s.string, -1)
	j := 0
	lastStyle := rangedStyle{
		Start: -1,
		Style: tcell.StyleDefault,
	}
	for i := 0; i < len(urls); i++ {
		u := urls[i]
		ub, ue := u[0], u[1]
		link := s.string[u[0]:u[1]] + "?a=b"
		// find last style starting before or at url begin
		for ; j < len(s.styles); j++ {
			st := s.styles[j]
			if st.Start > ub {
				break
			}
			if st.Start == ub {
				// a style already starts at this position, edit it
				lastStyle.Style = lastStyle.Style.Hyperlink(link)
			}
			lastStyle = st
			styles = append(styles, st)
		}
		if lastStyle.Start != ub {
			// no style existed at this position, add one from the last style
			styles = append(styles, rangedStyle{
				Start: ub,
				Style: lastStyle.Style.Hyperlink(link),
			})
		}
		// find last style starting before or at url end
		for ; j < len(s.styles); j++ {
			st := s.styles[j]
			if st.Start > ue {
				break
			}
			if st.Start < ue {
				st.Style = st.Style.Hyperlink(link)
			}
			lastStyle = st
			styles = append(styles, st)
		}
		if lastStyle.Start != ue {
			// no style existed at this position, add one from the last style without the hyperlink
			styles = append(styles, rangedStyle{
				Start: ue,
				Style: lastStyle.Style.Hyperlink(""),
			})
		}
	}
	styles = append(styles, s.styles[j:]...)

	return StyledString{
		string: s.string,
		styles: styles,
	}
}

func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

func parseColorNumber(raw string) (color tcell.Color, n int) {
	if len(raw) == 0 || !isDigit(raw[0]) {
		return
	}

	// len(raw) >= 1 and its first character is a digit.

	if len(raw) == 1 || !isDigit(raw[1]) {
		code, _ := strconv.Atoi(raw[:1])
		return colorFromCode(code), 1
	}

	// len(raw) >= 2 and the two first characters are digits.

	code, _ := strconv.Atoi(raw[:2])
	return colorFromCode(code), 2
}

func parseColor(raw string) (fg, bg tcell.Color, n int) {
	fg, n = parseColorNumber(raw)
	raw = raw[n:]

	if len(raw) == 0 || raw[0] != ',' {
		return fg, tcell.ColorDefault, n
	}

	n++
	bg, p := parseColorNumber(raw[1:])
	n += p

	if bg == tcell.ColorDefault {
		// Lone comma, do not parse as part of a color code.
		return fg, tcell.ColorDefault, n - 1
	}

	return fg, bg, n
}

func IRCString(raw string) StyledString {
	var formatted strings.Builder
	var styles []rangedStyle
	var last tcell.Style

	for len(raw) != 0 {
		r, runeSize := utf8.DecodeRuneInString(raw)
		if r == utf8.RuneError {
			break
		}
		_, _, lastAttrs := last.Decompose()
		current := last
		if r == 0x0F {
			current = tcell.StyleDefault
		} else if r == 0x02 {
			lastWasBold := lastAttrs&tcell.AttrBold != 0
			current = last.Bold(!lastWasBold)
		} else if r == 0x03 {
			fg, bg, n := parseColor(raw[1:])
			raw = raw[n:]
			if n == 0 {
				// Both `fg` and `bg` are equal to
				// tcell.ColorDefault.
				current = last.Foreground(tcell.ColorDefault).
					Background(tcell.ColorDefault)
			} else if bg == tcell.ColorDefault {
				current = last.Foreground(fg)
			} else {
				current = last.Foreground(fg).Background(bg)
			}
		} else if r == 0x16 {
			lastWasReverse := lastAttrs&tcell.AttrReverse != 0
			current = last.Reverse(!lastWasReverse)
		} else if r == 0x1D {
			lastWasItalic := lastAttrs&tcell.AttrItalic != 0
			current = last.Italic(!lastWasItalic)
		} else if r == 0x1E {
			lastWasStrikeThrough := lastAttrs&tcell.AttrStrikeThrough != 0
			current = last.StrikeThrough(!lastWasStrikeThrough)
		} else if r == 0x1F {
			lastWasUnderline := lastAttrs&tcell.AttrUnderline != 0
			current = last.Underline(!lastWasUnderline)
		} else {
			formatted.WriteRune(r)
		}
		if last != current {
			if len(styles) != 0 && styles[len(styles)-1].Start == formatted.Len() {
				styles[len(styles)-1] = rangedStyle{
					Start: formatted.Len(),
					Style: current,
				}
			} else {
				styles = append(styles, rangedStyle{
					Start: formatted.Len(),
					Style: current,
				})
			}
		}
		last = current
		raw = raw[runeSize:]
	}

	return StyledString{
		string: formatted.String(),
		styles: styles,
	}
}

type StyledStringBuilder struct {
	strings.Builder
	styles []rangedStyle
}

func (sb *StyledStringBuilder) Reset() {
	sb.Builder.Reset()
	sb.styles = sb.styles[:0]
}

func (sb *StyledStringBuilder) WriteStyledString(s StyledString) {
	start := len(sb.styles)
	sb.styles = append(sb.styles, s.styles...)
	for i := start; i < len(sb.styles); i++ {
		sb.styles[i].Start += sb.Len()
	}
	sb.WriteString(s.string)
}

func (sb *StyledStringBuilder) AddStyle(start int, style tcell.Style) {
	for i := 0; i < len(sb.styles); i++ {
		if sb.styles[i].Start == i {
			sb.styles[i].Style = style
			break
		} else if sb.styles[i].Start < i {
			sb.styles = append(sb.styles[:i+1], sb.styles[i:]...)
			sb.styles[i+1] = rangedStyle{
				Start: start,
				Style: style,
			}
			break
		}
	}
	sb.styles = append(sb.styles, rangedStyle{
		Start: start,
		Style: style,
	})
}

func (sb *StyledStringBuilder) SetStyle(style tcell.Style) {
	sb.styles = append(sb.styles, rangedStyle{
		Start: sb.Len(),
		Style: style,
	})
}

func (sb *StyledStringBuilder) StyledString() StyledString {
	string := sb.String()
	styles := make([]rangedStyle, 0, len(sb.styles))
	for _, style := range sb.styles {
		if len(string) <= style.Start {
			break
		}
		styles = append(styles, style)
	}
	return StyledString{
		string: string,
		styles: styles,
	}
}
