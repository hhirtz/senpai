package ui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
)

func printIdent(screen tcell.Screen, x, y, width int, st tcell.Style, s string) {
	s = truncate(s, width, "\u2026")
	x += width - StringWidth(s)
	screen.SetContent(x-1, y, ' ', nil, st)
	printString(screen, &x, y, st, s)
	screen.SetContent(x, y, ' ', nil, st)
}

func printString(screen tcell.Screen, x *int, y int, st tcell.Style, s string) {
	for _, r := range s {
		screen.SetContent(*x, y, r, nil, st)
		*x += runeWidth(r)
	}
}

func printNumber(screen tcell.Screen, x *int, y int, st tcell.Style, n int) {
	s := fmt.Sprintf("%d", n)
	printString(screen, x, y, st, s)
}

func printTime(screen tcell.Screen, x int, y int, st tcell.Style, t time.Time) {
	hr0 := rune(t.Hour()/10) + '0'
	hr1 := rune(t.Hour()%10) + '0'
	mn0 := rune(t.Minute()/10) + '0'
	mn1 := rune(t.Minute()%10) + '0'
	screen.SetContent(x+0, y, hr0, nil, st)
	screen.SetContent(x+1, y, hr1, nil, st)
	screen.SetContent(x+2, y, ':', nil, st)
	screen.SetContent(x+3, y, mn0, nil, st)
	screen.SetContent(x+4, y, mn1, nil, st)
}
