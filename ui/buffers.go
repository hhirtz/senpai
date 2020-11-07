package ui

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

func IsSplitRune(r rune) bool {
	return r == ' ' || r == '\t'
}

type point struct {
	X, I  int
	Split bool
}

type Line struct {
	At        time.Time
	Head      string
	Body      string
	HeadColor int
	Highlight bool
	Mergeable bool

	splitPoints []point
	width       int
	newLines    []int
}

func (l *Line) computeSplitPoints() {
	if l.splitPoints == nil {
		l.splitPoints = []point{}
	}

	var wb widthBuffer
	lastWasSplit := false
	l.splitPoints = l.splitPoints[:0]

	for i, r := range l.Body {
		curIsSplit := IsSplitRune(r)

		if i == 0 || lastWasSplit != curIsSplit {
			l.splitPoints = append(l.splitPoints, point{
				X:     wb.Width(),
				I:     i,
				Split: curIsSplit,
			})
		}

		lastWasSplit = curIsSplit
		wb.WriteRune(r)
	}

	if !lastWasSplit {
		l.splitPoints = append(l.splitPoints, point{
			X:     wb.Width(),
			I:     len(l.Body),
			Split: true,
		})
	}
}

func (l *Line) NewLines(width int) []int {
	// Beware! This function was made by your local Test Driven Developper™ who
	// doesn't understand one bit of this function and how it works (though it
	// might not work that well if you're here...).  The code below is thus very
	// cryptic and not well structured.  However, I'm going to try to explain
	// some of those lines!

	if l.width == width {
		return l.newLines
	}
	if l.newLines == nil {
		l.newLines = []int{}
	}
	l.newLines = l.newLines[:0]
	l.width = width

	x := 0
	for i := 1; i < len(l.splitPoints); i++ {
		// Iterate through the split points 2 by 2.  Split points are placed at
		// the begining of whitespace (see IsSplitRune) and at the begining of
		// non-whitespace. Iterating on 2 points each time, sp1 and sp2, allow
		// consideration of a "word" of (non-)whitespace.
		// Split points have the index I in the string and the width X of the
		// screen.  Finally, the Split field is set to true if the split point
		// is at the begining of a whitespace.

		// Below, "row" means a line in the terminal, while "line" means (l *Line).

		sp1 := l.splitPoints[i-1]
		sp2 := l.splitPoints[i]

		if 0 < len(l.newLines) && x == 0 && sp1.Split {
			// Except for the first row, let's skip the whitespace at the start
			// of the row.
		} else if !sp1.Split && sp2.X-sp1.X == width {
			// Some word occupies the width of the terminal, lets place a
			// newline at the PREVIOUS split point (i-2, which is whitespace)
			// ONLY if there isn't already one.
			if 1 < i && 0 < len(l.newLines) && l.newLines[len(l.newLines)-1] != l.splitPoints[i-2].I {
				l.newLines = append(l.newLines, l.splitPoints[i-2].I)
			}
			// and also place a newline after the word.
			x = 0
			l.newLines = append(l.newLines, sp2.I)
		} else if sp2.X-sp1.X+x < width {
			// It fits.  Advance the X coordinate with the width of the word.
			x += sp2.X - sp1.X
		} else if sp2.X-sp1.X+x == width {
			// It fits, but there is no more space in the row.
			x = 0
			l.newLines = append(l.newLines, sp2.I)
		} else if width < sp2.X-sp1.X {
			// It doesn't fit at all.  The word is longer than the width of the
			// terminal.  In this case, no newline is placed before (like in the
			// 2nd if-else branch).  The for loop is used to place newlines in
			// the word.
			var wb widthBuffer
			h := 1
			for j, r := range l.Body[sp1.I:sp2.I] {
				wb.WriteRune(r)
				if h*width < x+wb.Width() {
					l.newLines = append(l.newLines, sp1.I+j)
					h++
				}
			}
			x = (x + wb.Width()) % width
			if x == 0 {
				// The placement of the word is such that it ends right at the
				// end of the row.
				l.newLines = append(l.newLines, sp2.I)
			}
		} else {
			// So... IIUC this branch would be the same as
			//     else if width < sp2.X-sp1.X+x
			// IE. It doesn't fit, but the word can still be placed on the next
			// row.
			l.newLines = append(l.newLines, sp1.I)
			if sp1.Split {
				x = 0
			} else {
				x = sp2.X - sp1.X
			}
		}
	}

	if 0 < len(l.newLines) && l.newLines[len(l.newLines)-1] == len(l.Body) {
		// DROP any newline that is placed at the end of the string because we
		// don't care about those.
		l.newLines = l.newLines[:len(l.newLines)-1]
	}

	return l.newLines
}

type buffer struct {
	title      string
	highlights int
	unread     bool

	lines []Line

	scrollAmt int
	isAtTop   bool
}

type BufferList struct {
	list    []buffer
	current int
	clicked int

	tlWidth      int
	tlHeight     int
	nickColWidth int
}

func NewBufferList(tlWidth, tlHeight, nickColWidth int) BufferList {
	return BufferList{
		list:         []buffer{},
		clicked:      -1,
		tlWidth:      tlWidth,
		tlHeight:     tlHeight,
		nickColWidth: nickColWidth,
	}
}

func (bs *BufferList) ResizeTimeline(tlWidth, tlHeight, nickColWidth int) {
	bs.tlWidth = tlWidth
	bs.tlHeight = tlHeight
}

func (bs *BufferList) tlInnerWidth() int {
	return bs.tlWidth - bs.nickColWidth - 9
}

func (bs *BufferList) To(i int) {
	if 0 <= i && i < len(bs.list) {
		bs.current = i
		bs.list[bs.current].highlights = 0
		bs.list[bs.current].unread = false
	}
}

func (bs *BufferList) Next() {
	bs.current = (bs.current + 1) % len(bs.list)
	bs.list[bs.current].highlights = 0
	bs.list[bs.current].unread = false
}

func (bs *BufferList) Previous() {
	bs.current = (bs.current - 1 + len(bs.list)) % len(bs.list)
	bs.list[bs.current].highlights = 0
	bs.list[bs.current].unread = false
}

func (bs *BufferList) Add(title string) (ok bool) {
	lTitle := strings.ToLower(title)
	for _, b := range bs.list {
		if strings.ToLower(b.title) == lTitle {
			return
		}
	}

	ok = true
	bs.list = append(bs.list, buffer{title: title})
	return
}

func (bs *BufferList) Remove(title string) (ok bool) {
	lTitle := strings.ToLower(title)
	for i, b := range bs.list {
		if strings.ToLower(b.title) == lTitle {
			ok = true
			bs.list = append(bs.list[:i], bs.list[i+1:]...)
			if len(bs.list) <= bs.current {
				bs.current--
			}
			return
		}
	}
	return
}

func (bs *BufferList) AddLine(title string, highlight bool, line Line) {
	idx := bs.idx(title)
	if idx < 0 {
		return
	}

	b := &bs.list[idx]
	n := len(b.lines)
	line.Body = strings.TrimRight(line.Body, "\t ")
	line.At = line.At.UTC()

	if line.Mergeable && n != 0 && b.lines[n-1].Mergeable {
		l := &b.lines[n-1]
		l.Body += "  " + line.Body
		l.computeSplitPoints()
		l.width = 0
	} else {
		line.computeSplitPoints()
		b.lines = append(b.lines, line)
		if idx == bs.current && 0 < b.scrollAmt {
			b.scrollAmt += len(line.NewLines(bs.tlInnerWidth())) + 1
		}
	}

	if !line.Mergeable && idx != bs.current {
		b.unread = true
	}
	if highlight && idx != bs.current {
		b.highlights++
	}
}

func (bs *BufferList) AddLines(title string, lines []Line) {
	idx := bs.idx(title)
	if idx < 0 {
		return
	}

	b := &bs.list[idx]
	limit := len(lines)

	if 0 < len(b.lines) {
		firstLineTime := b.lines[0].At.Unix()
		for i, l := range lines {
			if firstLineTime < l.At.Unix() {
				limit = i
				break
			}
		}
	}
	for i := 0; i < limit; i++ {
		lines[i].computeSplitPoints()
	}

	b.lines = append(lines[:limit], b.lines...)
}

func (bs *BufferList) Current() (title string) {
	return bs.list[bs.current].title
}

func (bs *BufferList) CurrentOldestTime() (t *time.Time) {
	ls := bs.list[bs.current].lines
	if 0 < len(ls) {
		t = &ls[0].At
	}
	return
}

func (bs *BufferList) ScrollUp(n int) {
	b := &bs.list[bs.current]
	if b.isAtTop {
		return
	}
	b.scrollAmt += n
}

func (bs *BufferList) ScrollDown(n int) {
	b := &bs.list[bs.current]
	b.scrollAmt -= n

	if b.scrollAmt < 0 {
		b.scrollAmt = 0
	}
}

func (bs *BufferList) IsAtTop() bool {
	b := &bs.list[bs.current]
	return b.isAtTop
}

func (bs *BufferList) idx(title string) int {
	if title == "" {
		return bs.current
	}

	lTitle := strings.ToLower(title)
	for i, b := range bs.list {
		if strings.ToLower(b.title) == lTitle {
			return i
		}
	}
	return -1
}

func (bs *BufferList) DrawVerticalBufferList(screen tcell.Screen, x0, y0, width, height int) {
	width--
	st := tcell.StyleDefault

	for y := y0; y < y0+height; y++ {
		for x := x0; x < x0+width; x++ {
			screen.SetContent(x, y, ' ', nil, st)
		}
		screen.SetContent(x0+width, y, 0x2502, nil, st.Dim(true))
	}

	for i, b := range bs.list {
		st = tcell.StyleDefault
		x := x0
		y := y0 + i
		if b.unread {
			st = st.Bold(true)
		} else if y == bs.current {
			st = st.Underline(true)
		}
		if i == bs.clicked {
			st = st.Reverse(true).Dim(true)
		}
		title := truncate(b.title, width, "\u2026")
		printString(screen, &x, y, st, title)
		if 0 < b.highlights {
			st = st.Foreground(tcell.ColorRed).Reverse(true)
			screen.SetContent(x, y, ' ', nil, st)
			x++
			printNumber(screen, &x, y, st, b.highlights)
			screen.SetContent(x, y, ' ', nil, st)
			x++
		}
		if i == bs.clicked {
			st = st.Bold(false).Underline(false)
			for ; x < x0+width; x++ {
				screen.SetContent(x, y, ' ', nil, st)
			}
			screen.SetContent(x0+width, y, 0x2590, nil, st)
		}
		y++
	}
}

func (bs *BufferList) DrawTimeline(screen tcell.Screen, x0, y0, nickColWidth int) {
	st := tcell.StyleDefault
	for x := x0; x < x0+bs.tlWidth; x++ {
		for y := y0; y < y0+bs.tlHeight; y++ {
			screen.SetContent(x, y, ' ', nil, st)
		}
	}

	b := &bs.list[bs.current]
	yi := b.scrollAmt + y0 + bs.tlHeight
	for i := len(b.lines) - 1; 0 <= i; i-- {
		if yi < 0 {
			break
		}

		x1 := x0 + 9 + nickColWidth

		line := &b.lines[i]
		nls := line.NewLines(bs.tlInnerWidth())
		yi -= len(nls) + 1
		if y0+bs.tlHeight <= yi {
			continue
		}

		if i == 0 || b.lines[i-1].At.Truncate(time.Minute) != line.At.Truncate(time.Minute) {
			printTime(screen, x0, yi, st.Bold(true), line.At.Local())
		}

		identSt := st.Foreground(colorFromCode(line.HeadColor)).Reverse(line.Highlight)
		printIdent(screen, x0+7, yi, nickColWidth, identSt, line.Head)

		x := x1
		y := yi

		var sb StyleBuffer
		sb.Reset()
		for i, r := range line.Body {
			if 0 < len(nls) && i == nls[0] {
				x = x1
				y++
				nls = nls[1:]
				if bs.tlHeight < y {
					break
				}
			}

			if y != yi && x == x1 && IsSplitRune(r) {
				continue
			}

			if st, ok := sb.WriteRune(r); ok != 0 {
				if 1 < ok {
					screen.SetContent(x, y, ',', nil, st)
					x++
				}
				screen.SetContent(x, y, r, nil, st)
				x += runeWidth(r)
			}
		}

		sb.Reset()
	}

	b.isAtTop = y0 <= yi
}

func IrcColorCode(code int) string {
	var c [3]rune
	c[0] = 0x03
	c[1] = rune(code/10) + '0'
	c[2] = rune(code%10) + '0'
	return string(c[:])
}
