package senpai

import (
	"math/rand"
	"strings"
	"time"

	"git.sr.ht/~taiite/senpai/ui"
)

var Home = "home"

var homeMessages = []string{
	"\x1dYou open an IRC client.",
	"Welcome to the Internet Relay Network!",
	"DMs & cie go here.",
	"May the IRC be with you.",
	"Hey! I'm senpai, you everyday IRC student!",
	"Student? No, I'm an IRC \x02client\x02!",
}

func (app *App) initWindow() {
	hmIdx := rand.Intn(len(homeMessages))
	app.win.AddBuffer(Home)
	app.win.AddLine(Home, false, ui.Line{
		Head: "--",
		Body: homeMessages[hmIdx],
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
		app.win.AddLine(Home, false, line)
	}
	app.win.AddLine(buffer, false, line)
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
