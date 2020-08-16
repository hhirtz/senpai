package ui

import (
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"time"

	"github.com/gdamore/tcell"
)

var Home = "home"

var homeMessages = []string{
	"\x1dYou open an IRC client.",
	"Welcome to the Internet Relay Network!",
	"Mentions & cie go here.",
	"May the IRC be with you.",
	"Hey! I'm senpai, you everyday IRC student!",
	"Student? No, I'm an IRC \x02client\x02!",
}

func IsSplitRune(r rune) bool {
	return r == ' ' || r == '\t'
}

type point struct {
	X, I  int
	Split bool
}

type Line struct {
	at   time.Time
	head string
	body string

	isStatus    bool
	isHighlight bool

	splitPoints []point
	width       int
	newLines    []int
}

func NewLine(at time.Time, head string, body string, isStatus bool, isHighlight bool) Line {
	l := Line{
		at:          at,
		head:        head,
		body:        body,
		isStatus:    isStatus,
		isHighlight: isHighlight,
		splitPoints: []point{},
		newLines:    []int{},
	}
	l.computeSplitPoints()
	return l
}

func NewLineNow(head, body string) Line {
	return NewLine(time.Now(), head, body, false, false)
}

func LineFromIRCMessage(at time.Time, nick string, content string, isNotice bool, isHighlight bool) Line {
	if strings.HasPrefix(content, "\x01ACTION") {
		c := ircColorCode(identColor(nick))
		content = fmt.Sprintf("%s%s\x0F%s", c, nick, content[7:])
		nick = "*"
	} else if isNotice {
		c := ircColorCode(identColor(nick))
		content = fmt.Sprintf("(%s%s\x0F: %s)", c, nick, content)
		nick = "*"
	}
	return NewLine(at, nick, content, false, isHighlight)
}

func (l *Line) computeSplitPoints() {
	var wb widthBuffer
	lastWasSplit := false
	l.splitPoints = l.splitPoints[:0]

	for i, r := range l.body {
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
			I:     len(l.body),
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
			if 1 < i && l.newLines[len(l.newLines)-1] != l.splitPoints[i-2].I {
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
			for j, r := range l.body[sp1.I:sp2.I] {
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

	if 0 < len(l.newLines) && l.newLines[len(l.newLines)-1] == len(l.body) {
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

	lines   []Line
	typings []string

	scrollAmt int
	isAtTop   bool
}

func (b *buffer) DrawLines(screen tcell.Screen, width int, height int) {
	st := tcell.StyleDefault
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			screen.SetContent(x, y, ' ', nil, st)
		}
	}

	nickColWidth := 16

	y0 := b.scrollAmt + height
	for i := len(b.lines) - 1; 0 <= i; i-- {
		if y0 < 0 {
			break
		}

		x0 := 5 + 1 + nickColWidth + 2

		line := &b.lines[i]
		nls := line.NewLines(width - x0)
		y0 -= len(nls) + 1
		if height <= y0 {
			continue
		}

		if i == 0 || b.lines[i-1].at.Truncate(time.Minute) != line.at.Truncate(time.Minute) {
			printTime(screen, 0, y0, st.Bold(true), line.at)
		}

		head := truncate(line.head, nickColWidth, "\u2026")
		x := 6 + nickColWidth - StringWidth(head)
		c := identColor(line.head)
		if line.isHighlight {
			st = st.Reverse(true)
		}
		printString(screen, &x, y0, st.Foreground(colorFromCode(c)), head)
		st = st.Reverse(false)

		x = x0
		y := y0

		var sb styleBuffer
		sb.Reset()
		for i, r := range line.body {
			if 0 < len(nls) && i == nls[0] {
				x = x0
				y++
				nls = nls[1:]
				if height < y {
					break
				}
			}

			if y != y0 && x == x0 && IsSplitRune(r) {
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

	b.isAtTop = 0 <= y0
}

type bufferList struct {
	list    []buffer
	current int

	width  int
	height int
}

func newBufferList(width, height int) bufferList {
	return bufferList{
		list:   []buffer{},
		width:  width,
		height: height,
	}
}

func (bs *bufferList) Resize(width, height int) {
	bs.width = width
	bs.height = height
}

func (bs *bufferList) Next() {
	bs.current = (bs.current + 1) % len(bs.list)
	bs.list[bs.current].highlights = 0
	bs.list[bs.current].unread = false
}

func (bs *bufferList) Previous() {
	bs.current = (bs.current - 1 + len(bs.list)) % len(bs.list)
	bs.list[bs.current].highlights = 0
	bs.list[bs.current].unread = false
}

func (bs *bufferList) Add(title string) (ok bool) {
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

func (bs *bufferList) Remove(title string) (ok bool) {
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

func (bs *bufferList) AddLine(title string, line Line) {
	idx := bs.idx(title)
	if idx < 0 {
		return
	}

	b := &bs.list[idx]
	n := len(b.lines)
	line.body = strings.TrimRight(line.body, "\t ")

	if line.isStatus && n != 0 && b.lines[n-1].isStatus {
		l := &b.lines[n-1]
		l.body += " " + line.body
		l.computeSplitPoints()
		l.width = 0
	} else {
		b.lines = append(b.lines, line)
		if idx == bs.current && 0 < b.scrollAmt {
			b.scrollAmt++
		}
	}

	if !line.isStatus && idx != bs.current {
		b.unread = true
	}
	if line.isHighlight && idx != bs.current {
		b.highlights++
	}
}

func (bs *bufferList) AddLines(title string, lines []Line) {
	idx := bs.idx(title)
	if idx < 0 {
		return
	}

	b := &bs.list[idx]
	limit := len(lines)

	if 0 < len(b.lines) {
		firstLineTime := b.lines[0].at.Round(time.Millisecond)
		for i := len(lines) - 1; 0 <= i; i-- {
			if firstLineTime == lines[i].at.Round(time.Millisecond) {
				limit = i
				break
			}
		}
	}

	b.lines = append(lines[:limit], b.lines...)
}

func (bs *bufferList) TypingStart(title, nick string) {
	idx := bs.idx(title)
	if idx < 0 {
		return
	}
	b := &bs.list[idx]

	lNick := strings.ToLower(nick)
	for _, n := range b.typings {
		if strings.ToLower(n) == lNick {
			return
		}
	}
	b.typings = append(b.typings, nick)
}

func (bs *bufferList) TypingStop(title, nick string) {
	idx := bs.idx(title)
	if idx < 0 {
		return
	}
	b := &bs.list[idx]

	lNick := strings.ToLower(nick)
	for i, n := range b.typings {
		if strings.ToLower(n) == lNick {
			b.typings = append(b.typings[:i], b.typings[i+1:]...)
			return
		}
	}
}

func (bs *bufferList) Current() (title string) {
	return bs.list[bs.current].title
}

func (bs *bufferList) CurrentOldestTime() (t *time.Time) {
	ls := bs.list[bs.current].lines
	if 0 < len(ls) {
		t = &ls[0].at
	}
	return
}

func (bs *bufferList) ScrollUp() {
	b := &bs.list[bs.current]
	if b.isAtTop {
		return
	}
	b.scrollAmt += bs.height / 2
}

func (bs *bufferList) ScrollDown() {
	b := &bs.list[bs.current]
	b.scrollAmt -= bs.height / 2

	if b.scrollAmt < 0 {
		b.scrollAmt = 0
	}
}

func (bs *bufferList) IsAtTop() bool {
	b := &bs.list[bs.current]
	return b.isAtTop
}

func (bs *bufferList) idx(title string) int {
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

func (bs *bufferList) Draw(screen tcell.Screen) {
	bs.list[bs.current].DrawLines(screen, bs.width, bs.height-3)
	bs.drawStatusBar(screen, bs.height-3)
	bs.drawTitleList(screen, bs.height-1)
}

func (bs *bufferList) drawStatusBar(screen tcell.Screen, y int) {
	st := tcell.StyleDefault.Dim(true)
	nicks := bs.list[bs.current].typings
	verb := " is typing..."

	for x := 0; x < bs.width; x++ {
		screen.SetContent(x, y, 0x2500, nil, st)
	}

	if len(nicks) == 0 {
		return
	}

	screen.SetContent(1, y, 0x2524, nil, st)

	x := 2
	if 1 < len(nicks) {
		verb = " are typing..."
		for _, nick := range nicks[:len(nicks)-2] {
			printString(screen, &x, y, st, nick)
			printString(screen, &x, y, st, ", ")
		}
		printString(screen, &x, y, st, nicks[len(nicks)-2])
		printString(screen, &x, y, st, " and ")
	}
	if 0 < len(nicks) {
		printString(screen, &x, y, st, nicks[len(nicks)-1])
		printString(screen, &x, y, st, verb)
	}

	if 0 < x {
		screen.SetContent(x, y, 0x251c, nil, st)
		x++
	}
}

func (bs *bufferList) drawTitleList(screen tcell.Screen, y int) {
	var widths []int
	for _, b := range bs.list {
		width := StringWidth(b.title)
		if 0 < b.highlights {
			width += int(math.Log10(float64(b.highlights))) + 3
		}
		widths = append(widths, width)
	}

	st := tcell.StyleDefault

	for x := 0; x < bs.width; x++ {
		screen.SetContent(x, y, ' ', nil, st)
	}

	x := (bs.width - widths[bs.current]) / 2
	printString(screen, &x, y, st.Underline(true), bs.list[bs.current].title)
	x += 2

	i := (bs.current + 1) % len(bs.list)
	for x < bs.width && i != bs.current {
		b := &bs.list[i]
		st = tcell.StyleDefault
		if b.unread {
			st = st.Bold(true)
		}
		printString(screen, &x, y, st, b.title)
		if 0 < b.highlights {
			st = st.Foreground(tcell.ColorRed).Reverse(true)
			screen.SetContent(x, y, ' ', nil, st)
			x++
			printNumber(screen, &x, y, st, b.highlights)
			screen.SetContent(x, y, ' ', nil, st)
			x++
		}
		x += 2
		i = (i + 1) % len(bs.list)
	}

	i = (bs.current - 1 + len(bs.list)) % len(bs.list)
	x = (bs.width - widths[bs.current]) / 2
	for 0 < x && i != bs.current {
		x -= widths[i] + 2
		b := &bs.list[i]
		st = tcell.StyleDefault
		if b.unread {
			st = st.Bold(true)
		}
		printString(screen, &x, y, st, b.title)
		if 0 < b.highlights {
			st = st.Foreground(tcell.ColorRed).Reverse(true)
			screen.SetContent(x, y, ' ', nil, st)
			x++
			printNumber(screen, &x, y, st, b.highlights)
			screen.SetContent(x, y, ' ', nil, st)
			x++
		}
		x -= widths[i]
		i = (i - 1 + len(bs.list)) % len(bs.list)
	}
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

func identColor(s string) (code int) {
	h := fnv.New32()
	_, _ = h.Write([]byte(s))

	code = int(h.Sum32()) % 96
	if 1 <= code {
		code++
	}
	if 8 <= code {
		code++
	}

	return
}

func ircColorCode(code int) string {
	var c [3]rune
	c[0] = 0x03
	c[1] = rune(code/10) + '0'
	c[2] = rune(code%10) + '0'
	return string(c[:])
}
