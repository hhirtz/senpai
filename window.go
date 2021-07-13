package senpai

import (
	"hash/fnv"
	"strings"
	"time"

	"git.sr.ht/~taiite/senpai/ui"
	"github.com/gdamore/tcell/v2"
)

var Home = "home"

const welcomeMessage = "senpai dev build. See senpai(1) for a list of keybindings and commands. Private messages and status notices go here."

func (app *App) initWindow() {
	app.win.AddBuffer(Home)
	app.win.AddLine(Home, ui.NotifyNone, ui.Line{
		Head: "--",
		Body: ui.PlainString(welcomeMessage),
		At:   time.Now(),
	})
}

func (app *App) queueStatusLine(line ui.Line) {
	if line.At.IsZero() {
		line.At = time.Now()
	}
	app.events <- event{
		src:     uiEvent,
		content: line,
	}
}

func (app *App) addStatusLine(line ui.Line) {
	buffer := app.win.CurrentBuffer()
	if buffer != Home {
		app.win.AddLine(Home, ui.NotifyNone, line)
	}
	app.win.AddLine(buffer, ui.NotifyNone, line)
}

func (app *App) setStatus() {
	if app.s == nil {
		return
	}
	ts := app.s.Typings(app.win.CurrentBuffer())
	status := ""
	if 3 < len(ts) {
		status = "several people are typing..."
	} else {
		verb := " is typing..."
		if 1 < len(ts) {
			verb = " are typing..."
			status = strings.Join(ts[:len(ts)-1], ", ") + " and "
		}
		if 0 < len(ts) {
			status += ts[len(ts)-1] + verb
		}
	}
	app.win.SetStatus(status)
}

func identColor(ident string) tcell.Color {
	h := fnv.New32()
	_, _ = h.Write([]byte(ident))
	return tcell.Color((h.Sum32()%15)+1) + tcell.ColorValid
}

func identString(ident string) ui.StyledString {
	color := identColor(ident)
	style := tcell.StyleDefault.Foreground(color)
	return ui.Styled(ident, style)
}
