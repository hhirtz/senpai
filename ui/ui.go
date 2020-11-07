package ui

import (
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
)

type Config struct {
	NickColWidth int
	ChanColWidth int
	AutoComplete func(cursorIdx int, text []rune) []Completion
}

type UI struct {
	screen tcell.Screen
	Events chan tcell.Event
	exit   atomic.Value // bool
	config Config

	bs     BufferList
	e      Editor
	prompt string
	status string
}

func New(config Config) (ui *UI, err error) {
	ui = &UI{
		config: config,
	}

	ui.screen, err = tcell.NewScreen()
	if err != nil {
		return
	}

	err = ui.screen.Init()
	if err != nil {
		return
	}
	ui.screen.EnableMouse()
	ui.screen.EnablePaste()

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

	ui.bs = NewBufferList(w, h, ui.config.NickColWidth)
	ui.e = NewEditor(w, ui.config.AutoComplete)
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
}

func (ui *UI) PreviousBuffer() {
	ui.bs.Previous()
}

func (ui *UI) ScrollUp() {
	ui.bs.ScrollUp(ui.bs.tlHeight / 2)
}

func (ui *UI) ScrollDown() {
	ui.bs.ScrollDown(ui.bs.tlHeight / 2)
}

func (ui *UI) ScrollUpBy(n int) {
	ui.bs.ScrollUp(n)
}

func (ui *UI) ScrollDownBy(n int) {
	ui.bs.ScrollDown(n)
}

func (ui *UI) IsAtTop() bool {
	return ui.bs.IsAtTop()
}

func (ui *UI) AddBuffer(title string) {
	_ = ui.bs.Add(title)
}

func (ui *UI) RemoveBuffer(title string) {
	_ = ui.bs.Remove(title)
}

func (ui *UI) AddLine(buffer string, highlight bool, line Line) {
	ui.bs.AddLine(buffer, highlight, line)
}

func (ui *UI) AddLines(buffer string, lines []Line) {
	ui.bs.AddLines(buffer, lines)
}

func (ui *UI) SetStatus(status string) {
	ui.status = status
}

func (ui *UI) SetPrompt(prompt string) {
	ui.prompt = prompt
}

func (ui *UI) InputIsCommand() bool {
	return ui.e.IsCommand()
}

func (ui *UI) InputLen() int {
	return ui.e.TextLen()
}

func (ui *UI) InputRune(r rune) {
	ui.e.PutRune(r)
}

func (ui *UI) InputRight() {
	ui.e.Right()
}

func (ui *UI) InputLeft() {
	ui.e.Left()
}

func (ui *UI) InputHome() {
	ui.e.Home()
}

func (ui *UI) InputEnd() {
	ui.e.End()
}

func (ui *UI) InputUp() {
	ui.e.Up()
}

func (ui *UI) InputDown() {
	ui.e.Down()
}

func (ui *UI) InputBackspace() (ok bool) {
	return ui.e.RemRune()
}

func (ui *UI) InputDelete() (ok bool) {
	return ui.e.RemRuneForward()
}

func (ui *UI) InputAutoComplete() (ok bool) {
	return ui.e.AutoComplete()
}

func (ui *UI) InputEnter() (content string) {
	return ui.e.Flush()
}

func (ui *UI) Resize() {
	w, h := ui.screen.Size()
	ui.e.Resize(w - 9 - ui.config.ChanColWidth - ui.config.NickColWidth)
	ui.bs.ResizeTimeline(w-ui.config.ChanColWidth, h-2, ui.config.NickColWidth)
}

func (ui *UI) Draw() {
	w, h := ui.screen.Size()

	ui.e.Draw(ui.screen, 9+ui.config.ChanColWidth+ui.config.NickColWidth, h-1)

	ui.bs.DrawTimeline(ui.screen, ui.config.ChanColWidth, 0, ui.config.NickColWidth)
	ui.bs.DrawVerticalBufferList(ui.screen, 0, 0, ui.config.ChanColWidth, h)
	ui.drawStatusBar(ui.config.ChanColWidth, h-2, w-ui.config.ChanColWidth)

	for x := ui.config.ChanColWidth; x < 9+ui.config.ChanColWidth+ui.config.NickColWidth; x++ {
		ui.screen.SetContent(x, h-1, ' ', nil, tcell.StyleDefault)
	}
	st := tcell.StyleDefault.Foreground(colorFromCode(IdentColor(ui.prompt)))
	printIdent(ui.screen, ui.config.ChanColWidth+7, h-1, ui.config.NickColWidth, st, ui.prompt)

	ui.screen.Show()
}

func (ui *UI) drawStatusBar(x0, y, width int) {
	st := tcell.StyleDefault.Dim(true)

	for x := x0; x < x0+width; x++ {
		ui.screen.SetContent(x, y, ' ', nil, st)
	}

	if ui.status == "" {
		return
	}

	x := x0 + 5 + ui.config.NickColWidth
	printString(ui.screen, &x, y, st, "--")
	x += 2
	printString(ui.screen, &x, y, st, ui.status)
}
