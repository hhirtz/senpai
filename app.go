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

type source int

const (
	uiEvent source = iota
	ircEvent
)

type bound struct {
	first time.Time
	last  time.Time

	firstMessage string
	lastMessage  string
}

// Compare returns 0 if line is within bounds, -1 if before, 1 if after.
func (b *bound) Compare(line *ui.Line) int {
	if line.At.Before(b.first) {
		return -1
	}
	if line.At.After(b.last) {
		return 1
	}
	if line.At.Equal(b.first) && line.Body.String() != b.firstMessage {
		return -1
	}
	if line.At.Equal(b.last) && line.Body.String() != b.lastMessage {
		return -1
	}
	return 0
}

// Update updates the bounds to include the given line.
func (b *bound) Update(line *ui.Line) {
	if line.At.IsZero() {
		return
	}
	if b.first.IsZero() || line.At.Before(b.first) {
		b.first = line.At
		b.firstMessage = line.Body.String()
	} else if b.last.IsZero() || line.At.After(b.last) {
		b.last = line.At
		b.lastMessage = line.Body.String()
	}
}

type event struct {
	src     source
	content interface{}
}

type App struct {
	win     *ui.UI
	s       *irc.Session
	pasting bool
	events  chan event

	cfg        Config
	highlights []string

	lastQuery     string
	messageBounds map[string]bound
	lastBuffer    string
}

func NewApp(cfg Config, lastBuffer string) (app *App, err error) {
	app = &App{
		cfg:           cfg,
		events:        make(chan event, eventChanSize),
		messageBounds: map[string]bound{},
		lastBuffer:    lastBuffer,
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
		NickColWidth:   cfg.NickColWidth,
		ChanColWidth:   cfg.ChanColWidth,
		MemberColWidth: cfg.MemberColWidth,
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
	if app.s != nil {
		app.s.Close()
	}
}

func (app *App) Run() {
	go app.uiLoop()
	go app.ircLoop()
	app.eventLoop()
}

func (app *App) CurrentBuffer() string {
	return app.win.CurrentBuffer()
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
			app.updatePrompt()
			var currentMembers []irc.Member
			if app.s != nil {
				currentMembers = app.s.Names(app.win.CurrentBuffer())
			}
			app.win.Draw(currentMembers)
		}
	}
}

// ircLoop maintains a connection to the IRC server by connecting and then
// forwarding IRC events to app.events repeatedly.
func (app *App) ircLoop() {
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
			src:     ircEvent,
			content: session,
		}
		go func() {
			for stop := range session.TypingStops() {
				app.events <- event{
					src:     ircEvent,
					content: stop,
				}
			}
		}()
		for msg := range in {
			if app.cfg.Debug {
				app.queueStatusLine(ui.Line{
					At:   time.Now(),
					Head: "IN --",
					Body: ui.PlainString(msg.String()),
				})
			}
			app.events <- event{
				src:     ircEvent,
				content: msg,
			}
		}
		app.events <- event{
			src:     ircEvent,
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
		close(out)
	}()
	return debugOut
}

// uiLoop retrieves events from the UI and forwards them to app.events for
// handling in app.eventLoop().
func (app *App) uiLoop() {
	for ev := range app.win.Events {
		app.events <- event{
			src:     uiEvent,
			content: ev,
		}
	}
}

// handleEvents handles a batch of events.
func (app *App) handleEvents(evs []event) {
	for _, ev := range evs {
		switch ev.src {
		case uiEvent:
			app.handleUIEvent(ev.content)
		case ircEvent:
			app.handleIRCEvent(ev.content)
		default:
			panic("unreachable")
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
	w, _ := app.win.Size()
	if ev.Buttons()&tcell.WheelUp != 0 {
		if x < app.cfg.ChanColWidth {
			// TODO scroll chan list
		} else if x > w-app.cfg.MemberColWidth {
			app.win.ScrollMemberUpBy(4)
		} else {
			app.win.ScrollUpBy(4)
			app.requestHistory()
		}
	}
	if ev.Buttons()&tcell.WheelDown != 0 {
		if x < app.cfg.ChanColWidth {
			// TODO scroll chan list
		} else if x > w-app.cfg.MemberColWidth {
			app.win.ScrollMemberDownBy(4)
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
		}
		app.win.ClickBuffer(-1)
	}
}

func (app *App) handleKeyEvent(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyCtrlC:
		if app.win.InputClear() {
			app.typing()
		}
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
		} else if ev.Modifiers() == tcell.ModCtrl {
			app.win.InputRightWord()
		} else {
			app.win.InputRight()
		}
	case tcell.KeyLeft:
		if ev.Modifiers() == tcell.ModAlt {
			app.win.PreviousBuffer()
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
	case tcell.KeyDown:
		if ev.Modifiers() == tcell.ModAlt {
			app.win.NextBuffer()
		} else {
			app.win.InputDown()
		}
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
		}
	case tcell.KeyDelete:
		ok := app.win.InputDelete()
		if ok {
			app.typing()
		}
	case tcell.KeyCtrlW:
		ok := app.win.InputDeleteWord()
		if ok {
			app.typing()
		}
	case tcell.KeyCtrlR:
		app.win.InputBackSearch()
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
		buffer := app.win.CurrentBuffer()
		input := app.win.InputEnter()
		err := app.handleInput(buffer, input)
		if err != nil {
			app.win.AddLine(app.win.CurrentBuffer(), ui.NotifyUnread, ui.Line{
				At:        time.Now(),
				Head:      "!!",
				HeadColor: tcell.ColorRed,
				Body:      ui.PlainSprintf("%q: %s", input, err),
			})
		}
	case tcell.KeyRune:
		app.win.InputRune(ev.Rune())
		app.typing()
	default:
		return
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
		t := time.Now()
		if bound, ok := app.messageBounds[buffer]; ok {
			t = bound.first
		}
		app.s.NewHistoryRequest(buffer).
			WithLimit(100).
			Before(t)
	}
}

func (app *App) handleIRCEvent(ev interface{}) {
	if ev == nil {
		app.s.Close()
		app.s = nil
		return
	}
	if s, ok := ev.(*irc.Session); ok {
		app.s = s
		return
	}
	if _, ok := ev.(irc.Typing); ok {
		// Just refresh the screen.
		return
	}

	msg := ev.(irc.Message)

	// Mutate IRC state
	ev = app.s.HandleMessage(msg)

	// Mutate UI state
	switch ev := ev.(type) {
	case irc.RegisteredEvent:
		for _, channel := range app.cfg.Channels {
			// TODO: group JOIN messages
			// TODO: support autojoining channels with keys
			app.s.Join(channel, "")
		}
		var body ui.StyledStringBuilder
		body.WriteString("Connected to the server")
		if app.s.Nick() != app.cfg.Nick {
			body.WriteString(" as ")
			body.WriteString(app.s.Nick())
		}
		app.win.AddLine(Home, ui.NotifyUnread, ui.Line{
			At:   msg.TimeOrNow(),
			Head: "--",
			Body: body.StyledString(),
		})
	case irc.SelfNickEvent:
		var body ui.StyledStringBuilder
		body.Grow(len(ev.FormerNick) + 4 + len(app.s.Nick()))
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(ev.FormerNick)
		body.SetStyle(tcell.StyleDefault)
		body.WriteRune('\u2192') // right arrow
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(app.s.Nick())
		app.addStatusLine(ui.Line{
			At:        msg.TimeOrNow(),
			Head:      "--",
			HeadColor: tcell.ColorGray,
			Body:      body.StyledString(),
			Highlight: true,
		})
	case irc.UserNickEvent:
		var body ui.StyledStringBuilder
		body.Grow(len(ev.FormerNick) + 4 + len(ev.User))
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(ev.FormerNick)
		body.SetStyle(tcell.StyleDefault)
		body.WriteRune('\u2192') // right arrow
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(ev.User)
		for _, c := range app.s.ChannelsSharedWith(ev.User) {
			app.win.AddLine(c, ui.NotifyNone, ui.Line{
				At:        msg.TimeOrNow(),
				Head:      "--",
				HeadColor: tcell.ColorGray,
				Body:      body.StyledString(),
				Mergeable: true,
			})
		}
	case irc.SelfJoinEvent:
		i, added := app.win.AddBuffer(ev.Channel)
		bounds, ok := app.messageBounds[ev.Channel]
		if added || !ok {
			app.s.NewHistoryRequest(ev.Channel).
				WithLimit(200).
				Before(msg.TimeOrNow())
		} else {
			app.s.NewHistoryRequest(ev.Channel).
				WithLimit(200).
				After(bounds.last)
		}
		if ev.Requested {
			app.win.JumpBufferIndex(i)
		}
		if ev.Topic != "" {
			app.printTopic(ev.Channel)
		}

		// Restore last buffer
		lastBuffer := app.lastBuffer
		if ev.Channel == lastBuffer {
			app.win.JumpBuffer(lastBuffer)
		}
	case irc.UserJoinEvent:
		var body ui.StyledStringBuilder
		body.Grow(len(ev.User) + 1)
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen))
		body.WriteByte('+')
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(ev.User)
		app.win.AddLine(ev.Channel, ui.NotifyNone, ui.Line{
			At:        msg.TimeOrNow(),
			Head:      "--",
			HeadColor: tcell.ColorGray,
			Body:      body.StyledString(),
			Mergeable: true,
		})
	case irc.SelfPartEvent:
		app.win.RemoveBuffer(ev.Channel)
		delete(app.messageBounds, ev.Channel)
	case irc.UserPartEvent:
		var body ui.StyledStringBuilder
		body.Grow(len(ev.User) + 1)
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
		body.WriteByte('-')
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(ev.User)
		app.win.AddLine(ev.Channel, ui.NotifyNone, ui.Line{
			At:        msg.TimeOrNow(),
			Head:      "--",
			HeadColor: tcell.ColorGray,
			Body:      body.StyledString(),
			Mergeable: true,
		})
	case irc.UserQuitEvent:
		var body ui.StyledStringBuilder
		body.Grow(len(ev.User) + 1)
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
		body.WriteByte('-')
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString(ev.User)
		for _, c := range ev.Channels {
			app.win.AddLine(c, ui.NotifyNone, ui.Line{
				At:        msg.TimeOrNow(),
				Head:      "--",
				HeadColor: tcell.ColorGray,
				Body:      body.StyledString(),
				Mergeable: true,
			})
		}
	case irc.TopicChangeEvent:
		var body ui.StyledStringBuilder
		body.Grow(len(ev.Topic) + 18)
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString("Topic changed to: ")
		topic := ui.IRCString(ev.Topic)
		body.WriteString(topic.String())
		app.win.AddLine(ev.Channel, ui.NotifyUnread, ui.Line{
			At:        msg.TimeOrNow(),
			Head:      "--",
			HeadColor: tcell.ColorGray,
			Body:      body.StyledString(),
		})
	case irc.ModeChangeEvent:
		var body ui.StyledStringBuilder
		body.Grow(len(ev.Mode) + 13)
		body.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGray))
		body.WriteString("Mode change: ")
		body.WriteString(ev.Mode)
		app.win.AddLine(ev.Channel, ui.NotifyUnread, ui.Line{
			At:        msg.TimeOrNow(),
			Head:      "--",
			HeadColor: tcell.ColorGray,
			Body:      body.StyledString(),
		})
	case irc.MessageEvent:
		buffer, line, hlNotification := app.formatMessage(ev)
		var notify ui.NotifyType
		if hlNotification {
			notify = ui.NotifyHighlight
		} else {
			notify = ui.NotifyUnread
		}
		app.win.AddLine(buffer, notify, line)
		if hlNotification {
			app.notifyHighlight(buffer, ev.User, line.Body.String())
		}
		if !app.s.IsChannel(msg.Params[0]) && !app.s.IsMe(ev.User) {
			app.lastQuery = msg.Prefix.Name
		}
		bounds := app.messageBounds[ev.Target]
		bounds.Update(&line)
		app.messageBounds[ev.Target] = bounds
	case irc.HistoryEvent:
		var linesBefore []ui.Line
		var linesAfter []ui.Line
		bounds, hasBounds := app.messageBounds[ev.Target]
		for _, m := range ev.Messages {
			switch ev := m.(type) {
			case irc.MessageEvent:
				_, line, _ := app.formatMessage(ev)
				if hasBounds {
					c := bounds.Compare(&line)
					if c < 0 {
						linesBefore = append(linesBefore, line)
					} else if c > 0 {
						linesAfter = append(linesAfter, line)
					}
				} else {
					linesAfter = append(linesAfter, line)
				}
			}
		}
		app.win.AddLines(ev.Target, linesBefore, linesAfter)
		if len(linesBefore) != 0 {
			bounds.Update(&linesBefore[0])
			bounds.Update(&linesBefore[len(linesBefore)-1])
		}
		if len(linesAfter) != 0 {
			bounds.Update(&linesAfter[0])
			bounds.Update(&linesAfter[len(linesAfter)-1])
		}
		app.messageBounds[ev.Target] = bounds
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
func (app *App) isHighlight(content string) bool {
	contentCf := app.s.Casemap(content)
	if app.highlights == nil {
		return strings.Contains(contentCf, app.s.NickCf())
	}
	for _, h := range app.highlights {
		if strings.Contains(contentCf, app.s.Casemap(h)) {
			return true
		}
	}
	return false
}

// notifyHighlight executes the "on-highlight" command according to the given
// message context.
func (app *App) notifyHighlight(buffer, nick, content string) {
	if app.cfg.OnHighlight == "" {
		return
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		return
	}
	here := "0"
	if buffer == app.win.CurrentBuffer() {
		here = "1"
	}
	cmd := exec.Command(sh, "-c", app.cfg.OnHighlight)
	cmd.Env = append(os.Environ(),
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
	if app.s == nil || app.cfg.NoTypings {
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
	isFromSelf := app.s.IsMe(ev.User)
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
	content = strings.TrimRightFunc(content, unicode.IsSpace)
	if isAction {
		content = content[7:]
	}
	var body ui.StyledStringBuilder
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
	buffer := app.win.CurrentBuffer()
	command := app.win.InputIsCommand()
	var prompt ui.StyledString
	if buffer == Home || command {
		prompt = ui.Styled(">",
			tcell.
				StyleDefault.
				Foreground(tcell.Color(app.cfg.Colors.Prompt)),
		)
	} else if app.s == nil {
		prompt = ui.Styled("<offline>",
			tcell.
				StyleDefault.
				Foreground(tcell.ColorRed),
		)
	} else {
		prompt = identString(app.s.Nick())
	}
	app.win.SetPrompt(prompt)
}

func (app *App) printTopic(buffer string) {
	var body string

	topic, who, at := app.s.Topic(buffer)
	if who == nil {
		body = fmt.Sprintf("Topic: %s", topic)
	} else {
		body = fmt.Sprintf("Topic (by %s, %s): %s", who, at.Local().Format("Mon Jan 2 15:04:05"), topic)
	}
	app.win.AddLine(buffer, ui.NotifyNone, ui.Line{
		At:        time.Now(),
		Head:      "--",
		HeadColor: tcell.ColorGray,
		Body:      ui.Styled(body, tcell.StyleDefault.Foreground(tcell.ColorGray)),
	})
}
