package senpai

import (
	"time"

	"git.sr.ht/~taiite/senpai/ui"
)

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
