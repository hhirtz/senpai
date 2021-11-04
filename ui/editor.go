package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

func insertRune(s []rune, r rune, i int) []rune {
	if i == len(s) {
		return append(s, r)
	}
	s = append(s, 0)
	copy(s[i+1:], s[i:])
	s[i] = r
	return s
}

type Completion struct {
	Text      []rune
	CursorIdx int
}

// Editor is the text field where the user writes messages and commands.
type Editor struct {
	// text is a slice of lines, which are slices of runes.
	text [][]rune

	lineIdx int

	// runeQueue contains the runes that are to be inserted in text[lineIdx]
	runeQueue []rune

	// cursorIdx is the index in text[lineIdx] of the placement of the
	// cursor, or is equal to len(text[lineIdx]) if the cursor is at the
	// end.
	cursorIdx int

	// offsetWidth is the number of elements of text[lineIdx] that are skipped
	// when rendering.
	offsetWidth int

	// width is the width of the text field.
	width int

	autoComplete func(cursorIdx int, text []rune) []Completion
	autoCache    []Completion
	autoCacheIdx int

	backsearch        bool
	backsearchPattern []rune // pre-lowercased
	backsearchIdx     int
}

// NewEditor returns a new Editor.
// Call Resize() once before using it.
func NewEditor(autoComplete func(cursorIdx int, text []rune) []Completion) Editor {
	return Editor{
		text:         [][]rune{{}},
		autoComplete: autoComplete,
	}
}

func (e *Editor) Resize(width int) {
	if width < e.width {
		e.cursorIdx = 0
		e.offsetWidth = 0
		e.autoCache = nil
		e.backsearchEnd()
	}
	e.width = width
}

// Content result must not be modified.
func (e *Editor) Content() []rune {
	return e.text[e.lineIdx]
}

func (e *Editor) TextLen() int {
	return len(e.text[e.lineIdx])
}

func (e *Editor) PutRune(r rune) {
	e.autoCache = nil
	lowerRune := runeToLower(r)
	if e.backsearch && e.cursorIdx < e.TextLen() {
		lowerNext := runeToLower(e.text[e.lineIdx][e.cursorIdx])
		if lowerRune == lowerNext {
			e.right()
			e.backsearchPattern = append(e.backsearchPattern, lowerRune)
			return
		}
	}
	e.putRune(r)
	e.right()
	if e.backsearch {
		wasEmpty := len(e.backsearchPattern) == 0
		e.backsearchPattern = append(e.backsearchPattern, lowerRune)
		if wasEmpty {
			clearLine := e.lineIdx == len(e.text)-1
			e.backsearchUpdate(e.lineIdx - 1)
			if clearLine && e.lineIdx < len(e.text)-1 {
				e.text = e.text[:len(e.text)-1]
			}
		} else {
			e.backsearchUpdate(e.lineIdx)
		}
	}
}

func (e *Editor) putRune(r rune) {
	e.text[e.lineIdx] = insertRune(e.text[e.lineIdx], r, e.cursorIdx)
}

func (e *Editor) RemRune() (ok bool) {
	ok = 0 < e.cursorIdx
	if !ok {
		return
	}
	e.remRuneAt(e.cursorIdx - 1)
	e.left()
	e.autoCache = nil
	if e.backsearch {
		if e.TextLen() == 0 {
			e.backsearchEnd()
		} else {
			e.backsearchPattern = e.backsearchPattern[:len(e.backsearchPattern)-1]
			e.backsearchUpdate(e.lineIdx)
		}
	}
	return
}

func (e *Editor) RemRuneForward() (ok bool) {
	ok = e.cursorIdx < len(e.text[e.lineIdx])
	if !ok {
		return
	}
	e.remRuneAt(e.cursorIdx)
	e.autoCache = nil
	e.backsearchEnd()
	return
}

func (e *Editor) remRuneAt(idx int) {
	copy(e.text[e.lineIdx][idx:], e.text[e.lineIdx][idx+1:])
	e.text[e.lineIdx] = e.text[e.lineIdx][:len(e.text[e.lineIdx])-1]
}

func (e *Editor) RemWord() (ok bool) {
	ok = 0 < e.cursorIdx
	if !ok {
		return
	}

	line := e.text[e.lineIdx]

	// To allow doing something like this (| is the cursor):
	// Hello world|
	// Hello |
	// |
	for e.cursorIdx > 0 && line[e.cursorIdx-1] == ' ' {
		e.remRuneAt(e.cursorIdx - 1)
		e.left()
	}

	for i := e.cursorIdx - 1; i >= 0; i -= 1 {
		if line[i] == ' ' {
			break
		}
		e.remRuneAt(i)
		e.left()
	}

	e.autoCache = nil
	e.backsearchEnd()
	return
}

func (e *Editor) Flush() (content string) {
	content = string(e.text[e.lineIdx])
	if len(e.text[len(e.text)-1]) == 0 {
		e.lineIdx = len(e.text) - 1
	} else {
		e.lineIdx = len(e.text)
		e.text = append(e.text, []rune{})
	}
	e.cursorIdx = 0
	e.offsetWidth = 0
	e.autoCache = nil
	e.backsearchEnd()
	return
}

func (e *Editor) Clear() bool {
	if e.TextLen() == 0 {
		return false
	}
	e.text[e.lineIdx] = []rune{}
	e.cursorIdx = 0
	e.offsetWidth = 0
	e.autoCache = nil
	return true
}

func (e *Editor) Right() {
	e.right()
	e.autoCache = nil
	e.backsearchEnd()
}

func (e *Editor) right() {
	g := uniseg.NewGraphemes(string(e.text[e.lineIdx][e.cursorIdx:]))
	if g.Next() {
		e.cursorIdx += len(g.Runes())
	}
}

func (e *Editor) RightWord() {
	line := e.text[e.lineIdx]

	if e.cursorIdx == len(line) {
		return
	}

	for e.cursorIdx < len(line) && line[e.cursorIdx] == ' ' {
		e.Right()
	}
	for i := e.cursorIdx; i < len(line) && line[i] != ' '; i += 1 {
		e.Right()
	}
}

func (e *Editor) Left() {
	e.left()
	e.backsearchEnd()
}

func (e *Editor) left() {
	var clusterLens []int
	g := uniseg.NewGraphemes(string(e.text[e.lineIdx][:e.cursorIdx]))
	for g.Next() {
		clusterLens = append(clusterLens, len(g.Runes()))
	}
	if len(clusterLens) == 0 {
		return
	}
	e.cursorIdx -= clusterLens[len(clusterLens)-1]
}

func (e *Editor) LeftWord() {
	if e.cursorIdx == 0 {
		return
	}

	line := e.text[e.lineIdx]

	for e.cursorIdx > 0 && line[e.cursorIdx-1] == ' ' {
		e.left()
	}
	for i := e.cursorIdx - 1; i >= 0 && line[i] != ' '; i -= 1 {
		e.left()
	}

	e.autoCache = nil
	e.backsearchEnd()
}

func (e *Editor) Home() {
	if e.cursorIdx == 0 {
		return
	}
	e.cursorIdx = 0
	e.offsetWidth = 0
	e.autoCache = nil
	e.backsearchEnd()
}

func (e *Editor) End() {
	if e.cursorIdx == len(e.text[e.lineIdx]) {
		return
	}
	e.cursorIdx = len(e.text[e.lineIdx])
	e.autoCache = nil
	e.backsearchEnd()
}

func (e *Editor) Up() {
	if e.lineIdx == 0 {
		return
	}
	e.lineIdx--
	e.cursorIdx = 0
	e.offsetWidth = 0
	e.autoCache = nil
	e.backsearchEnd()
	e.End()
}

func (e *Editor) Down() {
	if e.lineIdx == len(e.text)-1 {
		if len(e.text[e.lineIdx]) == 0 {
			return
		}
		e.Flush()
		return
	}
	e.lineIdx++
	e.cursorIdx = 0
	e.offsetWidth = 0
	e.autoCache = nil
	e.backsearchEnd()
	e.End()
}

func (e *Editor) AutoComplete(offset int) (ok bool) {
	if e.autoCache == nil {
		e.autoCache = e.autoComplete(e.cursorIdx, e.text[e.lineIdx])
		if len(e.autoCache) == 0 {
			e.autoCache = nil
			return false
		}
		e.autoCacheIdx = 0
	} else {
		e.autoCacheIdx = (e.autoCacheIdx + len(e.autoCache) + offset) % len(e.autoCache)
	}

	e.text[e.lineIdx] = e.autoCache[e.autoCacheIdx].Text
	e.cursorIdx = e.autoCache[e.autoCacheIdx].CursorIdx

	e.backsearchEnd()
	return true
}

func (e *Editor) BackSearch() {
	clearLine := false
	if !e.backsearch {
		e.backsearch = true
		e.backsearchPattern = []rune(strings.ToLower(string(e.text[e.lineIdx])))
		clearLine = e.lineIdx == len(e.text)-1
	}
	e.backsearchUpdate(e.lineIdx - 1)
	if clearLine && e.lineIdx < len(e.text)-1 {
		e.text = e.text[:len(e.text)-1]
	}
}

func (e *Editor) backsearchUpdate(start int) {
	if len(e.backsearchPattern) == 0 {
		return
	}
	pattern := string(e.backsearchPattern)
	for i := start; i >= 0; i-- {
		if match := strings.Index(strings.ToLower(string(e.text[i])), pattern); match >= 0 {
			e.lineIdx = i
			e.cursorIdx = runeOffset(string(e.text[i]), match) + len(e.backsearchPattern)
			e.offsetWidth = 0
			e.autoCache = nil
			break
		}
	}
}

func (e *Editor) backsearchEnd() {
	e.backsearch = false
}

func (e *Editor) Draw(screen tcell.Screen, x0, y int) {
	type cluster struct {
		Start int // index in e.text[e.lineIdx] of the start of the cluster
		Width int
	}

	var clusters []cluster
	g := uniseg.NewGraphemes(string(e.text[e.lineIdx]))
	for start := 0; g.Next(); start += len(g.Runes()) {
		clusters = append(clusters, cluster{
			Start: start,
			Width: stringWidth(g.Str()),
		})
	}

	accWidth := 0
	for _, cluster := range clusters {
		if accWidth < e.offsetWidth && e.cursorIdx <= cluster.Start {
			e.offsetWidth = accWidth - 16
			if e.offsetWidth < 0 {
				e.offsetWidth = 0
			}
			break
		}
		if e.width-accWidth+e.offsetWidth <= 0 {
			e.offsetWidth = accWidth - e.width + 16
			break
		}
		accWidth += cluster.Width
	}

	x := x0
	for i, cluster := range clusters {
		if x0+e.width <= x {
			break
		}
		clusterEnd := len(e.text[e.lineIdx])
		if i+1 < len(clusters) {
			clusterEnd = clusters[i+1].Start
		}
		runes := e.text[e.lineIdx][cluster.Start:clusterEnd]
		if len(runes) == 0 {
			continue
		}
		s := tcell.StyleDefault
		if e.backsearch && i < e.cursorIdx && i >= e.cursorIdx-len(e.backsearchPattern) {
			s = s.Underline(true)
		}
		screen.SetContent(x, y, runes[0], runes[1:], s)
		if cluster.Start <= e.cursorIdx && e.cursorIdx < clusterEnd {
			screen.ShowCursor(x, y)
		}
		x += cluster.Width
	}
	if e.cursorIdx == len(e.text[e.lineIdx]) {
		screen.ShowCursor(x, y)
	}
	for x < x0+e.width {
		screen.SetContent(x, y, ' ', nil, tcell.StyleDefault)
		x++
	}
}

// runeOffset returns the lowercase version of a rune
// TODO: len(strings.ToLower(string(r))) == len(strings.ToUpper(string(r))) for all x?
func runeToLower(r rune) rune {
	return []rune(strings.ToLower(string(r)))[0]
}

// runeOffset returns the rune index of the rune starting at byte i in string s
func runeOffset(s string, pos int) int {
	n := 0
	for i := range s {
		if i >= pos {
			return n
		}
		n++
	}
	return n
}
