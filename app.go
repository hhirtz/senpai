package senpai

import (
	"crypto/tls"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"git.sr.ht/~taiite/senpai/irc"
	"git.sr.ht/~taiite/senpai/ui"
	"github.com/gdamore/tcell"
)

type App struct {
	win *ui.UI
	s   *irc.Session

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
		AutoComplete: func(cursorIdx int, text []rune) []ui.Completion {
			return app.completions(cursorIdx, text)
		},
	})
	if err != nil {
		return
	}

	var conn *tls.Conn
	app.win.AddLine(ui.Home, ui.NewLineNow("--", fmt.Sprintf("Connecting to %s...", cfg.Addr)))
	conn, err = tls.Dial("tcp", cfg.Addr, nil)
	if err != nil {
		app.win.AddLine(ui.Home, ui.NewLineNow("ERROR --", "Connection failed"))
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
		app.win.AddLine(ui.Home, ui.NewLineNow("ERROR --", "Registration failed"))
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
				app.handleIRCEvent(ev)
			case ev := <-app.win.Events:
				app.handleUIEvent(ev)
			}
		} else {
			ev := <-app.win.Events
			app.handleUIEvent(ev)
		}
	}
}

func (app *App) handleIRCEvent(ev irc.Event) {
	switch ev := ev.(type) {
	case irc.RawMessageEvent:
		head := "DEBUG  IN --"
		if ev.Outgoing {
			head = "DEBUG OUT --"
		} else if !ev.IsValid {
			head = "DEBUG  IN ??"
		}
		app.win.AddLine(ui.Home, ui.NewLineNow(head, ev.Message))
	case irc.RegisteredEvent:
		line := "Connected to the server"
		if app.s.Nick() != app.cfg.Nick {
			line += " as " + app.s.Nick()
		}
		app.win.AddLine(ui.Home, ui.NewLineNow("--", line))
	case irc.SelfNickEvent:
		line := fmt.Sprintf("\x0314%s\x03\u2192\x0314%s\x03", ev.FormerNick, ev.NewNick)
		app.win.AddLine(ui.Home, ui.NewLine(ev.Time, "--", line, true, true))
	case irc.UserNickEvent:
		line := fmt.Sprintf("\x0314%s\x03\u2192\x0314%s\x03", ev.FormerNick, ev.NewNick)
		app.win.AddLine(ui.Home, ui.NewLine(ev.Time, "--", line, true, false))
	case irc.SelfJoinEvent:
		app.win.AddBuffer(ev.Channel)
	case irc.UserJoinEvent:
		line := fmt.Sprintf("\x033+\x0314%s\x03", ev.Nick)
		app.win.AddLine(ev.Channel, ui.NewLine(ev.Time, "--", line, true, false))
	case irc.SelfPartEvent:
		app.win.RemoveBuffer(ev.Channel)
	case irc.UserPartEvent:
		line := fmt.Sprintf("\x034-\x0314%s\x03", ev.Nick)
		for _, channel := range ev.Channels {
			app.win.AddLine(channel, ui.NewLine(ev.Time, "--", line, true, false))
		}
	case irc.QueryMessageEvent:
		if ev.Command == "PRIVMSG" {
			l := ui.LineFromIRCMessage(ev.Time, ev.Nick, ev.Content, false, true)
			app.win.AddLine(ui.Home, l)
			app.win.TypingStop(ui.Home, ev.Nick)
			app.lastQuery = ev.Nick
			app.notifyHighlight("", ev.Nick, ev.Content)
		} else if ev.Command == "NOTICE" {
			l := ui.LineFromIRCMessage(ev.Time, ev.Nick, ev.Content, true, false)
			app.win.AddLine("", l)
			app.win.TypingStop("", ev.Nick)
		} else {
			log.Panicf("received unknown command for query event: %q\n", ev.Command)
		}
	case irc.ChannelMessageEvent:
		isHighlight := app.isHighlight(ev.Nick, ev.Content)
		l := ui.LineFromIRCMessage(ev.Time, ev.Nick, ev.Content, ev.Command == "NOTICE", isHighlight)
		app.win.AddLine(ev.Channel, l)
		app.win.TypingStop(ev.Channel, ev.Nick)
		if isHighlight {
			app.notifyHighlight(ev.Channel, ev.Nick, ev.Content)
		}
	case irc.QueryTagEvent:
		if ev.Typing == irc.TypingActive || ev.Typing == irc.TypingPaused {
			app.win.TypingStart(ui.Home, ev.Nick)
		} else if ev.Typing == irc.TypingDone {
			app.win.TypingStop(ui.Home, ev.Nick)
		}
	case irc.ChannelTagEvent:
		if ev.Typing == irc.TypingActive || ev.Typing == irc.TypingPaused {
			app.win.TypingStart(ev.Channel, ev.Nick)
		} else if ev.Typing == irc.TypingDone {
			app.win.TypingStop(ev.Channel, ev.Nick)
		}
	case irc.HistoryEvent:
		var lines []ui.Line
		for _, m := range ev.Messages {
			switch m := m.(type) {
			case irc.ChannelMessageEvent:
				isHighlight := app.isHighlight(m.Nick, m.Content)
				l := ui.LineFromIRCMessage(m.Time, m.Nick, m.Content, m.Command == "NOTICE", isHighlight)
				lines = append(lines, l)
			default:
				panic("TODO")
			}
		}
		app.win.AddLines(ev.Target, lines)
	case error:
		log.Panicln(ev)
	}
}

func (app *App) handleUIEvent(ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		app.win.Resize()
	case *tcell.EventKey:
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
			app.requestHistory()
		case tcell.KeyCtrlP:
			app.win.PreviousBuffer()
			app.requestHistory()
		case tcell.KeyRight:
			if ev.Modifiers() == tcell.ModAlt {
				app.win.NextBuffer()
				app.requestHistory()
			} else {
				app.win.InputRight()
			}
		case tcell.KeyLeft:
			if ev.Modifiers() == tcell.ModAlt {
				app.win.PreviousBuffer()
				app.requestHistory()
			} else {
				app.win.InputLeft()
			}
		case tcell.KeyUp:
			app.win.InputUp()
		case tcell.KeyDown:
			app.win.InputDown()
		case tcell.KeyHome:
			app.win.InputHome()
		case tcell.KeyEnd:
			app.win.InputEnd()
		case tcell.KeyBackspace2:
			ok := app.win.InputBackspace()
			if ok {
				app.typing()
			}
		case tcell.KeyDelete:
			ok := app.win.InputDelete()
			if ok {
				app.typing()
			}
		case tcell.KeyTab:
			ok := app.win.InputAutoComplete()
			if ok {
				app.typing()
			}
		case tcell.KeyCR, tcell.KeyLF:
			buffer := app.win.CurrentBuffer()
			input := app.win.InputEnter()
			app.handleInput(buffer, input)
		case tcell.KeyRune:
			app.win.InputRune(ev.Rune())
			app.typing()
		}
	}
}

func (app *App) isHighlight(nick, content string) bool {
	if app.s.NickCf() == strings.ToLower(nick) {
		return false
	}
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

func (app *App) notifyHighlight(context, nick, content string) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		return
	}
	command := app.cfg.OnHighlight
	command = strings.Replace(command, "%%", "%", -1)
	command = strings.Replace(command, "%c", context, -1)
	command = strings.Replace(command, "%n", nick, -1)
	command = strings.Replace(command, "%m", content, -1)
	exec.Command(sh, "-c", command).Run()
}

func (app *App) requestHistory() {
	if app.s == nil {
		return
	}
	buffer := app.win.CurrentBuffer()
	if app.win.IsAtTop() && buffer != ui.Home {
		at := time.Now()
		if t := app.win.CurrentBufferOldestTime(); t != nil {
			at = *t
		}
		app.s.RequestHistory(buffer, at)
	}
}

func (app *App) typing() {
	if app.s == nil {
		return
	}
	buffer := app.win.CurrentBuffer()
	if app.win.InputLen() == 0 {
		app.s.TypingStop(buffer)
	} else if buffer != ui.Home && !app.win.InputIsCommand() {
		app.s.Typing(app.win.CurrentBuffer())
	}
}

func parseCommand(s string) (command, args string) {
	if s == "" {
		return
	}

	if s[0] != '/' {
		args = s
		return
	}

	i := strings.IndexByte(s, ' ')
	if i < 0 {
		i = len(s)
	}

	command = strings.ToUpper(s[1:i])
	args = strings.TrimLeft(s[i:], " ")

	return
}

func (app *App) handleInput(buffer, content string) {
	cmd, args := parseCommand(content)

	switch cmd {
	case "":
		if buffer == ui.Home || len(strings.TrimSpace(args)) == 0 {
			return
		}

		app.s.PrivMsg(buffer, args)
		if !app.s.HasCapability("echo-message") {
			app.win.AddLine(buffer, ui.NewLineNow(app.s.Nick(), args))
		}
	case "QUOTE":
		app.s.SendRaw(args)
	case "J", "JOIN":
		app.s.Join(args)
	case "PART":
		if buffer == ui.Home {
			return
		}

		if args == "" {
			args = buffer
		}

		app.s.Part(args)
	case "NAMES":
		if buffer == ui.Home {
			return
		}

		var sb strings.Builder
		sb.WriteString("\x0314Names: ")
		for _, name := range app.s.Names(buffer) {
			if name.PowerLevel != "" {
				sb.WriteString("\x033")
				sb.WriteString(name.PowerLevel)
				sb.WriteString("\x0314")
			}
			sb.WriteString(name.Nick)
			sb.WriteRune(' ')
		}
		line := sb.String()
		app.win.AddLine(buffer, ui.NewLineNow("--", line[:len(line)-1]))
	case "TOPIC":
		if buffer == ui.Home {
			return
		}

		if args == "" {
			var line string

			topic, who, at := app.s.Topic(buffer)
			if who == "" {
				line = fmt.Sprintf("\x0314Topic: %s", topic)
			} else {
				line = fmt.Sprintf("\x0314Topic (by %s, %s): %s", who, at.Format("Mon Jan 2 15:04:05"), topic)
			}
			app.win.AddLine(buffer, ui.NewLineNow("--", line))
		} else {
			app.s.SetTopic(buffer, args)
		}
	case "ME":
		if buffer == ui.Home {
			return
		}

		args := fmt.Sprintf("\x01ACTION %s\x01", args)
		app.s.PrivMsg(buffer, args)
		if !app.s.HasCapability("echo-message") {
			line := ui.LineFromIRCMessage(time.Now(), app.s.Nick(), args, false, false)
			app.win.AddLine(buffer, line)
		}
	case "MSG":
		split := strings.SplitN(args, " ", 2)
		if len(split) < 2 {
			return
		}

		target := split[0]
		content := split[1]
		app.s.PrivMsg(target, content)
		if !app.s.HasCapability("echo-message") {
			if app.s.IsChannel(target) {
				buffer = ui.Home
			} else {
				buffer = target
			}
			line := ui.LineFromIRCMessage(time.Now(), app.s.Nick(), content, false, false)
			app.win.AddLine(buffer, line)
		}
	case "R":
		if buffer != ui.Home {
			return
		}

		app.s.PrivMsg(app.lastQuery, args)
		if !app.s.HasCapability("echo-message") {
			line := ui.LineFromIRCMessage(time.Now(), app.s.Nick(), args, false, false)
			app.win.AddLine(ui.Home, line)
		}
	}
}

func (app *App) completions(cursorIdx int, text []rune) []ui.Completion {
	var cs []ui.Completion

	if len(text) == 0 {
		return cs
	}

	var start int
	for start = cursorIdx - 1; 0 <= start; start-- {
		if text[start] == ' ' {
			break
		}
	}
	start++
	word := string(text[start:cursorIdx])
	wordCf := strings.ToLower(word)
	for _, name := range app.s.Names(app.win.CurrentBuffer()) {
		if strings.HasPrefix(strings.ToLower(name.Nick), wordCf) {
			nickComp := []rune(name.Nick)
			if start == 0 {
				nickComp = append(nickComp, ':')
			}
			nickComp = append(nickComp, ' ')
			c := make([]rune, len(text)+len(nickComp)-len(word))
			copy(c[:start], text[:start])
			if cursorIdx < len(text) {
				copy(c[start+len(nickComp):], text[cursorIdx:])
			}
			copy(c[start:], nickComp)
			cs = append(cs, ui.Completion{
				Text:      c,
				CursorIdx: start + len(nickComp),
			})
		}
	}

	if cs != nil {
		cs = append(cs, ui.Completion{
			Text:      text,
			CursorIdx: cursorIdx,
		})
	}

	return cs
}
