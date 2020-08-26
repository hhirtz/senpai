package ui

import (
	"github.com/gdamore/tcell"
	"github.com/mattn/go-runewidth"
)

var condition runewidth.Condition = runewidth.Condition{}

func runeWidth(r rune) int {
	return condition.RuneWidth(r)
}

func truncate(s string, w int, tail string) string {
	return condition.Truncate(s, w, tail)
}

type widthBuffer struct {
	width int
	color colorBuffer
}

func (wb *widthBuffer) Width() int {
	return wb.width
}

func (wb *widthBuffer) WriteString(s string) {
	for _, r := range s {
		wb.WriteRune(r)
	}
}

func (wb *widthBuffer) WriteRune(r rune) {
	if ok := wb.color.WriteRune(r); ok != 0 {
		if 1 < ok {
			wb.width++
		}
		wb.width += runeWidth(r)
	}
}

func StringWidth(s string) int {
	var wb widthBuffer
	wb.WriteString(s)
	return wb.Width()
}

type StyleBuffer struct {
	st        tcell.Style
	color     colorBuffer
	bold      bool
	reverse   bool
	italic    bool
	underline bool
}

func (sb *StyleBuffer) Reset() {
	sb.color.Reset()
	sb.st = tcell.StyleDefault
	sb.bold = false
	sb.reverse = false
	sb.italic = false
	sb.underline = false
}

func (sb *StyleBuffer) WriteRune(r rune) (st tcell.Style, ok int) {
	if r == 0x00 || r == 0x0F {
		sb.Reset()
		return sb.st, 0
	}
	if r == 0x02 {
		sb.bold = !sb.bold
		sb.st = sb.st.Bold(sb.bold)
		return sb.st, 0
	}
	if r == 0x16 {
		sb.reverse = !sb.reverse
		sb.st = st.Reverse(sb.reverse)
		return sb.st, 0
	}
	if r == 0x1D {
		sb.italic = !sb.italic
		sb.st = st.Italic(sb.italic)
		return sb.st, 0
	}
	if r == 0x1F {
		sb.underline = !sb.underline
		sb.st = st.Underline(sb.underline)
		return sb.st, 0
	}
	if ok = sb.color.WriteRune(r); ok != 0 {
		sb.st = sb.color.Style(sb.st)
	}

	return sb.st, ok
}

type colorBuffer struct {
	state  int
	fg, bg int
}

func (cb *colorBuffer) Reset() {
	cb.state = 0
	cb.fg = -1
	cb.bg = -1
}

func (cb *colorBuffer) Style(st tcell.Style) tcell.Style {
	if 0 <= cb.fg {
		st = st.Foreground(colorFromCode(cb.fg))
	} else {
		st = st.Foreground(tcell.ColorDefault)
	}
	if 0 <= cb.bg {
		st = st.Background(colorFromCode(cb.bg))
	} else {
		st = st.Background(tcell.ColorDefault)
	}
	return st
}

func (cb *colorBuffer) WriteRune(r rune) (ok int) {
	if cb.state == 1 {
		if '0' <= r && r <= '9' {
			cb.fg = int(r - '0')
			cb.state = 2
			return
		}
	} else if cb.state == 2 {
		if '0' <= r && r <= '9' {
			cb.fg = 10*cb.fg + int(r-'0')
			cb.state = 3
			return
		}
		if r == ',' {
			cb.state = 4
			return
		}
	} else if cb.state == 3 {
		if r == ',' {
			cb.state = 4
			return
		}
	} else if cb.state == 4 {
		if '0' <= r && r <= '9' {
			cb.bg = int(r - '0')
			cb.state = 5
			return
		}
		ok++
	} else if cb.state == 5 {
		cb.state = 0
		if '0' <= r && r <= '9' {
			cb.bg = 10*cb.bg + int(r-'0')
			return
		}
	}

	if r == 0x03 {
		cb.state = 1
		cb.fg = -1
		cb.bg = -1
		return
	}

	cb.state = 0
	ok++
	return
}

const (
	ColorWhite = iota
	ColorBlack
	ColorBlue
	ColorGreen
	ColorRed
)

var ansiCodes = []tcell.Color{
	// Taken from <https://modern.ircdocs.horse/formatting.html>
	tcell.ColorWhite, tcell.ColorBlack, tcell.ColorBlue, tcell.ColorGreen,
	tcell.ColorRed, tcell.ColorBrown, tcell.ColorPurple, tcell.ColorOrange,
	tcell.ColorYellow, tcell.ColorLightGreen, tcell.ColorTeal, tcell.ColorLightCyan,
	tcell.ColorLightBlue, tcell.ColorPink, tcell.ColorGrey, tcell.ColorLightGrey,
	/* 16-27 */ 52, 94, 100, 58, 22, 29, 23, 24, 17, 54, 53, 89,
	/* 28-39 */ 88, 130, 142, 64, 28, 35, 30, 25, 18, 91, 90, 125,
	/* 40-51 */ 124, 166, 184, 106, 34, 49, 37, 33, 19, 129, 127, 161,
	/* 52-63 */ 196, 208, 226, 154, 46, 86, 51, 75, 21, 171, 201, 198,
	/* 64-75 */ 203, 215, 227, 191, 83, 122, 87, 111, 63, 177, 207, 205,
	/* 76-87 */ 217, 223, 229, 193, 157, 158, 159, 153, 147, 183, 219, 212,
	/* 88-98 */ 16, 233, 235, 237, 239, 241, 244, 247, 250, 254, 231,
}

func colorFromCode(code int) (color tcell.Color) {
	if code < 0 || len(ansiCodes) <= code {
		color = tcell.ColorDefault
	} else {
		color = ansiCodes[code]
	}
	return
}
