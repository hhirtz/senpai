package senpai

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"git.sr.ht/~taiite/senpai/irc"
	"git.sr.ht/~taiite/senpai/ui"
	"github.com/gdamore/tcell/v2"
)

const eventChanSize = 64

type event struct {
	src     string // empty string for UI events, network for IRC events
	content interface{}
}

type App struct {
	win      *ui.UI
	sessions map[string]*irc.Session // network to irc session
	pasting  bool
	events   chan event

	cfg        Config
	highlights []string

	lastQuery string
}

func NewApp(cfg Config) (app *App, err error) {
	app = &App{
		cfg:      cfg,
		sessions: map[string]*irc.Session{},
		events:   make(chan event, eventChanSize),
	}

	if cfg.Highlights != nil {
		app.highlights = make([]string, len(cfg.Highlights))
		for i := range app.highlights {
			app.highlights[i] = strings.ToLower(cfg.Highlights[i])
		}
	}

	mouse := true
	if cfg.Mouse != nil {
		mouse = *cfg.Mouse
	}

	app.win, err = ui.New(ui.Config{
		NickColWidth: cfg.NickColWidth,
		ChanColWidth: cfg.ChanColWidth,
		AutoComplete: func(cursorIdx int, text []rune) []ui.Completion {
			return app.completions(cursorIdx, text)
		},
		Mouse: mouse,
	})
	if err != nil {
		return
	}
	app.win.SetPrompt(ui.Styled(">",
		tcell.
			StyleDefault.
			Foreground(tcell.Color(app.cfg.Colors.Prompt))),
	)

	app.initWindow()

	return
}

func (app *App) Close() {
	app.win.Close()
	for _, s := range app.sessions {
		s.Close()
	}
}

func (app *App) Run() {
	go app.uiLoop()
	go app.ircLoop("*")
	app.eventLoop()
}

// eventLoop retrieves events (in batches) from the event channel and handle
// them, then draws the interface after each batch is handled.
func (app *App) eventLoop() {
	evs := make([]event, 0, eventChanSize)
	for !app.win.ShouldExit() {
		ev := <-app.events
		evs = evs[:0]
		evs = append(evs, ev)
	Batch:
		for i := 0; i < eventChanSize; i++ {
			select {
			case ev := <-app.events:
				evs = append(evs, ev)
			default:
				break Batch
			}
		}

		app.handleEvents(evs)
		if !app.pasting {
			app.setStatus()
			app.win.Draw()
		}
	}
}

// ircLoop maintains a connection to the IRC server by connecting and then
// forwarding IRC events to app.events repeatedly.
func (app *App) ircLoop(network string) {
	var auth irc.SASLClient
	if app.cfg.Password != nil {
		auth = &irc.SASLPlain{
			Username: app.cfg.User,
			Password: *app.cfg.Password,
		}
	}
	params := irc.SessionParams{
		Nickname: app.cfg.Nick,
		Username: app.cfg.User,
		RealName: app.cfg.Real,
		Auth:     auth,
	}
	for !app.win.ShouldExit() {
		conn := app.connect()
		in, out := irc.ChanInOut(conn)
		if app.cfg.Debug {
			out = app.debugOutputMessages(out)
		}
		session := irc.NewSession(out, params)
		app.events <- event{
			src:     network,
			content: session,
		}
		for msg := range in {
			if app.cfg.Debug {
				app.queueStatusLine(ui.Line{
					At:   time.Now(),
					Head: "IN --",
					Body: ui.PlainString(msg.String()),
				})
			}
			app.events <- event{
				src:     network,
				content: msg,
			}
		}
		app.events <- event{
			src:     network,
			content: nil,
		}
		app.queueStatusLine(ui.Line{
			Head:      "!!",
			HeadColor: tcell.ColorRed,
			Body:      ui.PlainString("Connection lost"),
		})
		time.Sleep(10 * time.Second)
	}
}

func (app *App) connect() net.Conn {
	for {
		app.queueStatusLine(ui.Line{
			Head: "--",
			Body: ui.PlainSprintf("Connecting to %s...", app.cfg.Addr),
		})
		conn, err := app.tryConnect()
		if err == nil {
			return conn
		}
		app.queueStatusLine(ui.Line{
			Head:      "!!",
			HeadColor: tcell.ColorRed,
			Body:      ui.PlainSprintf("Connection failed: %v", err),
		})
		time.Sleep(1 * time.Minute)
	}
}

func (app *App) tryConnect() (conn net.Conn, err error) {
	addr := app.cfg.Addr
	colonIdx := strings.LastIndexByte(addr, ':')
	bracketIdx := strings.LastIndexByte(addr, ']')
	if colonIdx <= bracketIdx {
		// either colonIdx < 0, or the last colon is before a ']' (end
		// of IPv6 address. -> missing port
		if app.cfg.NoTLS {
			addr += ":6667"
		} else {
			addr += ":6697"
		}
	}

	conn, err = net.Dial("tcp", addr)
	if err != nil {
		return
	}

	if !app.cfg.NoTLS {
		host, _, _ := net.SplitHostPort(addr) // should succeed since net.Dial did.
		conn = tls.Client(conn, &tls.Config{
			ServerName: host,
			NextProtos: []string{"irc"},
		})
		err = conn.(*tls.Conn).Handshake()
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	return
}

func (app *App) debugOutputMessages(out chan<- irc.Message) chan<- irc.Message {
	debugOut := make(chan irc.Message, cap(out))
	go func() {
		for msg := range debugOut {
			app.queueStatusLine(ui.Line{
				At:   time.Now(),
				Head: "OUT --",
				Body: ui.PlainString(msg.String()),
			})
			out <- msg
		}
	}()
	return debugOut
}

// uiLoop retrieves events from the UI and forwards them to app.events for
// handling in app.eventLoop().
func (app *App) uiLoop() {
	for {
		ev, ok := <-app.win.Events
		if !ok {
			break
		}
		app.events <- event{
			src:     "",
			content: ev,
		}
	}
}

// handleEvents handles a batch of events.
func (app *App) handleEvents(evs []event) {
	for _, ev := range evs {
		if ev.src == "" {
			app.handleUIEvent(ev.content)
		} else {
			app.handleIRCEvent(ev.src, ev.content)
		}
	}
}

func (app *App) handleUIEvent(ev interface{}) {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		app.win.Resize()
	case *tcell.EventPaste:
		app.pasting = ev.Start()
	case *tcell.EventMouse:
		app.handleMouseEvent(ev)
	case *tcell.EventKey:
		app.handleKeyEvent(ev)
	case ui.Line:
		app.addStatusLine(ev)
	default:
		return
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
		} else if ev.Modifiers() == tcell.ModCtrl {
			app.win.InputRightWord()
		} else {
			app.win.InputRight()
		}
	case tcell.KeyLeft:
		if ev.Modifiers() == tcell.ModAlt {
			app.win.PreviousBuffer()
			app.updatePrompt()
		} else if ev.Modifiers() == tcell.ModCtrl {
			app.win.InputLeftWord()
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
	case tcell.KeyBackspace, tcell.KeyBackspace2:
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
	case tcell.KeyCtrlW:
		ok := app.win.InputDeleteWord()
		if ok {
			app.typing()
			app.updatePrompt()
		}
	case tcell.KeyTab:
		ok := app.win.InputAutoComplete(1)
		if ok {
			app.typing()
		}
	case tcell.KeyBacktab:
		ok := app.win.InputAutoComplete(-1)
		if ok {
			app.typing()
		}
	case tcell.KeyCR, tcell.KeyLF:
		network, buffer := app.win.CurrentBuffer()
		input := app.win.InputEnter()
		err := app.handleInput(network, buffer, input)
		if err != nil {
			app.win.AddLine(network, buffer, false, ui.Line{
				At:        time.Now(),
				Head:      "!!",
				HeadColor: tcell.ColorRed,
				Body:      ui.PlainSprintf("%q: %s", input, err),
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

// requestHistory is a wrapper around irc.Session.RequestHistory to only request
// history when needed.
func (app *App) requestHistory() {
	s := app.currentSession()
	if s == nil {
		return
	}
	_, buffer := app.win.CurrentBuffer()
	if app.win.IsAtTop() && buffer != "" {
		t := time.Now()
		if oldest := app.win.CurrentBufferOldestTime(); oldest != nil {
			t = *oldest
		}
		s.NewHistoryRequest(buffer).
			WithLimit(100).
			Before(t)
	}
}

func (app *App) handleIRCEvent(network string, ev interface{}) {
	if ev == nil {
		s, ok := app.sessions[network]
		if !ok {
			return
		}
		s.Close()
		delete(app.sessions, network)
		return
	}
	if s, ok := ev.(*irc.Session); ok {
		app.sessions[network] = s
		app.win.AddBuffer(network, "")
		return
	}

	s, ok := app.sessions[network]
	if !ok {
		return
	}

	msg := ev.(irc.Message)

	// Mutate IRC state
	ev = s.HandleMessage(msg)

	// Mutate UI state
	switch ev := ev.(type) {
	case irc.RegisteredEvent:
		body := new(ui.StyledStringBuilder)
		body.WriteString("Connected to the server")
		if s.Nick() != app.cfg.Nick {
			body.WriteString(" as ")
			body.WriteString(s.Nick())
		}
		app.win.AddLine(network, "", false, ui.Line{
			At:   msg.TimeOrNow(),
			Head: "--",
			Body: body.StyledString(),
		})
	case irc.SelfNickEvent:
		body := new(ui.StyledStringBuilder)
		body.Grow(len(ev.FormerNick) + 4 + len(s.Nick()))
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(ev.FormerNick)
		body.SetStyle(tcell.StyleDefault)
		body.WriteRune('\u2192') // right arrow
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(s.Nick())
		app.addStatusLine(ui.Line{
			At:        msg.TimeOrNow(),
			Head:      "--",
			HeadColor: tcell.ColorGray,
			Body:      body.StyledString(),
			Highlight: true,
		})
	case irc.UserNickEvent:
		body := new(ui.StyledStringBuilder)
		body.Grow(len(ev.FormerNick) + 4 + len(ev.User))
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(ev.FormerNick)
		body.SetStyle(tcell.StyleDefault)
		body.WriteRune('\u2192') // right arrow
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(ev.User)
		for _, c := range s.ChannelsSharedWith(ev.User) {
			app.win.AddLine(network, c, false, ui.Line{
				At:        msg.TimeOrNow(),
				Head:      "--",
				HeadColor: tcell.ColorGray,
				Body:      body.StyledString(),
				Mergeable: true,
			})
		}
	case irc.SelfJoinEvent:
		app.win.AddBuffer(network, ev.Channel)
		s.NewHistoryRequest(ev.Channel).
			WithLimit(200).
			Before(msg.TimeOrNow())
	case irc.UserJoinEvent:
		body := new(ui.StyledStringBuilder)
		body.Grow(len(ev.User) + 1)
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
		body.WriteByte('+')
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(ev.User)
		app.win.AddLine(network, ev.Channel, false, ui.Line{
			At:        msg.TimeOrNow(),
			Head:      "--",
			HeadColor: tcell.ColorGray,
			Body:      body.StyledString(),
			Mergeable: true,
		})
	case irc.SelfPartEvent:
		app.win.RemoveBuffer(network, ev.Channel)
	case irc.UserPartEvent:
		body := new(ui.StyledStringBuilder)
		body.Grow(len(ev.User) + 1)
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
		body.WriteByte('-')
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(ev.User)
		app.win.AddLine(network, ev.Channel, false, ui.Line{
			At:        msg.TimeOrNow(),
			Head:      "--",
			HeadColor: tcell.ColorGray,
			Body:      body.StyledString(),
			Mergeable: true,
		})
	case irc.UserQuitEvent:
		body := new(ui.StyledStringBuilder)
		body.Grow(len(ev.User) + 1)
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
		body.WriteByte('-')
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(ev.User)
		for _, c := range ev.Channels {
			app.win.AddLine(network, c, false, ui.Line{
				At:        msg.TimeOrNow(),
				Head:      "--",
				HeadColor: tcell.ColorGray,
				Body:      body.StyledString(),
				Mergeable: true,
			})
		}
	case irc.TopicChangeEvent:
		body := new(ui.StyledStringBuilder)
		body.Grow(len(ev.Topic) + 18)
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString("Topic changed to: ")
		body.WriteString(ev.Topic)
		app.win.AddLine(network, ev.Channel, false, ui.Line{
			At:        msg.TimeOrNow(),
			Head:      "--",
			HeadColor: tcell.ColorGray,
			Body:      body.StyledString(),
		})
	case irc.MessageEvent:
		buffer, line, hlNotification := app.formatMessage(s, ev)
		app.win.AddLine(network, buffer, hlNotification, line)
		if hlNotification {
			app.notifyHighlight(network, buffer, ev.User, line.Body.String())
		}
		if !s.IsChannel(msg.Params[0]) && !s.IsMe(ev.User) {
			app.lastQuery = msg.Prefix.Name
		}
	case irc.HistoryEvent:
		var lines []ui.Line
		for _, m := range ev.Messages {
			switch ev := m.(type) {
			case irc.MessageEvent:
				_, line, _ := app.formatMessage(s, ev)
				lines = append(lines, line)
			}
		}
		app.win.AddLines(network, ev.Target, lines)
	case irc.ErrorEvent:
		if isBlackListed(msg.Command) {
			break
		}
		var head string
		var body string
		switch ev.Severity {
		case irc.SeverityFail:
			head = "--"
			body = fmt.Sprintf("Error (code %s): %s", ev.Code, ev.Message)
		case irc.SeverityWarn:
			head = "--"
			body = fmt.Sprintf("Warning (code %s): %s", ev.Code, ev.Message)
		case irc.SeverityNote:
			head = ev.Code + " --"
			body = ev.Message
		default:
			panic("unreachable")
		}
		app.addStatusLine(ui.Line{
			At:   msg.TimeOrNow(),
			Head: head,
			Body: ui.PlainString(body),
		})
	}
}

func isBlackListed(command string) bool {
	switch command {
	case "002", "003", "004", "422":
		// useless connection messages
		return true
	}
	return false
}

// isHighlight reports whether the given message content is a highlight.
func (app *App) isHighlight(s *irc.Session, content string) bool {
	contentCf := s.Casemap(content)
	if app.highlights == nil {
		return strings.Contains(contentCf, s.NickCf())
	}
	for _, h := range app.highlights {
		if strings.Contains(contentCf, s.Casemap(h)) {
			return true
		}
	}
	return false
}

// notifyHighlight executes the "on-highlight" command according to the given
// message context.
func (app *App) notifyHighlight(network, buffer, nick, content string) {
	if app.cfg.OnHighlight == "" {
		return
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		return
	}
	here := "0"
	currentNetwork, currentBuffer := app.win.CurrentBuffer()
	if network == currentNetwork && buffer == currentBuffer {
		here = "1"
	}
	cmd := exec.Command(sh, "-c", app.cfg.OnHighlight)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("NETWORK=%s", network),
		fmt.Sprintf("BUFFER=%s", buffer),
		fmt.Sprintf("HERE=%s", here),
		fmt.Sprintf("SENDER=%s", nick),
		fmt.Sprintf("MESSAGE=%s", content),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		body := fmt.Sprintf("Failed to invoke on-highlight command: %v. Output: %q", err, string(output))
		app.addStatusLine(ui.Line{
			At:        time.Now(),
			Head:      "!!",
			HeadColor: tcell.ColorRed,
			Body:      ui.PlainString(body),
		})
	}
}

// typing sends typing notifications to the IRC server according to the user
// input.
func (app *App) typing() {
	if app.cfg.NoTypings {
		return
	}
	s := app.currentSession()
	if s == nil {
		return
	}
	_, buffer := app.win.CurrentBuffer()
	if buffer == "" {
		return
	}
	if app.win.InputLen() == 0 {
		s.TypingStop(buffer)
	} else if !app.win.InputIsCommand() {
		s.Typing(buffer)
	}
}

// completions computes the list of completions given the input text and the
// cursor position.
func (app *App) completions(cursorIdx int, text []rune) []ui.Completion {
	var cs []ui.Completion

	if len(text) == 0 {
		return cs
	}

	s := app.currentSession()
	_, buffer := app.win.CurrentBuffer()
	if s != nil {
		if s.IsChannel(buffer) {
			cs = app.completionsChannelTopic(cs, cursorIdx, text, s)
			cs = app.completionsChannelMembers(cs, cursorIdx, text, s)
		}
		cs = app.completionsMsg(cs, cursorIdx, text, s)
	}

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
func (app *App) formatMessage(s *irc.Session, ev irc.MessageEvent) (buffer string, line ui.Line, hlNotification bool) {
	isFromSelf := s.IsMe(ev.User)
	isHighlight := app.isHighlight(s, ev.Content)
	isAction := strings.HasPrefix(ev.Content, "\x01ACTION")
	isQuery := !ev.TargetIsChannel && ev.Command == "PRIVMSG"
	isNotice := ev.Command == "NOTICE"

	if !ev.TargetIsChannel && isNotice {
		_, buffer = app.win.CurrentBuffer()
	} else if !ev.TargetIsChannel {
		buffer = ""
	} else {
		buffer = ev.Target
	}

	hlLine := ev.TargetIsChannel && isHighlight && !isFromSelf
	hlNotification = (isHighlight || isQuery) && !isFromSelf

	head := ev.User
	headColor := tcell.ColorWhite
	if isFromSelf && isQuery {
		head = "\u2192 " + ev.Target
		headColor = identColor(ev.Target)
	} else if isAction || isNotice {
		head = "*"
	} else {
		headColor = identColor(head)
	}

	content := strings.TrimSuffix(ev.Content, "\x01")
	content = strings.TrimRightFunc(ev.Content, unicode.IsSpace)
	if isAction {
		content = content[7:]
	}
	body := new(ui.StyledStringBuilder)
	if isNotice {
		color := identColor(ev.User)
		body.SetStyle(tcell.StyleDefault.Foreground(color))
		body.WriteString(ev.User)
		body.SetStyle(tcell.StyleDefault)
		body.WriteString(": ")
		body.WriteStyledString(ui.IRCString(content))
	} else if isAction {
		color := identColor(ev.User)
		body.SetStyle(tcell.StyleDefault.Foreground(color))
		body.WriteString(ev.User)
		body.SetStyle(tcell.StyleDefault)
		body.WriteStyledString(ui.IRCString(content))
	} else {
		body.WriteStyledString(ui.IRCString(content))
	}

	line = ui.Line{
		At:        ev.Time,
		Head:      head,
		HeadColor: headColor,
		Body:      body.StyledString(),
		Highlight: hlLine,
	}
	return
}

// updatePrompt changes the prompt text according to the application context.
func (app *App) updatePrompt() {
	_, buffer := app.win.CurrentBuffer()
	s := app.currentSession()
	command := app.win.InputIsCommand()
	var prompt ui.StyledString
	if buffer == "" || command || s == nil {
		prompt = ui.Styled(">",
			tcell.
				StyleDefault.
				Foreground(tcell.Color(app.cfg.Colors.Prompt)),
		)
	} else {
		prompt = identString(s.Nick())
	}
	app.win.SetPrompt(prompt)
}
