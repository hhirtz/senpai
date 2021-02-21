package senpai

import (
	"crypto/tls"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"git.sr.ht/~taiite/senpai/irc"
	"git.sr.ht/~taiite/senpai/ui"
	"github.com/gdamore/tcell/v2"
)

type App struct {
	win     *ui.UI
	s       *irc.Session
	pasting bool

	cfg        Config
	highlights []string

	lastQuery string
}

func NewApp(cfg Config) (app *App, err error) {
	app = &App{
		cfg: cfg,
	}

	if cfg.Highlights != nil {
		app.highlights = make([]string, len(cfg.Highlights))
		for i := range app.highlights {
			app.highlights[i] = strings.ToLower(cfg.Highlights[i])
		}
	}

	app.win, err = ui.New(ui.Config{
		NickColWidth: cfg.NickColWidth,
		ChanColWidth: cfg.ChanColWidth,
		AutoComplete: func(cursorIdx int, text []rune) []ui.Completion {
			return app.completions(cursorIdx, text)
		},
	})
	if err != nil {
		return
	}
	app.win.SetPrompt(">")

	app.initWindow()

	var conn net.Conn
	app.addLineNow(Home, ui.Line{
		Head: "--",
		Body: fmt.Sprintf("Connecting to %s...", cfg.Addr),
	})
	if cfg.NoTLS {
		conn, err = net.Dial("tcp", cfg.Addr)
	} else {
		conn, err = tls.Dial("tcp", cfg.Addr, nil)
	}
	if err != nil {
		app.addLineNow(Home, ui.Line{
			Head:      "!!",
			HeadColor: ui.ColorRed,
			Body:      fmt.Sprintf("Connection failed: %v", err),
		})
		err = nil
		return
	}

	var auth irc.SASLClient
	if cfg.Password != nil {
		auth = &irc.SASLPlain{Username: cfg.User, Password: *cfg.Password}
	}
	app.s, err = irc.NewSession(conn, irc.SessionParams{
		Nickname: cfg.Nick,
		Username: cfg.User,
		RealName: cfg.Real,
		Auth:     auth,
		Debug:    cfg.Debug,
	})
	if err != nil {
		app.addLineNow(Home, ui.Line{
			Head:      "!!",
			HeadColor: ui.ColorRed,
			Body:      fmt.Sprintf("Connection failed: %v", err),
		})
	}

	return
}

func (app *App) Close() {
	app.win.Close()
	if app.s != nil {
		app.s.Stop()
	}
}

func (app *App) Run() {
	for !app.win.ShouldExit() {
		if app.s != nil {
			select {
			case ev := <-app.s.Poll():
				evs := []irc.Event{ev}
			Batch:
				for i := 0; i < 64; i++ {
					select {
					case ev := <-app.s.Poll():
						evs = append(evs, ev)
					default:
						break Batch
					}
				}
				app.handleIRCEvents(evs)
			case ev := <-app.win.Events:
				app.handleUIEvent(ev)
			}
		} else {
			ev := <-app.win.Events
			app.handleUIEvent(ev)
		}
	}
}

func (app *App) handleIRCEvents(evs []irc.Event) {
	for _, ev := range evs {
		app.handleIRCEvent(ev)
	}
	if !app.pasting {
		app.draw()
	}
}

func (app *App) handleIRCEvent(ev irc.Event) {
	switch ev := ev.(type) {
	case irc.RawMessageEvent:
		head := "IN --"
		if ev.Outgoing {
			head = "OUT --"
		} else if !ev.IsValid {
			head = "IN ??"
		}
		app.win.AddLine(Home, false, ui.Line{
			At:   time.Now(),
			Head: head,
			Body: ev.Message,
		})
	case irc.ErrorEvent:
		var severity string
		switch ev.Severity {
		case irc.SeverityNote:
			severity = "Note"
		case irc.SeverityWarn:
			severity = "Warning"
		case irc.SeverityFail:
			severity = "Error"
		}
		app.win.AddLine(app.win.CurrentBuffer(), false, ui.Line{
			At:        time.Now(),
			Head:      "!!",
			HeadColor: ui.ColorRed,
			Body:      fmt.Sprintf("%s (code %s): %s", severity, ev.Code, ev.Message),
		})
	case irc.RegisteredEvent:
		body := "Connected to the server"
		if app.s.Nick() != app.cfg.Nick {
			body += " as " + app.s.Nick()
		}
		app.win.AddLine(Home, false, ui.Line{
			At:   time.Now(),
			Head: "--",
			Body: body,
		})
	case irc.SelfNickEvent:
		app.win.AddLine(app.win.CurrentBuffer(), true, ui.Line{
			At:        ev.Time,
			Head:      "--",
			Body:      fmt.Sprintf("\x0314%s\x03\u2192\x0314%s\x03", ev.FormerNick, app.s.Nick()),
			Highlight: true,
		})
	case irc.UserNickEvent:
		for _, c := range app.s.ChannelsSharedWith(ev.User.Name) {
			app.win.AddLine(c, false, ui.Line{
				At:        ev.Time,
				Head:      "--",
				Body:      fmt.Sprintf("\x0314%s\x03\u2192\x0314%s\x03", ev.FormerNick, ev.User.Name),
				Mergeable: true,
			})
		}
	case irc.SelfJoinEvent:
		app.win.AddBuffer(ev.Channel)
		app.s.RequestHistory(ev.Channel, time.Now())
	case irc.UserJoinEvent:
		app.win.AddLine(ev.Channel, false, ui.Line{
			At:        time.Now(),
			Head:      "--",
			Body:      fmt.Sprintf("\x033+\x0314%s\x03", ev.User.Name),
			Mergeable: true,
		})
	case irc.SelfPartEvent:
		app.win.RemoveBuffer(ev.Channel)
	case irc.UserPartEvent:
		app.win.AddLine(ev.Channel, false, ui.Line{
			At:        ev.Time,
			Head:      "--",
			Body:      fmt.Sprintf("\x034-\x0314%s\x03", ev.User.Name),
			Mergeable: true,
		})
	case irc.UserQuitEvent:
		for _, c := range ev.Channels {
			app.win.AddLine(c, false, ui.Line{
				At:        ev.Time,
				Head:      "--",
				Body:      fmt.Sprintf("\x034-\x0314%s\x03", ev.User.Name),
				Mergeable: true,
			})
		}
	case irc.TopicChangeEvent:
		app.win.AddLine(ev.Channel, false, ui.Line{
			At:   ev.Time,
			Head: "--",
			Body: fmt.Sprintf("\x0314Topic changed to: %s\x03", ev.Topic),
		})
	case irc.MessageEvent:
		buffer, line, hlNotification := app.formatMessage(ev)
		app.win.AddLine(buffer, hlNotification, line)
		if hlNotification {
			app.notifyHighlight(buffer, ev.User.Name, ev.Content)
		}
		if !ev.TargetIsChannel && app.s.NickCf() != app.s.Casemap(ev.User.Name) {
			app.lastQuery = ev.User.Name
		}
	case irc.HistoryEvent:
		var lines []ui.Line
		for _, m := range ev.Messages {
			switch m := m.(type) {
			case irc.MessageEvent:
				_, line, _ := app.formatMessage(m)
				lines = append(lines, line)
			default:
			}
		}
		app.win.AddLines(ev.Target, lines)
	case error:
		panic(ev)
	}
}

func (app *App) handleMouseEvent(ev *tcell.EventMouse) {
	x, y := ev.Position()
	if ev.Buttons()&tcell.WheelUp != 0 {
		if x < app.cfg.ChanColWidth {
			// TODO scroll chan list
		} else {
			app.win.ScrollUpBy(4)
			app.requestHistory()
		}
	}
	if ev.Buttons()&tcell.WheelDown != 0 {
		if x < app.cfg.ChanColWidth {
			// TODO scroll chan list
		} else {
			app.win.ScrollDownBy(4)
		}
	}
	if ev.Buttons()&tcell.ButtonPrimary != 0 && x < app.cfg.ChanColWidth {
		app.win.ClickBuffer(y)
	}
	if ev.Buttons() == 0 {
		if y == app.win.ClickedBuffer() && x < app.cfg.ChanColWidth {
			app.win.GoToBufferNo(y)
			app.updatePrompt()
		}
		app.win.ClickBuffer(-1)
	}
}

func (app *App) handleKeyEvent(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyCtrlC:
		app.win.Exit()
	case tcell.KeyCtrlL:
		app.win.Resize()
	case tcell.KeyCtrlU, tcell.KeyPgUp:
		app.win.ScrollUp()
		app.requestHistory()
	case tcell.KeyCtrlD, tcell.KeyPgDn:
		app.win.ScrollDown()
	case tcell.KeyCtrlN:
		app.win.NextBuffer()
	case tcell.KeyCtrlP:
		app.win.PreviousBuffer()
	case tcell.KeyRight:
		if ev.Modifiers() == tcell.ModAlt {
			app.win.NextBuffer()
			app.updatePrompt()
		} else {
			app.win.InputRight()
		}
	case tcell.KeyLeft:
		if ev.Modifiers() == tcell.ModAlt {
			app.win.PreviousBuffer()
			app.updatePrompt()
		} else {
			app.win.InputLeft()
		}
	case tcell.KeyUp:
		if ev.Modifiers() == tcell.ModAlt {
			app.win.PreviousBuffer()
		} else {
			app.win.InputUp()
		}
		app.updatePrompt()
	case tcell.KeyDown:
		if ev.Modifiers() == tcell.ModAlt {
			app.win.NextBuffer()
		} else {
			app.win.InputDown()
		}
		app.updatePrompt()
	case tcell.KeyHome:
		if ev.Modifiers() == tcell.ModAlt {
			app.win.GoToBufferNo(0)
		} else {
			app.win.InputHome()
		}
	case tcell.KeyEnd:
		if ev.Modifiers() == tcell.ModAlt {
			maxInt := int(^uint(0) >> 1)
			app.win.GoToBufferNo(maxInt)
		} else {
			app.win.InputEnd()
		}
	case tcell.KeyBackspace2:
		ok := app.win.InputBackspace()
		if ok {
			app.typing()
			app.updatePrompt()
		}
	case tcell.KeyDelete:
		ok := app.win.InputDelete()
		if ok {
			app.typing()
			app.updatePrompt()
		}
	case tcell.KeyTab:
		ok := app.win.InputAutoComplete()
		if ok {
			app.typing()
		}
	case tcell.KeyCR, tcell.KeyLF:
		buffer := app.win.CurrentBuffer()
		input := app.win.InputEnter()
		err := app.handleInput(buffer, input)
		if err != nil {
			app.win.AddLine(app.win.CurrentBuffer(), false, ui.Line{
				At:        time.Now(),
				Head:      "!!",
				HeadColor: ui.ColorRed,
				Body:      fmt.Sprintf("%q: %s", input, err),
			})
		}
		app.updatePrompt()
	case tcell.KeyRune:
		app.win.InputRune(ev.Rune())
		app.typing()
		app.updatePrompt()
	default:
		return
	}
}

func (app *App) handleUIEvent(ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		app.win.Resize()
	case *tcell.EventPaste:
		app.pasting = ev.Start()
	case *tcell.EventMouse:
		app.handleMouseEvent(ev)
	case *tcell.EventKey:
		app.handleKeyEvent(ev)
	default:
		return
	}
	if !app.pasting {
		app.draw()
	}
}

// requestHistory is a wrapper around irc.Session.RequestHistory to only request
// history when needed.
func (app *App) requestHistory() {
	if app.s == nil {
		return
	}
	buffer := app.win.CurrentBuffer()
	if app.win.IsAtTop() && buffer != Home {
		at := time.Now()
		if t := app.win.CurrentBufferOldestTime(); t != nil {
			at = *t
		}
		app.s.RequestHistory(buffer, at)
	}
}

// isHighlight reports whether the given message content is a highlight.
func (app *App) isHighlight(content string) bool {
	contentCf := strings.ToLower(content)
	if app.highlights == nil {
		return strings.Contains(contentCf, app.s.NickCf())
	}
	for _, h := range app.highlights {
		if strings.Contains(contentCf, h) {
			return true
		}
	}
	return false
}

// notifyHighlight executes the "on-highlight" command according to the given
// message context.
func (app *App) notifyHighlight(buffer, nick, content string) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		return
	}
	here := "0"
	if buffer == app.win.CurrentBuffer() {
		here = "1"
	}
	r := strings.NewReplacer(
		"%%", "%",
		"%b", buffer,
		"%h", here,
		"%n", nick,
		"%m", cleanMessage(content))
	command := r.Replace(app.cfg.OnHighlight)
	output, err := exec.Command(sh, "-c", command).CombinedOutput()
	if err != nil {
		body := fmt.Sprintf("Failed to invoke on-highlight command: %v. Output: %q", err, string(output))
		app.win.AddLine(Home, false, ui.Line{
			At:        time.Now(),
			Head:      "!!",
			HeadColor: ui.ColorRed,
			Body:      body,
		})
	}
}

// typing sends typing notifications to the IRC server according to the user
// input.
func (app *App) typing() {
	if app.s == nil {
		return
	}
	buffer := app.win.CurrentBuffer()
	if buffer == Home {
		return
	}
	if app.win.InputLen() == 0 {
		app.s.TypingStop(buffer)
	} else if !app.win.InputIsCommand() {
		app.s.Typing(app.win.CurrentBuffer())
	}
}

// completions computes the list of completions given the input text and the
// cursor position.
func (app *App) completions(cursorIdx int, text []rune) []ui.Completion {
	var cs []ui.Completion

	if len(text) == 0 {
		return cs
	}

	buffer := app.win.CurrentBuffer()
	if app.s.IsChannel(buffer) {
		cs = app.completionsChannelTopic(cs, cursorIdx, text)
		cs = app.completionsChannelMembers(cs, cursorIdx, text)
	}
	cs = app.completionsMsg(cs, cursorIdx, text)

	if cs != nil {
		cs = append(cs, ui.Completion{
			Text:      text,
			CursorIdx: cursorIdx,
		})
	}

	return cs
}

// formatMessage sets how a given message must be formatted.
//
// It computes three things:
// - which buffer the message must be added to,
// - the UI line,
// - whether senpai must trigger the "on-highlight" command.
func (app *App) formatMessage(ev irc.MessageEvent) (buffer string, line ui.Line, hlNotification bool) {
	isFromSelf := app.s.NickCf() == app.s.Casemap(ev.User.Name)
	isHighlight := app.isHighlight(ev.Content)
	isAction := strings.HasPrefix(ev.Content, "\x01ACTION")
	isQuery := !ev.TargetIsChannel && ev.Command == "PRIVMSG"
	isNotice := ev.Command == "NOTICE"

	if !ev.TargetIsChannel && isNotice {
		buffer = app.win.CurrentBuffer()
	} else if !ev.TargetIsChannel {
		buffer = Home
	} else {
		buffer = ev.Target
	}

	hlLine := ev.TargetIsChannel && isHighlight && !isFromSelf
	hlNotification = (isHighlight || isQuery) && !isFromSelf

	head := ev.User.Name
	headColor := ui.ColorWhite
	if isFromSelf && isQuery {
		head = "\u2192 " + ev.Target
		headColor = ui.IdentColor(ev.Target)
	} else if isAction || isNotice {
		head = "*"
	} else {
		headColor = ui.IdentColor(head)
	}

	body := strings.TrimSuffix(ev.Content, "\x01")
	if isNotice && isAction {
		c := ircColorSequence(ui.IdentColor(ev.User.Name))
		body = fmt.Sprintf("(%s%s\x0F:%s)", c, ev.User.Name, body[7:])
	} else if isAction {
		c := ircColorSequence(ui.IdentColor(ev.User.Name))
		body = fmt.Sprintf("%s%s\x0F%s", c, ev.User.Name, body[7:])
	} else if isNotice {
		c := ircColorSequence(ui.IdentColor(ev.User.Name))
		body = fmt.Sprintf("(%s%s\x0F: %s)", c, ev.User.Name, body)
	}

	line = ui.Line{
		At:        ev.Time,
		Head:      head,
		Body:      body,
		HeadColor: headColor,
		Highlight: hlLine,
	}
	return
}

// updatePrompt changes the prompt text according to the application context.
func (app *App) updatePrompt() {
	buffer := app.win.CurrentBuffer()
	command := app.win.InputIsCommand()
	if buffer == Home || command {
		app.win.SetPrompt(">")
	} else {
		app.win.SetPrompt(app.s.Nick())
	}
}

// ircColorSequence returns the color formatting sequence of a color code.
func ircColorSequence(code int) string {
	var c [3]rune
	c[0] = 0x03
	c[1] = rune(code/10) + '0'
	c[2] = rune(code%10) + '0'
	return string(c[:])
}

// cleanMessage removes IRC formatting from a string.
func cleanMessage(s string) string {
	var res strings.Builder
	var sb ui.StyleBuffer
	res.Grow(len(s))
	for _, r := range s {
		if _, ok := sb.WriteRune(r); ok != 0 {
			if 1 < ok {
				res.WriteRune(',')
			}
			res.WriteRune(r)
		}
	}
	return res.String()
}
