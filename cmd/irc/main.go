package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"git.sr.ht/~taiite/senpai"
	"git.sr.ht/~taiite/senpai/irc"
	"git.sr.ht/~taiite/senpai/ui"
	"github.com/gdamore/tcell"
)

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	tcell.SetEncodingFallback(tcell.EncodingFallbackASCII)

	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Panicln(err)
	}

	cfg, err := senpai.LoadConfigFile(configDir + "/senpai/senpai.yaml")
	if err != nil {
		log.Panicln(err)
	}

	app, err := ui.New()
	if err != nil {
		log.Panicln(err)
	}
	defer app.Close()

	addr := cfg.Addr
	app.AddLine(ui.Home, ui.NewLineNow("--", fmt.Sprintf("Connecting to %s...", addr)), false)

	conn, err := tls.Dial("tcp", addr, nil)
	if err != nil {
		log.Panicln(err)
	}

	s, err := irc.NewSession(conn, irc.SessionParams{
		Nickname: cfg.Nick,
		Username: cfg.Nick,
		RealName: cfg.Real,
		Auth:     &irc.SASLPlain{Username: cfg.User, Password: cfg.Password},
	})
	if err != nil {
		log.Panicln(err)
	}
	defer s.Stop()

	for !app.ShouldExit() {
		select {
		case ev := <-s.Poll():
			handleIRCEvent(app, &s, ev)
		case ev := <-app.Events:
			handleUIEvent(app, &s, ev)
		}
	}
}

func handleIRCEvent(app *ui.UI, s *irc.Session, ev irc.Event) {
	switch ev := ev.(type) {
	case irc.RegisteredEvent:
		app.AddLine("", ui.NewLineNow("--", "Connected to the server"), false)
	case irc.SelfJoinEvent:
		app.AddBuffer(ev.Channel)
	case irc.UserJoinEvent:
		line := fmt.Sprintf("\x033+\x0314%s", ev.Nick)
		app.AddLine(ev.Channel, ui.NewLine(ev.Time, "--", line, true), false)
	case irc.SelfPartEvent:
		app.RemoveBuffer(ev.Channel)
	case irc.UserPartEvent:
		line := fmt.Sprintf("\x034-\x0314%s", ev.Nick)
		app.AddLine(ev.Channel, ui.NewLine(ev.Time, "--", line, true), false)
	case irc.QueryMessageEvent:
		if ev.Command == "PRIVMSG" {
			l := ui.LineFromIRCMessage(ev.Time, ev.Nick, ev.Content, false)
			app.AddLine(ui.Home, l, true)
			app.TypingStop(ui.Home, ev.Nick)
		} else if ev.Command == "NOTICE" {
			l := ui.LineFromIRCMessage(ev.Time, ev.Nick, ev.Content, true)
			app.AddLine("", l, true)
			app.TypingStop("", ev.Nick)
		} else {
			panic("unknown command")
		}
	case irc.ChannelMessageEvent:
		l := ui.LineFromIRCMessage(ev.Time, ev.Nick, ev.Content, ev.Command == "NOTICE")
		isHighlight := strings.Contains(strings.ToLower(ev.Content), strings.ToLower(s.Nick()))
		app.AddLine(ev.Channel, l, isHighlight)
		app.TypingStop(ev.Channel, ev.Nick)
	case irc.QueryTypingEvent:
		if ev.State == 1 || ev.State == 2 {
			app.TypingStart(ui.Home, ev.Nick)
		} else {
			app.TypingStop(ui.Home, ev.Nick)
		}
	case irc.ChannelTypingEvent:
		if ev.State == 1 || ev.State == 2 {
			app.TypingStart(ev.Channel, ev.Nick)
		} else {
			app.TypingStop(ev.Channel, ev.Nick)
		}
	case irc.HistoryEvent:
		var lines []ui.Line
		for _, m := range ev.Messages {
			switch m := m.(type) {
			case irc.ChannelMessageEvent:
				l := ui.LineFromIRCMessage(m.Time, m.Nick, m.Content, m.Command == "NOTICE")
				lines = append(lines, l)
			default:
				panic("TODO")
			}
		}
		app.AddLines(ev.Target, lines)
	case error:
		log.Panicln(ev)
	}
}

func handleUIEvent(app *ui.UI, s *irc.Session, ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		app.Resize()
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyCtrlC:
			app.Exit()
		case tcell.KeyCtrlL:
			app.Resize()
		case tcell.KeyCtrlU, tcell.KeyPgUp:
			app.ScrollUp()
			if app.IsAtTop() {
				buffer := app.CurrentBuffer()
				at := time.Now()
				if t := app.CurrentBufferOldestTime(); t != nil {
					at = *t
				}
				s.RequestHistory(buffer, at)
			}
		case tcell.KeyCtrlD, tcell.KeyPgDn:
			app.ScrollDown()
		case tcell.KeyCtrlN:
			app.NextBuffer()
			if app.IsAtTop() {
				buffer := app.CurrentBuffer()
				at := time.Now()
				if t := app.CurrentBufferOldestTime(); t != nil {
					at = *t
				}
				s.RequestHistory(buffer, at)
			}
		case tcell.KeyCtrlP:
			app.PreviousBuffer()
			if app.IsAtTop() {
				buffer := app.CurrentBuffer()
				at := time.Now()
				if t := app.CurrentBufferOldestTime(); t != nil {
					at = *t
				}
				s.RequestHistory(buffer, at)
			}
		case tcell.KeyRight:
			if ev.Modifiers() == tcell.ModAlt {
				app.NextBuffer()
				if app.IsAtTop() {
					buffer := app.CurrentBuffer()
					at := time.Now()
					if t := app.CurrentBufferOldestTime(); t != nil {
						at = *t
					}
					s.RequestHistory(buffer, at)
				}
			} else {
				app.InputRight()
			}
		case tcell.KeyLeft:
			if ev.Modifiers() == tcell.ModAlt {
				app.PreviousBuffer()
				if app.IsAtTop() {
					buffer := app.CurrentBuffer()
					at := time.Now()
					if t := app.CurrentBufferOldestTime(); t != nil {
						at = *t
					}
					s.RequestHistory(buffer, at)
				}
			} else {
				app.InputLeft()
			}
		case tcell.KeyBackspace2:
			ok := app.InputBackspace()
			if ok && app.InputLen() == 0 {
				s.TypingStop(app.CurrentBuffer())
			}
		case tcell.KeyEnter:
			buffer := app.CurrentBuffer()
			input := app.InputEnter()
			handleInput(app, s, buffer, input)
		case tcell.KeyRune:
			app.InputRune(ev.Rune())
			if app.CurrentBuffer() != ui.Home && !app.InputIsCommand() {
				s.Typing(app.CurrentBuffer())
			}
		}
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

func handleInput(app *ui.UI, s *irc.Session, buffer, content string) {
	cmd, args := parseCommand(content)

	switch cmd {
	case "":
		if buffer == ui.Home || len(strings.TrimSpace(args)) == 0 {
			return
		}

		s.PrivMsg(buffer, args)
		if !s.HasCapability("echo-message") {
			app.AddLine(buffer, ui.NewLineNow(s.Nick(), args), false)
		}
	case "QUOTE":
		s.SendRaw(args)
	case "J", "JOIN":
		s.Join(args)
	case "PART":
		if buffer == ui.Home {
			return
		}

		if args == "" {
			args = buffer
		}

		s.Part(args)
	case "ME":
		if buffer == ui.Home {
			return
		}

		line := fmt.Sprintf("\x01ACTION %s\x01", args)
		s.PrivMsg(buffer, line)
		// TODO echo message
	case "MSG":
		split := strings.SplitN(args, " ", 2)
		if len(split) < 2 {
			return
		}

		target := split[0]
		content := split[1]
		s.PrivMsg(target, content)
		// TODO echo mssage
	}
}
