package ui

import (
	"github.com/mattn/go-runewidth"
)

type widthBuffer struct {
	width int
	color int
	comma bool
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
	if wb.color == 1 {
		if '0' <= r && r <= '9' {
			wb.color = 2
			return
		}
		wb.color = 0
	} else if wb.color == 2 {
		if '0' <= r && r <= '9' {
			wb.color = 3
			return
		}
		if r == ',' {
			wb.color = 4
			return
		}
		wb.color = 0
	} else if wb.color == 3 {
		if r == ',' {
			wb.color = 4
			return
		}
		wb.color = 0
	} else if wb.color == 4 {
		if '0' <= r && r <= '9' {
			wb.color = 5
			return
		}

		wb.width++
		wb.color = 0
	} else if wb.color == 5 {
		wb.color = 0
		if '0' <= r && r <= '9' {
			return
		}
	}

	if r == 0x03 {
		wb.color = 1
		return
	}

	wb.width += runewidth.RuneWidth(r)
}

func StringWidth(s string) int {
	var wb widthBuffer

	wb.WriteString(s)

	return wb.Width()
}
