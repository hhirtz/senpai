package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

func IsSplitRune(r rune) bool {
	return r == ' ' || r == '\t'
}

func IsSplitCluster(cluster []rune) bool {
	return len(cluster) == 1 && IsSplitRune(cluster[0])
}

type point struct {
	X, I  int
	Split bool
}

type NotifyType int

const (
	NotifyNone NotifyType = iota
	NotifyUnread
	NotifyHighlight
)

type Line struct {
	At        time.Time
	Head      string
	Body      StyledString
	HeadColor tcell.Color
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

	width := 0
	lastWasSplit := false
	l.splitPoints = l.splitPoints[:0]
	g := uniseg.NewGraphemes(l.Body.string)
	for i := 0; g.Next(); i += clusterSize(g) {
		curIsSplit := IsSplitCluster(g.Runes())

		if i == 0 || lastWasSplit != curIsSplit {
			l.splitPoints = append(l.splitPoints, point{
				X:     width,
				I:     i,
				Split: curIsSplit,
			})
		}

		lastWasSplit = curIsSplit
		width += clusterWidth(g.Runes())
	}

	if !lastWasSplit {
		l.splitPoints = append(l.splitPoints, point{
			X:     width,
			I:     len(l.Body.string),
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
		// the beginning of whitespace (see IsSplitRune) and at the beginning
		// of non-whitespace. Iterating on 2 points each time, sp1 and sp2,
		// allows consideration of a "word" of (non-)whitespace.
		// Split points have the index I in the string and the width X of the
		// screen.  Finally, the Split field is set to true if the split point
		// is at the beginning of a whitespace.

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
		} else if sp1.Split && width < sp2.X-sp1.X {
			// Some whitespace occupies a width larger than the terminal's.
			x = 0
			l.newLines = append(l.newLines, sp1.I)
		} else if width < sp2.X-sp1.X {
			// It doesn't fit at all.  The word is longer than the width of the
			// terminal.  In this case, no newline is placed before (like in the
			// 2nd if-else branch).  The for loop is used to place newlines in
			// the word.
			wordWidth := 0
			h := 1
			g := uniseg.NewGraphemes(l.Body.string[sp1.I:sp2.I])
			for j := sp1.I; g.Next(); j += clusterSize(g) {
				wordWidth += clusterWidth(g.Runes())
				if h*width < x+wordWidth {
					l.newLines = append(l.newLines, j)
					h++
				}
			}
			x = (x + wordWidth) % width
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

	if 0 < len(l.newLines) && l.newLines[len(l.newLines)-1] == len(l.Body.string) {
		// DROP any newline that is placed at the end of the string because we
		// don't care about those.
		l.newLines = l.newLines[:len(l.newLines)-1]
	}

	return l.newLines
}

type buffer struct {
	netID      string
	netName    string
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

	tlInnerWidth int
	tlHeight     int

	showBufferNumbers bool
}

// NewBufferList returns a new BufferList.
// Call Resize() once before using it.
func NewBufferList() BufferList {
	return BufferList{
		list:    []buffer{},
		clicked: -1,
	}
}

func (bs *BufferList) ResizeTimeline(tlInnerWidth, tlHeight int) {
	bs.tlInnerWidth = tlInnerWidth
	bs.tlHeight = tlHeight
}

func (bs *BufferList) To(i int) bool {
	if i == bs.current {
		return false
	}
	if 0 <= i {
		bs.current = i
		if len(bs.list) <= bs.current {
			bs.current = len(bs.list) - 1
		}
		bs.list[bs.current].highlights = 0
		bs.list[bs.current].unread = false
		return true
	}
	return false
}

func (bs *BufferList) ShowBufferNumbers(enabled bool) {
	bs.showBufferNumbers = enabled
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

func (bs *BufferList) Add(netID, netName, title string) (i int, added bool) {
	lTitle := strings.ToLower(title)
	gotNetID := false
	for i, b := range bs.list {
		lbTitle := strings.ToLower(b.title)
		if b.netID == netID {
			gotNetID = true
			if lbTitle == lTitle {
				return i, false
			}
		} else if gotNetID || (b.netID == netID && lbTitle < lTitle) {
			b := buffer{
				netID:   netID,
				netName: netName,
				title:   title,
			}
			bs.list = append(bs.list[:i+1], bs.list[i:]...)
			bs.list[i] = b
			if i <= bs.current && bs.current < len(bs.list) {
				bs.current++
			}
			return i, true
		}
	}

	if netName == "" {
		netName = netID
	}
	bs.list = append(bs.list, buffer{
		netID:   netID,
		netName: netName,
		title:   title,
	})
	return len(bs.list) - 1, true
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

func (bs *BufferList) AddLine(netID, title string, notify NotifyType, line Line) {
	idx := bs.idx(netID, title)
	if idx < 0 {
		return
	}

	b := &bs.list[idx]
	n := len(b.lines)
	line.At = line.At.UTC()

	if !line.Mergeable {
		line.Body = line.Body.ParseURLs()
	}

	if line.Mergeable && n != 0 && b.lines[n-1].Mergeable {
		l := &b.lines[n-1]
		newBody := new(StyledStringBuilder)
		newBody.Grow(len(l.Body.string) + 2 + len(line.Body.string))
		newBody.WriteStyledString(l.Body)
		newBody.WriteString("  ")
		newBody.WriteStyledString(line.Body)
		l.Body = newBody.StyledString()
		l.computeSplitPoints()
		l.width = 0
		// TODO change b.scrollAmt if it's not 0 and bs.current is idx.
	} else {
		line.computeSplitPoints()
		b.lines = append(b.lines, line)
		if idx == bs.current && 0 < b.scrollAmt {
			b.scrollAmt += len(line.NewLines(bs.tlInnerWidth)) + 1
		}
	}

	if notify != NotifyNone && idx != bs.current {
		b.unread = true
	}
	if notify == NotifyHighlight && idx != bs.current {
		b.highlights++
	}
}

func (bs *BufferList) AddLines(netID, title string, before, after []Line) {
	idx := bs.idx(netID, title)
	if idx < 0 {
		return
	}

	b := &bs.list[idx]

	for i := 0; i < len(before); i++ {
		before[i].Body = before[i].Body.ParseURLs()
		before[i].computeSplitPoints()
	}
	for i := 0; i < len(after); i++ {
		after[i].Body = after[i].Body.ParseURLs()
		after[i].computeSplitPoints()
	}

	if len(before) != 0 {
		b.lines = append(before, b.lines...)
	}
	if len(after) != 0 {
		b.lines = append(b.lines, after...)
	}
}

func (bs *BufferList) Current() (netID, title string) {
	b := &bs.list[bs.current]
	return b.netID, b.title
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

func (bs *BufferList) ScrollUpHighlight() bool {
	b := &bs.list[bs.current]
	ymin := b.scrollAmt + bs.tlHeight
	y := 0
	for i := len(b.lines) - 1; 0 <= i; i-- {
		line := &b.lines[i]
		if ymin <= y && line.Highlight {
			b.scrollAmt = y - bs.tlHeight + 1
			return true
		}
		y += len(line.NewLines(bs.tlInnerWidth)) + 1
	}
	return false
}

func (bs *BufferList) ScrollDownHighlight() bool {
	b := &bs.list[bs.current]
	yLastHighlight := 0
	y := 0
	for i := len(b.lines) - 1; 0 <= i && y < b.scrollAmt; i-- {
		line := &b.lines[i]
		if line.Highlight {
			yLastHighlight = y
		}
		y += len(line.NewLines(bs.tlInnerWidth)) + 1
	}
	b.scrollAmt = yLastHighlight
	return b.scrollAmt != 0
}

func (bs *BufferList) IsAtTop() bool {
	b := &bs.list[bs.current]
	return b.isAtTop
}

func (bs *BufferList) idx(netID, title string) int {
	lTitle := strings.ToLower(title)
	for i, b := range bs.list {
		if b.netID == netID && strings.ToLower(b.title) == lTitle {
			return i
		}
	}
	return -1
}

func (bs *BufferList) DrawVerticalBufferList(screen tcell.Screen, x0, y0, width, height int, offset *int) {
	if y0+len(bs.list)-*offset < height {
		*offset = y0 + len(bs.list) - height
		if *offset < 0 {
			*offset = 0
		}
	}

	width--
	drawVerticalLine(screen, x0+width, y0, height)
	clearArea(screen, x0, y0, width, height)

	indexPadding := 1 + int(math.Ceil(math.Log10(float64(len(bs.list)))))
	for i, b := range bs.list[*offset:] {
		bi := *offset + i
		x := x0
		y := y0 + i
		st := tcell.StyleDefault
		if b.unread {
			st = st.Bold(true)
		} else if bi == bs.current {
			st = st.Underline(true)
		}
		if bi == bs.clicked {
			st = st.Reverse(true)
		}
		if bs.showBufferNumbers {
			indexSt := st.Foreground(tcell.ColorGray)
			indexText := fmt.Sprintf("%d:", bi)
			printString(screen, &x, y, Styled(indexText, indexSt))
			x = x0 + indexPadding
		}

		var title string
		if b.title == "" {
			title = b.netName
		} else {
			if bi == bs.clicked {
				screen.SetContent(x, y, ' ', nil, tcell.StyleDefault.Reverse(true))
				screen.SetContent(x+1, y, ' ', nil, tcell.StyleDefault.Reverse(true))
			}
			x += 2
			title = b.title
		}
		title = truncate(title, width-(x-x0), "\u2026")
		printString(screen, &x, y, Styled(title, st))

		if bi == bs.clicked {
			st := tcell.StyleDefault.Reverse(true)
			for ; x < x0+width; x++ {
				screen.SetContent(x, y, ' ', nil, st)
			}
			screen.SetContent(x, y, 0x2590, nil, st)
		}

		if b.highlights != 0 {
			highlightSt := st.Foreground(tcell.ColorRed).Reverse(true)
			highlightText := fmt.Sprintf(" %d ", b.highlights)
			x = x0 + width - len(highlightText)
			printString(screen, &x, y, Styled(highlightText, highlightSt))
		}
	}
}

func (bs *BufferList) DrawHorizontalBufferList(screen tcell.Screen, x0, y0, width int) {
	x := x0

	for i, b := range bs.list {
		if width <= x-x0 {
			break
		}
		st := tcell.StyleDefault
		if b.unread {
			st = st.Bold(true)
		} else if i == bs.current {
			st = st.Underline(true)
		}
		if i == bs.clicked {
			st = st.Reverse(true)
		}

		var title string
		if b.title == "" {
			st = st.Dim(true)
			title = b.netName
		} else {
			title = b.title
		}
		title = truncate(title, width-x, "\u2026")
		printString(screen, &x, y0, Styled(title, st))

		if 0 < b.highlights {
			st = st.Foreground(tcell.ColorRed).Reverse(true)
			screen.SetContent(x, y0, ' ', nil, st)
			x++
			printNumber(screen, &x, y0, st, b.highlights)
			screen.SetContent(x, y0, ' ', nil, st)
			x++
		}
		screen.SetContent(x, y0, ' ', nil, tcell.StyleDefault)
		x++
	}
	for x < width {
		screen.SetContent(x, y0, ' ', nil, tcell.StyleDefault)
		x++
	}
}

func (bs *BufferList) DrawTimeline(screen tcell.Screen, x0, y0, nickColWidth int) {
	clearArea(screen, x0, y0, bs.tlInnerWidth+nickColWidth+9, bs.tlHeight)

	b := &bs.list[bs.current]
	yi := b.scrollAmt + y0 + bs.tlHeight
	for i := len(b.lines) - 1; 0 <= i; i-- {
		if yi < 0 {
			break
		}

		x1 := x0 + 9 + nickColWidth

		line := &b.lines[i]
		nls := line.NewLines(bs.tlInnerWidth)
		yi -= len(nls) + 1
		if y0+bs.tlHeight <= yi {
			continue
		}

		if i == 0 || b.lines[i-1].At.Truncate(time.Minute) != line.At.Truncate(time.Minute) {
			st := tcell.StyleDefault.Bold(true)
			printTime(screen, x0, yi, st, line.At.Local())
		}

		identSt := tcell.StyleDefault.
			Foreground(line.HeadColor).
			Reverse(line.Highlight)
		printIdent(screen, x0+7, yi, nickColWidth, Styled(line.Head, identSt))

		x := x1
		y := yi
		style := tcell.StyleDefault
		nextStyles := line.Body.styles

		g := uniseg.NewGraphemes(line.Body.string)
		for i := 0; g.Next(); i += clusterSize(g) {
			if 0 < len(nextStyles) && nextStyles[0].Start == i {
				style = nextStyles[0].Style
				nextStyles = nextStyles[1:]
			}
			if 0 < len(nls) && i == nls[0] {
				x = x1
				y++
				nls = nls[1:]
				if bs.tlHeight <= y {
					break
				}
			}

			cluster := g.Runes()
			if len(cluster) == 0 {
				continue
			}
			if y != yi && x == x1 && IsSplitCluster(cluster) {
				continue
			}

			screen.SetContent(x, y, cluster[0], cluster[1:], style)
			x += clusterWidth(g.Runes())
		}
	}

	b.isAtTop = y0 <= yi
}
