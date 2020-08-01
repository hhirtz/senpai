package main

import (
	"crypto/tls"
	"fmt"
	"git.sr.ht/~taiite/senpai"
	"git.sr.ht/~taiite/senpai/irc"
	"git.sr.ht/~taiite/senpai/ui"
	"github.com/gdamore/tcell"
	"hash/fnv"
	"log"
	"os"
	"strings"
	"time"
)

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
	app.AddLine("home", fmt.Sprintf("Connecting to %s...", addr), time.Now(), false)

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
			handleIRCEvent(app, ev)
		case ev := <-app.Events:
			handleUIEvent(app, &s, ev)
		}
	}
}

func handleIRCEvent(app *ui.UI, ev irc.Event) {
	switch ev := ev.(type) {
	case irc.RegisteredEvent:
		app.AddLine("home", "Connected to the server", time.Now(), false)
	case irc.SelfJoinEvent:
		app.AddBuffer(ev.Channel)
	case irc.UserJoinEvent:
		line := fmt.Sprintf("\x033+\x0314%s", ev.Nick)
		app.AddLine(ev.Channel, line, ev.Time, true)
	case irc.SelfPartEvent:
		app.RemoveBuffer(ev.Channel)
	case irc.UserPartEvent:
		line := fmt.Sprintf("\x034-\x0314%s", ev.Nick)
		app.AddLine(ev.Channel, line, ev.Time, true)
	case irc.ChannelMessageEvent:
		line := formatIRCMessage(ev.Nick, ev.Content)
		app.AddLine(ev.Channel, line, ev.Time, false)
		app.TypingStop(ev.Channel, ev.Nick)
	case irc.ChannelTypingEvent:
		if ev.State == 1 || ev.State == 2 {
			app.TypingStart(ev.Channel, ev.Nick)
		} else {
			app.TypingStop(ev.Channel, ev.Nick)
		}
	case irc.HistoryEvent:
		var lines []ui.Line
		var lastT time.Time
		isChannel := ev.Target[0] == '#'
		for _, m := range ev.Messages {
			switch m := m.(type) {
			case irc.ChannelMessageEvent:
				if isChannel {
					line := formatIRCMessage(m.Nick, m.Content)
					line = strings.TrimRight(line, "\t ")
					if lastT.Truncate(time.Minute) != m.Time.Truncate(time.Minute) {
						lastT = m.Time
						hour := lastT.Hour()
						minute := lastT.Minute()
						line = fmt.Sprintf("\x02%02d:%02d\x00 %s", hour, minute, line)
					}
					lines = append(lines, ui.NewLine(m.Time, false, line))
				} else {
					panic("TODO")
				}
			}
		}
		app.AddHistoryLines(ev.Target, lines)
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
				t := app.CurrentBufferOldestTime()
				s.RequestHistory(buffer, t)
			}
		case tcell.KeyCtrlD, tcell.KeyPgDn:
			app.ScrollDown()
		case tcell.KeyCtrlN:
			if app.NextBuffer() && app.IsAtTop() {
				buffer := app.CurrentBuffer()
				t := app.CurrentBufferOldestTime()
				s.RequestHistory(buffer, t)
			}
		case tcell.KeyCtrlP:
			if app.PreviousBuffer() && app.IsAtTop() {
				buffer := app.CurrentBuffer()
				t := app.CurrentBufferOldestTime()
				s.RequestHistory(buffer, t)
			}
		case tcell.KeyRight:
			if ev.Modifiers() == tcell.ModAlt {
				if app.NextBuffer() && app.IsAtTop() {
					buffer := app.CurrentBuffer()
					t := app.CurrentBufferOldestTime()
					s.RequestHistory(buffer, t)
				}
			} else {
				app.InputRight()
			}
		case tcell.KeyLeft:
			if ev.Modifiers() == tcell.ModAlt {
				if app.PreviousBuffer() && app.IsAtTop() {
					buffer := app.CurrentBuffer()
					t := app.CurrentBufferOldestTime()
					s.RequestHistory(buffer, t)
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
			handleInput(s, buffer, input)
		case tcell.KeyRune:
			app.InputRune(ev.Rune())
			if app.CurrentBuffer() != "home" && !strings.HasPrefix(app.Input(), "/") {
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

func handleInput(s *irc.Session, buffer, content string) {
	cmd, args := parseCommand(content)

	switch cmd {
	case "":
		if buffer == "home" {
			return
		}

		s.PrivMsg(buffer, args)
	case "J", "JOIN":
		s.Join(args)
	case "PART":
		if buffer == "home" {
			return
		}

		if args == "" {
			args = buffer
		}

		s.Part(args)
	case "ME":
		if buffer == "home" {
			return
		}

		line := fmt.Sprintf("\x01ACTION %s\x01", args)
		s.PrivMsg(buffer, line)
	case "MSG":
		split := strings.SplitN(args, " ", 2)
		if len(split) < 2 {
			return
		}

		target := split[0]
		content := split[1]
		s.PrivMsg(target, content)
	}
}

func formatIRCMessage(nick, content string) (line string) {
	c := color(nick)

	if content == "" {
		line = fmt.Sprintf("%s%s\x00:", string(c[:]), nick)
		return
	}

	if content[0] == 1 {
		content = strings.TrimSuffix(content[1:], "\x01")

		if strings.HasPrefix(content, "ACTION") {
			line = fmt.Sprintf("%s%s\x00%s", c, nick, content[6:])
		} else {
			line = fmt.Sprintf("\x1dCTCP request from\x1d %s%s\x00: %s", c, nick, content)
		}

		return
	}

	line = fmt.Sprintf("%s%s\x00: %s", string(c[:]), nick, content)

	return
}

func color(nick string) string {
	h := fnv.New32()
	_, _ = h.Write([]byte(nick))

	sum := h.Sum32() % 96

	if 1 <= sum {
		sum++
	}
	if 8 <= sum {
		sum++
	}

	var c [3]rune
	c[0] = '\x03'
	c[1] = rune(sum/10) + '0'
	c[2] = rune(sum%10) + '0'

	return string(c[:])
}
