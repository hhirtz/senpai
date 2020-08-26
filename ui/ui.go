package ui

import (
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell"
)

type Config struct {
	NickColWidth int
	AutoComplete func(cursorIdx int, text []rune) []Completion
}

type UI struct {
	screen tcell.Screen
	Events chan tcell.Event
	exit   atomic.Value // bool
	config Config

	bs BufferList
	e  Editor
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
	ui.bs.ScrollUp()
}

func (ui *UI) ScrollDown() {
	ui.bs.ScrollDown()
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

func (ui *UI) TypingStart(buffer, nick string) {
	ui.bs.TypingStart(buffer, nick)
}

func (ui *UI) TypingStop(buffer, nick string) {
	ui.bs.TypingStop(buffer, nick)
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
	ui.e.Resize(w)
	ui.bs.Resize(w, h)
}

func (ui *UI) Draw() {
	_, h := ui.screen.Size()
	ui.e.Draw(ui.screen, h-2)
	ui.bs.Draw(ui.screen)
	ui.screen.Show()
}
