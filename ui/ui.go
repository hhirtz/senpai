package ui

import (
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell"
)

type UI struct {
	screen tcell.Screen
	Events chan tcell.Event
	exit   atomic.Value // bool

	bs bufferList
	e  editor
}

func New() (ui *UI, err error) {
	ui = &UI{}

	ui.screen, err = tcell.NewScreen()
	if err != nil {
		return
	}

	err = ui.screen.Init()
	if err != nil {
		return
	}

	w, h := ui.screen.Size()
	ui.screen.Clear()
	ui.screen.ShowCursor(0, h-2)

	ui.Events = make(chan tcell.Event, 128)
	go func() {
		for !ui.ShouldExit() {
			ui.Events <- ui.screen.PollEvent()
		}
	}()

	ui.exit.Store(false)

	hmIdx := rand.Intn(len(homeMessages))
	ui.bs = newBufferList(w, h)
	ui.bs.Add(Home)
	ui.bs.AddLine("", NewLineNow("--", homeMessages[hmIdx]))

	ui.e = newEditor(w)

	ui.Resize()

	return
}

func (ui *UI) ShouldExit() bool {
	return ui.exit.Load().(bool)
}

func (ui *UI) Exit() {
	ui.exit.Store(true)
}

func (ui *UI) Close() {
	ui.screen.Fini()
}

func (ui *UI) CurrentBuffer() string {
	return ui.bs.Current()
}

func (ui *UI) CurrentBufferOldestTime() (t *time.Time) {
	return ui.bs.CurrentOldestTime()
}

func (ui *UI) NextBuffer() {
	ui.bs.Next()
	ui.draw()
}

func (ui *UI) PreviousBuffer() {
	ui.bs.Previous()
	ui.draw()
}

func (ui *UI) ScrollUp() {
	ui.bs.ScrollUp()
	ui.draw()
}

func (ui *UI) ScrollDown() {
	ui.bs.ScrollDown()
	ui.draw()
}

func (ui *UI) IsAtTop() bool {
	return ui.bs.IsAtTop()
}

func (ui *UI) AddBuffer(title string) {
	ok := ui.bs.Add(title)
	if ok {
		ui.draw()
	}
}

func (ui *UI) RemoveBuffer(title string) {
	ok := ui.bs.Remove(title)
	if ok {
		ui.draw()
	}
}

func (ui *UI) AddLine(buffer string, line Line) {
	ui.bs.AddLine(buffer, line)
	ui.draw()
}

func (ui *UI) AddLines(buffer string, lines []Line) {
	ui.bs.AddLines(buffer, lines)
	ui.draw()
}

func (ui *UI) TypingStart(buffer, nick string) {
	ui.bs.TypingStart(buffer, nick)
	ui.draw()
}

func (ui *UI) TypingStop(buffer, nick string) {
	ui.bs.TypingStop(buffer, nick)
	ui.draw()
}

func (ui *UI) InputIsCommand() bool {
	return ui.e.IsCommand()
}

func (ui *UI) InputLen() int {
	return ui.e.TextLen()
}

func (ui *UI) InputRune(r rune) {
	ui.e.PutRune(r)
	ui.draw()
}

func (ui *UI) InputRight() {
	ui.e.Right()
	ui.draw()
}

func (ui *UI) InputLeft() {
	ui.e.Left()
	ui.draw()
}

func (ui *UI) InputBackspace() (ok bool) {
	ok = ui.e.RemRune()
	if ok {
		ui.draw()
	}
	return
}

func (ui *UI) InputEnter() (content string) {
	content = ui.e.Flush()
	ui.draw()
	return
}

func (ui *UI) Resize() {
	w, h := ui.screen.Size()
	ui.e.Resize(w)
	ui.bs.Resize(w, h)
	ui.draw()
}

func (ui *UI) draw() {
	_, h := ui.screen.Size()
	ui.e.Draw(ui.screen, h-2)
	ui.bs.Draw(ui.screen)
	ui.screen.Show()
}

/*
func (ui *UI) drawStatus() {
	st := tcell.StyleDefault
	w, h := ui.screen.Size()

	x := 0
	y := h - 1

	l := 0
	start := 0
	for _, b := range ui.bufferList.List {
		if 0 < b.Highlights {
			l += 3 // TODO
		}

		sw := runewidth.StringWidth(b.Title) + 1
		l += sw
	}

	if w < l && ui.bufferList.Current > 0 {
		start = ui.bufferList.Current - 1
	}

	for i := start; i < len(ui.bufferList.List); i++ {
		b := ui.bufferList.List[i]

		if i == ui.bufferList.Current {
			st = st.Underline(true)
		}
		if 0 < b.Highlights {
			st = st.Bold(true)
		}

		rs := []rune(b.Title)
		for _, r := range rs {
			if w <= x {
				break
			}

			ui.screen.SetContent(x, y, r, nil, st)
			x += runewidth.RuneWidth(r)
		}

		if w+1 <= x {
			break
		}

		st = st.Normal()

		ui.screen.SetContent(x, y, ' ', nil, st)
		x++
	}

	for ; x < w; x++ {
		ui.screen.SetContent(x, y, ' ', nil, st)
	}

	ui.screen.Show()
}
// */
