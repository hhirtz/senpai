package ui

import (
	"github.com/gdamore/tcell"
	"github.com/mattn/go-runewidth"
)

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
		wb.width += runewidth.RuneWidth(r)
	}
}

func StringWidth(s string) int {
	var wb widthBuffer
	wb.WriteString(s)
	return wb.Width()
}

type styleBuffer struct {
	st        tcell.Style
	color     colorBuffer
	bold      bool
	italic    bool
	underline bool
}

func (sb *styleBuffer) Reset() {
	sb.color.Reset()
	sb.st = tcell.StyleDefault
	sb.bold = false
	sb.italic = false
	sb.underline = false
}

func (sb *styleBuffer) WriteRune(r rune) (st tcell.Style, ok int) {
	if r == 0x00 || r == 0x0F {
		sb.Reset()
		return sb.st, 0
	}
	if r == 0x02 {
		sb.bold = !sb.bold
		sb.st = sb.st.Bold(sb.bold)
		return sb.st, 0
	}
	if r == 0x1D {
		sb.italic = !sb.italic
		//sb.st = st.Italic(sb.italic)
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
	}
	if 0 <= cb.bg {
		st = st.Background(colorFromCode(cb.bg))
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

func colorFromCode(code int) (color tcell.Color) {
	switch code {
	case 0:
		color = tcell.ColorWhite
	case 1:
		color = tcell.ColorBlack
	case 2:
		color = tcell.ColorBlue
	case 3:
		color = tcell.ColorGreen
	case 4:
		color = tcell.ColorRed
	case 5:
		color = tcell.ColorBrown
	case 6:
		color = tcell.ColorPurple
	case 7:
		color = tcell.ColorOrange
	case 8:
		color = tcell.ColorYellow
	case 9:
		color = tcell.ColorLightGreen
	case 10:
		color = tcell.ColorTeal
	case 11:
		color = tcell.ColorFuchsia
	case 12:
		color = tcell.ColorLightBlue
	case 13:
		color = tcell.ColorPink
	case 14:
		color = tcell.ColorGrey
	case 15:
		color = tcell.ColorLightGrey
	case 99:
		color = tcell.ColorDefault
	default:
		color = tcell.Color(code)
	}
	return
}
