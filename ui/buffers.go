package ui

import (
	"fmt"
	"strings"
	"time"
)

var homeMessages = []string{
	"\x1dYou open an IRC client.",
	"Welcome to the Internet Relay Network!",
	"Mentions & cie go here.",
	"May the IRC be with you.",
	"Hey! I'm senpai, you every IRC student!",
	"Student? No, I'm an IRC \x02client\x02!",
}

func IsSplitRune(c rune) bool {
	return c == ' ' || c == '\t'
}

type Point struct {
	X int
	I int

	Split bool
}

type Line struct {
	Time     time.Time
	IsStatus bool
	Content  string

	SplitPoints    []Point
	renderedHeight int
}

func NewLine(t time.Time, isStatus bool, content string) (line Line) {
	line.Time = t
	line.IsStatus = isStatus
	line.Content = content

	line.Invalidate()
	line.computeSplitPoints()

	return
}

func NewLineNow(content string) (line Line) {
	line = NewLine(time.Now(), false, content)
	return
}

func (line *Line) Invalidate() {
	line.renderedHeight = -1
}

func (line *Line) RenderedHeight(screenWidth int) (height int) {
	if line.renderedHeight < 0 {
		line.computeRenderedHeight(screenWidth)
	}
	height = line.renderedHeight
	return
}

func (line *Line) computeRenderedHeight(screenWidth int) {
	var lastSP Point
	line.renderedHeight = 1
	x := 0

	fmt.Printf("\n%d %q\n", screenWidth, line.Content)
	for _, sp := range line.SplitPoints {
		l := sp.X - lastSP.X

		if !sp.Split && x == 0 {
			// Don't add space at the beginning of a row
		} else if screenWidth < l {
			line.renderedHeight += (x + l) / screenWidth
			x = (x + l) % screenWidth
		} else if screenWidth < x+l {
			line.renderedHeight++
			x = l % screenWidth
		} else {
			x = (x + l) % screenWidth
		}

		fmt.Printf("%d %d %t occupied by %q\n", line.renderedHeight, x, sp.Split, line.Content[:sp.I])
		lastSP = sp
	}
}

func (line *Line) computeSplitPoints() {
	var wb widthBuffer
	lastWasSplit := false

	for i, r := range line.Content {
		curIsSplit := IsSplitRune(r)

		if lastWasSplit != curIsSplit {
			line.SplitPoints = append(line.SplitPoints, Point{
				X:     wb.Width(),
				I:     i,
				Split: curIsSplit,
			})
		}

		lastWasSplit = curIsSplit
		wb.WriteRune(r)
	}

	if !lastWasSplit {
		line.SplitPoints = append(line.SplitPoints, Point{
			X:     wb.Width(),
			I:     len(line.Content),
			Split: true,
		})
	}
}

type Buffer struct {
	Title      string
	Highlights int
	Content    []Line
}

type BufferList struct {
	List    []Buffer
	Current int
}

func (bs *BufferList) Add(title string) (pos int, ok bool) {
	for i, b := range bs.List {
		if b.Title == title {
			pos = i
			return
		}
	}

	pos = len(bs.List)
	ok = true
	bs.List = append(bs.List, Buffer{Title: title})

	return
}

func (bs *BufferList) Remove(title string) (ok bool) {
	for i, b := range bs.List {
		if b.Title == title {
			ok = true
			bs.List = append(bs.List[:i], bs.List[i+1:]...)

			if i == bs.Current {
				bs.Current = 0
			}

			return
		}
	}

	return
}

func (bs *BufferList) Previous() (ok bool) {
	if bs.Current <= 0 {
		ok = false
	} else {
		bs.Current--
		ok = true
	}

	return
}

func (bs *BufferList) Next() (ok bool) {
	if bs.Current+1 < len(bs.List) {
		bs.Current++
		ok = true
	} else {
		ok = false
	}

	return
}

func (bs *BufferList) Idx(title string) (idx int) {
	if title == "" {
		idx = 0
		return
	}

	for pos, b := range bs.List {
		if b.Title == title {
			idx = pos
			return
		}
	}

	idx = -1
	return
}

func (bs *BufferList) AddLine(idx int, line string, t time.Time, isStatus bool) {
	b := &bs.List[idx]
	n := len(bs.List[idx].Content)

	line = strings.TrimRight(line, "\t ")

	if isStatus && n != 0 && b.Content[n-1].IsStatus {
		l := &b.Content[n-1]
		l.Content += " " + line

		lineWidth := StringWidth(line)
		lastSP := l.SplitPoints[len(l.SplitPoints)-1]
		sp := Point{
			X: lastSP.X + 1 + lineWidth,
			I: len(l.SplitPoints),
		}

		l.SplitPoints = append(l.SplitPoints, sp)
		l.Invalidate()
	} else {
		if n == 0 || b.Content[n-1].Time.Truncate(time.Minute) != t.Truncate(time.Minute) {
			hour := t.Hour()
			minute := t.Minute()

			line = fmt.Sprintf("\x02%02d:%02d\x00 %s", hour, minute, line)
		}

		l := NewLine(t, isStatus, line)
		b.Content = append(b.Content, l)
	}
}

func (bs *BufferList) Invalidate() {
	for i := range bs.List {
		for j := range bs.List[i].Content {
			bs.List[i].Content[j].Invalidate()
		}
	}
}
