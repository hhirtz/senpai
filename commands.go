package senpai

import (
	"fmt"
	"strings"
	"time"

	"git.sr.ht/~taiite/senpai/irc"
	"git.sr.ht/~taiite/senpai/ui"
	"github.com/gdamore/tcell/v2"
)

type command struct {
	AllowHome bool
	MinArgs   int
	MaxArgs   int
	Usage     string
	Desc      string
	Handle    func(app *App, buffer string, args []string) error
}

type commandSet map[string]*command

var commands commandSet

func init() {
	commands = commandSet{
		"HELP": {
			AllowHome: true,
			MaxArgs:   1,
			Usage:     "[command]",
			Desc:      "show the list of commands, or how to use the given one",
			Handle:    commandDoHelp,
		},
		"JOIN": {
			AllowHome: true,
			MinArgs:   1,
			MaxArgs:   2,
			Usage:     "<channels> [keys]",
			Desc:      "join a channel",
			Handle:    commandDoJoin,
		},
		"ME": {
			AllowHome: true,
			MinArgs:   1,
			MaxArgs:   1,
			Usage:     "<message>",
			Desc:      "send an action (reply to last query if sent from home)",
			Handle:    commandDoMe,
		},
		"MSG": {
			AllowHome: true,
			MinArgs:   2,
			MaxArgs:   2,
			Usage:     "<target> <message>",
			Desc:      "send a message to the given target",
			Handle:    commandDoMsg,
		},
		"NAMES": {
			Desc:   "show the member list of the current channel",
			Handle: commandDoNames,
		},
		"NICK": {
			AllowHome: true,
			MinArgs:   1,
			MaxArgs:   1,
			Usage:     "<nickname>",
			Desc:      "change your nickname",
			Handle:    commandDoNick,
		},
		"MODE": {
			AllowHome: true,
			MinArgs:   2,
			MaxArgs:   5, // <channel> <flags> <limit> <user> <ban mask>
			Usage:     "<nick/channel> <flags> [args]",
			Desc:      "change channel or user modes",
			Handle:    commandDoMode,
		},
		"PART": {
			AllowHome: true,
			MaxArgs:   2,
			Usage:     "[channel] [reason]",
			Desc:      "part a channel",
			Handle:    commandDoPart,
		},
		"QUIT": {
			AllowHome: true,
			MaxArgs:   1,
			Usage:     "[reason]",
			Desc:      "quit senpai",
			Handle:    commandDoQuit,
		},
		"QUOTE": {
			AllowHome: true,
			MinArgs:   1,
			MaxArgs:   1,
			Usage:     "<raw message>",
			Desc:      "send raw protocol data",
			Handle:    commandDoQuote,
		},
		"REPLY": {
			AllowHome: true,
			MinArgs:   1,
			MaxArgs:   1,
			Usage:     "<message>",
			Desc:      "reply to the last query",
			Handle:    commandDoR,
		},
		"TOPIC": {
			MaxArgs: 1,
			Usage:   "[topic]",
			Desc:    "show or set the topic of the current channel",
			Handle:  commandDoTopic,
		},
		"BUFFER": {
			AllowHome: true,
			MinArgs:   1,
			MaxArgs:   1,
			Usage:     "<name>",
			Desc:      "switch to the buffer containing a substring",
			Handle:    commandDoBuffer,
		},
	}
}

func noCommand(app *App, buffer, content string) error {
	// You can't send messages to home buffer, and it might get
	// delivered to a user "home" without a bouncer, which will be bad.
	if buffer == Home {
		return fmt.Errorf("Can't send message to home")
	}

	app.s.PrivMsg(buffer, content)
	if !app.s.HasCapability("echo-message") {
		buffer, line, _ := app.formatMessage(irc.MessageEvent{
			User:            app.s.Nick(),
			Target:          buffer,
			TargetIsChannel: true,
			Command:         "PRIVMSG",
			Content:         content,
			Time:            time.Now(),
		})
		app.win.AddLine(buffer, false, line)
	}

	return nil
}

func commandDoHelp(app *App, buffer string, args []string) (err error) {
	t := time.Now()
	if len(args) == 0 {
		app.win.AddLine(app.win.CurrentBuffer(), false, ui.Line{
			At:   t,
			Head: "--",
			Body: ui.PlainString("Available commands:"),
		})
		for cmdName, cmd := range commands {
			if cmd.Desc == "" {
				continue
			}
			app.win.AddLine(app.win.CurrentBuffer(), false, ui.Line{
				At:   t,
				Body: ui.PlainSprintf("  \x02%s\x02 %s", cmdName, cmd.Usage),
			})
			app.win.AddLine(app.win.CurrentBuffer(), false, ui.Line{
				At:   t,
				Body: ui.PlainSprintf("    %s", cmd.Desc),
			})
			app.win.AddLine(app.win.CurrentBuffer(), false, ui.Line{
				At: t,
			})
		}
	} else {
		search := strings.ToUpper(args[0])
		found := false
		app.win.AddLine(app.win.CurrentBuffer(), false, ui.Line{
			At:   t,
			Head: "--",
			Body: ui.PlainSprintf("Commands that match \"%s\":", search),
		})
		for cmdName, cmd := range commands {
			if !strings.Contains(cmdName, search) {
				continue
			}
			usage := new(ui.StyledStringBuilder)
			usage.Grow(len(cmdName) + 1 + len(cmd.Usage))
			usage.SetStyle(tcell.StyleDefault.Bold(true))
			usage.WriteString(cmdName)
			usage.SetStyle(tcell.StyleDefault)
			usage.WriteByte(' ')
			usage.WriteString(cmd.Usage)
			app.win.AddLine(app.win.CurrentBuffer(), false, ui.Line{
				At:   t,
				Body: usage.StyledString(),
			})
			app.win.AddLine(app.win.CurrentBuffer(), false, ui.Line{
				At:   t,
				Body: ui.PlainSprintf("  %s", cmd.Desc),
			})
			app.win.AddLine(app.win.CurrentBuffer(), false, ui.Line{
				At: t,
			})
			found = true
		}
		if !found {
			app.win.AddLine(app.win.CurrentBuffer(), false, ui.Line{
				At:   t,
				Body: ui.PlainSprintf("  no command matches %q", args[0]),
			})
		}
	}
	return
}

func commandDoJoin(app *App, buffer string, args []string) (err error) {
	key := ""
	if len(args) == 2 {
		key = args[1]
	}
	app.s.Join(args[0], key)
	return
}

func commandDoMe(app *App, buffer string, args []string) (err error) {
	if buffer == Home {
		buffer = app.lastQuery
	}
	content := fmt.Sprintf("\x01ACTION %s\x01", args[0])
	app.s.PrivMsg(buffer, content)
	if !app.s.HasCapability("echo-message") {
		buffer, line, _ := app.formatMessage(irc.MessageEvent{
			User:            app.s.Nick(),
			Target:          buffer,
			TargetIsChannel: true,
			Command:         "PRIVMSG",
			Content:         content,
			Time:            time.Now(),
		})
		app.win.AddLine(buffer, false, line)
	}
	return
}

func commandDoMsg(app *App, buffer string, args []string) (err error) {
	target := args[0]
	content := args[1]
	app.s.PrivMsg(target, content)
	if !app.s.HasCapability("echo-message") {
		buffer, line, _ := app.formatMessage(irc.MessageEvent{
			User:            app.s.Nick(),
			Target:          target,
			TargetIsChannel: true,
			Command:         "PRIVMSG",
			Content:         content,
			Time:            time.Now(),
		})
		app.win.AddLine(buffer, false, line)
	}
	return
}

func commandDoNames(app *App, buffer string, args []string) (err error) {
	sb := new(ui.StyledStringBuilder)
	sb.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGrey))
	sb.WriteString("Names: ")
	for _, name := range app.s.Names(buffer) {
		if name.PowerLevel != "" {
			sb.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
			sb.WriteString(name.PowerLevel)
			sb.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGrey))
		}
		sb.WriteString(name.Name.Name)
		sb.WriteByte(' ')
	}
	body := sb.StyledString()
	// TODO remove last space
	app.win.AddLine(buffer, false, ui.Line{
		At:        time.Now(),
		Head:      "--",
		HeadColor: tcell.ColorGray,
		Body:      body,
	})
	return
}

func commandDoNick(app *App, buffer string, args []string) (err error) {
	nick := args[0]
	if i := strings.IndexAny(nick, " :@!*?"); i >= 0 {
		return fmt.Errorf("illegal char %q in nickname", nick[i])
	}
	app.s.ChangeNick(nick)
	return
}

func commandDoMode(app *App, buffer string, args []string) (err error) {
	channel := args[0]
	flags := args[1]
	mode_args := args[2:]

	app.s.ChangeMode(channel, flags, mode_args)
	return
}

func commandDoPart(app *App, buffer string, args []string) (err error) {
	channel := buffer
	reason := ""
	if 0 < len(args) {
		if app.s.IsChannel(args[0]) {
			channel = args[0]
			if 1 < len(args) {
				reason = args[1]
			}
		} else {
			reason = args[0]
		}
	}

	if channel != Home {
		app.s.Part(channel, reason)
	} else {
		err = fmt.Errorf("cannot part home!")
	}
	return
}

func commandDoQuit(app *App, buffer string, args []string) (err error) {
	reason := ""
	if 0 < len(args) {
		reason = args[0]
	}
	if app.s != nil {
		app.s.Quit(reason)
	}
	app.win.Exit()
	return
}

func commandDoQuote(app *App, buffer string, args []string) (err error) {
	app.s.SendRaw(args[0])
	return
}

func commandDoR(app *App, buffer string, args []string) (err error) {
	app.s.PrivMsg(app.lastQuery, args[0])
	if !app.s.HasCapability("echo-message") {
		buffer, line, _ := app.formatMessage(irc.MessageEvent{
			User:            app.s.Nick(),
			Target:          app.lastQuery,
			TargetIsChannel: true,
			Command:         "PRIVMSG",
			Content:         args[0],
			Time:            time.Now(),
		})
		app.win.AddLine(buffer, false, line)
	}
	return
}

func commandDoTopic(app *App, buffer string, args []string) (err error) {
	if len(args) == 0 {
		var body string

		topic, who, at := app.s.Topic(buffer)
		if who == nil {
			body = fmt.Sprintf("Topic: %s", topic)
		} else {
			body = fmt.Sprintf("Topic (by %s, %s): %s", who, at.Local().Format("Mon Jan 2 15:04:05"), topic)
		}
		app.win.AddLine(buffer, false, ui.Line{
			At:        time.Now(),
			Head:      "--",
			HeadColor: tcell.ColorGray,
			Body:      ui.Styled(body, tcell.StyleDefault.Foreground(tcell.ColorGray)),
		})
	} else {
		app.s.ChangeTopic(buffer, args[0])
	}
	return
}

// implemented from https://golang.org/src/strings/strings.go?s=8055:8085#L310
func fieldsN(s string, n int) []string {
	s = strings.TrimSpace(s)
	if s == "" || n == 0 {
		return nil
	}
	if n == 1 {
		return []string{s}
	}
	n--
	// Start of the ASCII fast path.
	var a []string
	na := 0
	fieldStart := 0
	i := 0
	// Skip spaces in front of the input.
	for i < len(s) && s[i] == ' ' {
		i++
	}
	fieldStart = i
	for i < len(s) {
		if s[i] != ' ' {
			i++
			continue
		}
		a = append(a, s[fieldStart:i])
		na++
		i++
		// Skip spaces in between fields.
		for i < len(s) && s[i] == ' ' {
			i++
		}
		fieldStart = i
		if n <= na {
			a = append(a, s[fieldStart:])
			return a
		}
	}
	if fieldStart < len(s) {
		// Last field ends at EOF.
		a = append(a, s[fieldStart:])
	}
	return a
}

func parseCommand(s string) (command, args string, isCommand bool) {
	if s[0] != '/' {
		return "", s, false
	}
	if s[1] == '/' {
		// Input starts with two slashes.
		return "", s[1:], false
	}

	i := strings.IndexByte(s, ' ')
	if i < 0 {
		i = len(s)
	}

	isCommand = true
	command = strings.ToUpper(s[1:i])
	args = strings.TrimLeft(s[i:], " ")
	return
}

func (app *App) handleInput(buffer, content string) error {
	if content == "" {
		return nil
	}

	cmdName, rawArgs, isCommand := parseCommand(content)
	if !isCommand {
		return noCommand(app, buffer, rawArgs)
	}
	if cmdName == "" {
		return fmt.Errorf("lone slash at the begining")
	}

	var chosenCMDName string
	var found bool
	for key := range commands {
		if !strings.HasPrefix(key, cmdName) {
			continue
		}
		if found {
			return fmt.Errorf("ambiguous command %q (could mean %v or %v)", cmdName, chosenCMDName, key)
		}
		chosenCMDName = key
		found = true
	}
	if !found {
		return fmt.Errorf("command %q doesn't exist", cmdName)
	}

	cmd := commands[chosenCMDName]

	var args []string
	if rawArgs != "" && cmd.MaxArgs != 0 {
		args = fieldsN(rawArgs, cmd.MaxArgs)
	}

	if len(args) < cmd.MinArgs {
		return fmt.Errorf("usage: %s %s", cmdName, cmd.Usage)
	}
	if buffer == Home && !cmd.AllowHome {
		return fmt.Errorf("command %q cannot be executed from home", cmdName)
	}

	return cmd.Handle(app, buffer, args)
}

func commandDoBuffer(app *App, buffer string, args []string) error {
	name := args[0]
	if !app.win.JumpBuffer(args[0]) {
		return fmt.Errorf("none of the buffers match %q", name)
	}

	return nil
}
