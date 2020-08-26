package senpai

import (
	"math/rand"
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
	app.addLineNow("", ui.Line{
		Head: "--",
		Body: homeMessages[hmIdx],
	})
}

func (app *App) addLineNow(buffer string, line ui.Line) {
	if line.At.IsZero() {
		line.At = time.Now()
	}
	app.win.AddLine(buffer, false, line)
	app.draw()
}

func (app *App) draw() {
	app.win.Draw()
}
