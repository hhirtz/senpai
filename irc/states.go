package irc

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type SASLClient interface {
	Handshake() (mech string)
	Respond(challenge string) (res string, err error)
}

type SASLPlain struct {
	Username string
	Password string
}

func (auth *SASLPlain) Handshake() (mech string) {
	mech = "PLAIN"
	return
}

func (auth *SASLPlain) Respond(challenge string) (res string, err error) {
	if challenge != "+" {
		err = errors.New("unexpected challenge")
		return
	}

	user := []byte(auth.Username)
	pass := []byte(auth.Password)
	payload := bytes.Join([][]byte{user, user, pass}, []byte{0})
	res = base64.StdEncoding.EncodeToString(payload)

	return
}

// SupportedCapabilities is the set of capabilities supported by this library.
var SupportedCapabilities = map[string]struct{}{
	"account-notify":    {},
	"account-tag":       {},
	"away-notify":       {},
	"batch":             {},
	"cap-notify":        {},
	"draft/chathistory": {},
	"echo-message":      {},
	"extended-join":     {},
	"invite-notify":     {},
	"labeled-response":  {},
	"message-tags":      {},
	"multi-prefix":      {},
	"server-time":       {},
	"sasl":              {},
	"setname":           {},
	"userhost-in-names": {},
}

// Values taken by the "@+typing=" client tag.  TypingUnspec means the value or
// tag is absent.
const (
	TypingUnspec = iota
	TypingActive
	TypingPaused
	TypingDone
)

// action contains the arguments of a user action.
//
// To keep connection reads and writes in a single coroutine, the library
// interface functions like Join("#channel") or PrivMsg("target", "message")
// don't interact with the IRC session directly.  Instead, they push an action
// in the action channel.  This action is then processed by the correct
// coroutine.
type action interface{}

type (
	actionSendRaw struct {
		raw string
	}

	actionJoin struct {
		Channel string
	}
	actionPart struct {
		Channel string
		Reason  string
	}
	actionSetTopic struct {
		Channel string
		Topic   string
	}

	actionPrivMsg struct {
		Target  string
		Content string
	}

	actionTyping struct {
		Channel string
	}
	actionTypingStop struct {
		Channel string
	}

	actionRequestHistory struct {
		Target string
		Before time.Time
	}
)

// User is a known IRC user (we share a channel with it).
type User struct {
	Name    *Prefix // the nick, user and hostname of the user if known.
	AwayMsg string  // the away message if the user is away, "" otherwise.
}

// Channel is a joined channel.
type Channel struct {
	Name      string           // the name of the channel.
	Members   map[*User]string // the set of members associated with their membership.
	Topic     string           // the topic of the channel, or "" if absent.
	TopicWho  *Prefix          // the name of the last user who set the topic.
	TopicTime time.Time        // the last time the topic has been changed.
	Secret    bool             // whether the channel is on the server channel list.

	complete bool // whether this stucture is fully initialized.
}

// SessionParams defines how to connect to an IRC server.
type SessionParams struct {
	Nickname string
	Username string
	RealName string

	Auth SASLClient

	Debug bool // whether the Session should report all messages it sends and receive.
}

// Session is an IRC session/connection/whatever.
type Session struct {
	conn io.ReadWriteCloser
	msgs chan Message // incoming messages.
	acts chan action  // user actions.
	evts chan Event   // events sent to the user.

	debug bool

	running      atomic.Value // bool
	registered   bool
	typings      *Typings             // incoming typing notifications.
	typingStamps map[string]time.Time // user typing instants.

	nick   string
	nickCf string // casemapped nickname.
	user   string
	real   string
	acct   string
	host   string
	auth   SASLClient

	availableCaps map[string]string
	enabledCaps   map[string]struct{}
	features      map[string]string // server ISUPPORT advertized features.

	users     map[string]*User        // known users.
	channels  map[string]Channel      // joined channels.
	chBatches map[string]HistoryEvent // channel history batches being processed.
}

// NewSession starts an IRC session from the given connection and session
// parameters.
//
// It returns an error when the paramaters are invalid, or when it cannot write
// to the connection.
func NewSession(conn io.ReadWriteCloser, params SessionParams) (*Session, error) {
	s := &Session{
		conn:          conn,
		msgs:          make(chan Message, 64),
		acts:          make(chan action, 64),
		evts:          make(chan Event, 64),
		debug:         params.Debug,
		typings:       NewTypings(),
		typingStamps:  map[string]time.Time{},
		nick:          params.Nickname,
		nickCf:        CasemapASCII(params.Nickname),
		user:          params.Username,
		real:          params.RealName,
		auth:          params.Auth,
		availableCaps: map[string]string{},
		enabledCaps:   map[string]struct{}{},
		features:      map[string]string{},
		users:         map[string]*User{},
		channels:      map[string]Channel{},
		chBatches:     map[string]HistoryEvent{},
	}

	if s.nick == "" {
		return nil, errors.New("no nickname specified")
	}
	if s.user == "" {
		s.user = s.nick
	}
	if s.real == "" {
		s.real = s.nick
	}

	s.running.Store(true)

	err := s.send("CAP LS 302\r\nNICK %s\r\nUSER %s 0 * :%s\r\n", s.nick, s.user, s.real)
	if err != nil {
		return nil, err
	}

	go func() {
		r := bufio.NewScanner(conn)

		for r.Scan() {
			line := r.Text()
			msg, err := ParseMessage(line)
			if err != nil {
				continue
			}
			valid := msg.IsValid()
			if s.debug {
				s.evts <- RawMessageEvent{Message: line, IsValid: valid}
			}
			if valid {
				s.msgs <- msg
			}
		}

		s.Stop()
	}()

	go s.run()

	return s, nil
}

// Running reports whether we are still connected to the server.
func (s *Session) Running() bool {
	return s.running.Load().(bool)
}

// Stop stops the session and closes the connection.
func (s *Session) Stop() {
	if !s.Running() {
		return
	}
	s.running.Store(false)
	_ = s.conn.Close()
	close(s.acts)
	close(s.evts)
	close(s.msgs)
	s.typings.Stop()
}

// Poll returns the event channel where incoming events are reported.
func (s *Session) Poll() (events <-chan Event) {
	return s.evts
}

// HasCapability reports whether the given capability has been negociated
// successfully.
func (s *Session) HasCapability(capability string) bool {
	_, ok := s.enabledCaps[capability]
	return ok
}

func (s *Session) Nick() string {
	return s.nick
}

// NickCf is our casemapped nickname.
func (s *Session) NickCf() string {
	return s.nickCf
}

func (s *Session) IsChannel(name string) bool {
	chantypes, ok := s.features["CHANTYPES"]
	if !ok {
		chantypes = "#&"
	}
	return strings.IndexAny(name, chantypes) == 0
}

func (s *Session) Casemap(name string) string {
	if s.features["CASEMAPPING"] == "ascii" {
		return CasemapASCII(name)
	} else {
		return CasemapRFC1459(name)
	}
}

// Users returns the list of all known nicknames.
func (s *Session) Users() []string {
	users := make([]string, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u.Name.Name)
	}
	return users
}

// Names returns the list of users in the given channel, or nil if this channel
// is not known by the session.
func (s *Session) Names(channel string) []Member {
	var names []Member
	if c, ok := s.channels[s.Casemap(channel)]; ok {
		names = make([]Member, 0, len(c.Members))
		for u, pl := range c.Members {
			names = append(names, Member{
				PowerLevel: pl,
				Name:       u.Name.Copy(),
			})
		}
	}
	return names
}

// Typings returns the list of nickname who are currently typing.
func (s *Session) Typings(target string) []string {
	targetCf := s.Casemap(target)
	var res []string
	for t := range s.typings.targets {
		if targetCf == t.Target {
			res = append(res, s.users[t.Name].Name.Name)
		}
	}
	return res
}

func (s *Session) ChannelsSharedWith(name string) []string {
	var user *User
	if u, ok := s.users[s.Casemap(name)]; ok {
		user = u
	} else {
		return nil
	}
	var channels []string
	for _, c := range s.channels {
		if _, ok := c.Members[user]; ok {
			channels = append(channels, c.Name)
		}
	}
	return channels
}

func (s *Session) Topic(channel string) (topic string, who *Prefix, at time.Time) {
	channelCf := s.Casemap(channel)
	if c, ok := s.channels[channelCf]; ok {
		topic = c.Topic
		who = c.TopicWho
		at = c.TopicTime
	}
	return
}

// SendRaw sends its given argument verbatim to the server.
func (s *Session) SendRaw(raw string) {
	s.acts <- actionSendRaw{raw}
}

func (s *Session) sendRaw(act actionSendRaw) (err error) {
	err = s.send("%s\r\n", act.raw)
	return
}

func (s *Session) Join(channel string) {
	s.acts <- actionJoin{channel}
}

func (s *Session) join(act actionJoin) (err error) {
	err = s.send("JOIN %s\r\n", act.Channel)
	return
}

func (s *Session) Part(channel, reason string) {
	s.acts <- actionPart{channel, reason}
}

func (s *Session) part(act actionPart) (err error) {
	err = s.send("PART %s :%s\r\n", act.Channel, act.Reason)
	return
}

func (s *Session) SetTopic(channel, topic string) {
	s.acts <- actionSetTopic{channel, topic}
}

func (s *Session) setTopic(act actionSetTopic) (err error) {
	err = s.send("TOPIC %s :%s\r\n", act.Channel, act.Topic)
	return
}

func (s *Session) PrivMsg(target, content string) {
	s.acts <- actionPrivMsg{target, content}
}

func (s *Session) privMsg(act actionPrivMsg) (err error) {
	err = s.send("PRIVMSG %s :%s\r\n", act.Target, act.Content)
	target := s.Casemap(act.Target)
	delete(s.typingStamps, target)
	return
}

func (s *Session) Typing(channel string) {
	s.acts <- actionTyping{channel}
}

func (s *Session) typing(act actionTyping) (err error) {
	if _, ok := s.enabledCaps["message-tags"]; !ok {
		return
	}

	to := s.Casemap(act.Channel)
	now := time.Now()

	if t, ok := s.typingStamps[to]; ok && now.Sub(t).Seconds() < 3.0 {
		return
	}

	s.typingStamps[to] = now

	err = s.send("@+typing=active TAGMSG %s\r\n", act.Channel)
	return
}

func (s *Session) TypingStop(channel string) {
	s.acts <- actionTypingStop{channel}
}

func (s *Session) typingStop(act actionTypingStop) (err error) {
	if _, ok := s.enabledCaps["message-tags"]; !ok {
		return
	}

	err = s.send("@+typing=done TAGMSG %s\r\n", act.Channel)
	return
}

func (s *Session) RequestHistory(target string, before time.Time) {
	s.acts <- actionRequestHistory{target, before}
}

func (s *Session) requestHistory(act actionRequestHistory) (err error) {
	if _, ok := s.enabledCaps["draft/chathistory"]; !ok {
		return
	}

	t := act.Before.UTC()
	err = s.send("CHATHISTORY BEFORE %s timestamp=%04d-%02d-%02dT%02d:%02d:%02d.%03dZ 100\r\n", act.Target, t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second()+1, t.Nanosecond()/1e6)

	return
}

func (s *Session) run() {
	for s.Running() {
		var err error

		select {
		case act, ok := <-s.acts:
			if !ok {
				break
			}
			switch act := act.(type) {
			case actionSendRaw:
				err = s.sendRaw(act)
			case actionJoin:
				err = s.join(act)
			case actionPart:
				err = s.part(act)
			case actionSetTopic:
				err = s.setTopic(act)
			case actionPrivMsg:
				err = s.privMsg(act)
			case actionTyping:
				err = s.typing(act)
			case actionTypingStop:
				err = s.typingStop(act)
			case actionRequestHistory:
				err = s.requestHistory(act)
			}
		case msg, ok := <-s.msgs:
			if !ok {
				break
			}
			if s.registered {
				err = s.handle(msg)
			} else {
				err = s.handleStart(msg)
			}
		case t, ok := <-s.typings.Stops():
			if !ok {
				break
			}
			s.evts <- TagEvent{
				User:   s.users[t.Name].Name,
				Target: s.channels[t.Target].Name,
				Typing: TypingDone,
				Time:   time.Now(),
			}
		}

		if err != nil {
			s.evts <- err
		}
	}
}

func (s *Session) handleStart(msg Message) (err error) {
	switch msg.Command {
	case "AUTHENTICATE":
		if s.auth != nil {
			var res string

			res, err = s.auth.Respond(msg.Params[0])
			if err != nil {
				err = s.send("AUTHENTICATE *\r\n")
				return
			}

			err = s.send("AUTHENTICATE %s\r\n", res)
			if err != nil {
				return
			}
		}
	case rplLoggedin:
		err = s.send("CAP END\r\n")
		if err != nil {
			return
		}

		s.acct = msg.Params[2]
		s.host = ParsePrefix(msg.Params[1]).Host
	case errNicklocked, errSaslfail, errSasltoolong, errSaslaborted, errSaslalready, rplSaslmechs:
		err = s.send("CAP END\r\n")
		if err != nil {
			return
		}
	case "CAP":
		switch msg.Params[1] {
		case "LS":
			var willContinue bool
			var ls string

			if msg.Params[2] == "*" {
				willContinue = true
				ls = msg.Params[3]
			} else {
				willContinue = false
				ls = msg.Params[2]
			}

			for _, c := range ParseCaps(ls) {
				if c.Enable {
					s.availableCaps[c.Name] = c.Value
				} else {
					delete(s.availableCaps, c.Name)
				}
			}

			if !willContinue {
				var req strings.Builder

				for c := range s.availableCaps {
					if _, ok := SupportedCapabilities[c]; !ok {
						continue
					}

					_, _ = fmt.Fprintf(&req, "CAP REQ %s\r\n", c)
				}

				_, ok := s.availableCaps["sasl"]
				if s.auth == nil || !ok {
					_, _ = fmt.Fprintf(&req, "CAP END\r\n")
				}

				err = s.send(req.String())
				if err != nil {
					return
				}
			}
		default:
			s.handle(msg)
		}
	case errNicknameinuse:
		err = s.send("NICK %s_\r\n", msg.Params[1])
		if err != nil {
			return
		}
	default:
		err = s.handle(msg)
	}

	return
}

func (s *Session) handle(msg Message) (err error) {
	if id, ok := msg.Tags["batch"]; ok {
		if b, ok := s.chBatches[id]; ok {
			s.chBatches[id] = HistoryEvent{
				Target:   b.Target,
				Messages: append(b.Messages, s.privmsgToEvent(msg)),
			}
			return
		}
	}

	switch msg.Command {
	case rplWelcome:
		s.nick = msg.Params[0]
		s.nickCf = s.Casemap(s.nick)
		s.registered = true
		s.users[s.nickCf] = &User{Name: &Prefix{
			Name: s.nick, User: s.user, Host: s.host,
		}}
		s.evts <- RegisteredEvent{}

		if s.host == "" {
			err = s.send("WHO %s\r\n", s.nick)
			if err != nil {
				return
			}
		}
	case rplIsupport:
		s.updateFeatures(msg.Params[1 : len(msg.Params)-1])
	case rplWhoreply:
		if s.nickCf == s.Casemap(msg.Params[5]) {
			s.host = msg.Params[3]
		}
	case "CAP":
		switch msg.Params[1] {
		case "ACK":
			for _, c := range strings.Split(msg.Params[2], " ") {
				s.enabledCaps[c] = struct{}{}

				if s.auth != nil && c == "sasl" {
					h := s.auth.Handshake()
					err = s.send("AUTHENTICATE %s\r\n", h)
					if err != nil {
						return
					}
				} else if len(s.channels) != 0 && c == "multi-prefix" {
					// TODO merge NAMES commands
					var sb strings.Builder
					sb.Grow(512)
					for _, c := range s.channels {
						sb.WriteString("NAMES ")
						sb.WriteString(c.Name)
						sb.WriteString("\r\n")
					}
					err = s.send(sb.String())
					if err != nil {
						return
					}
				}
			}
		case "NAK":
			for _, c := range strings.Split(msg.Params[2], " ") {
				delete(s.enabledCaps, c)
			}
		case "NEW":
			diff := ParseCaps(msg.Params[2])

			for _, c := range diff {
				if c.Enable {
					s.availableCaps[c.Name] = c.Value
				} else {
					delete(s.availableCaps, c.Name)
				}
			}

			var req strings.Builder

			for _, c := range diff {
				_, ok := SupportedCapabilities[c.Name]
				if !c.Enable || !ok {
					continue
				}

				_, _ = fmt.Fprintf(&req, "CAP REQ %s\r\n", c.Name)
			}

			_, ok := s.availableCaps["sasl"]
			if s.acct == "" && ok {
				// TODO authenticate
			}

			err = s.send(req.String())
			if err != nil {
				return
			}
		case "DEL":
			diff := ParseCaps(msg.Params[2])

			for i := range diff {
				diff[i].Enable = !diff[i].Enable
			}

			for _, c := range diff {
				if c.Enable {
					s.availableCaps[c.Name] = c.Value
				} else {
					delete(s.availableCaps, c.Name)
				}
			}

			var req strings.Builder

			for _, c := range diff {
				_, ok := SupportedCapabilities[c.Name]
				if !c.Enable || !ok {
					continue
				}

				_, _ = fmt.Fprintf(&req, "CAP REQ %s\r\n", c.Name)
			}

			_, ok := s.availableCaps["sasl"]
			if s.acct == "" && ok {
				// TODO authenticate
			}

			err = s.send(req.String())
			if err != nil {
				return
			}
		}
	case "JOIN":
		nickCf := s.Casemap(msg.Prefix.Name)
		channelCf := s.Casemap(msg.Params[0])

		if nickCf == s.nickCf {
			s.channels[channelCf] = Channel{
				Name:    msg.Params[0],
				Members: map[*User]string{},
			}
		} else if c, ok := s.channels[channelCf]; ok {
			if _, ok := s.users[nickCf]; !ok {
				s.users[nickCf] = &User{Name: msg.Prefix.Copy()}
			}
			c.Members[s.users[nickCf]] = ""
			t := msg.TimeOrNow()

			s.evts <- UserJoinEvent{
				User:    msg.Prefix.Copy(),
				Channel: c.Name,
				Time:    t,
			}
		}
	case "PART":
		nickCf := s.Casemap(msg.Prefix.Name)
		channelCf := s.Casemap(msg.Params[0])

		if nickCf == s.nickCf {
			if c, ok := s.channels[channelCf]; ok {
				delete(s.channels, channelCf)
				for u := range c.Members {
					s.cleanUser(u)
				}
				s.evts <- SelfPartEvent{Channel: c.Name}
			}
		} else if c, ok := s.channels[channelCf]; ok {
			if u, ok := s.users[nickCf]; ok {
				delete(c.Members, u)
				s.cleanUser(u)
				s.typings.Done(channelCf, nickCf)

				s.evts <- UserPartEvent{
					User:    msg.Prefix.Copy(),
					Channel: c.Name,
					Time:    msg.TimeOrNow(),
				}
			}
		}
	case "KICK":
		channelCf := s.Casemap(msg.Params[0])
		nickCf := s.Casemap(msg.Params[1])

		if nickCf == s.nickCf {
			if c, ok := s.channels[channelCf]; ok {
				delete(s.channels, channelCf)
				for u := range c.Members {
					s.cleanUser(u)
				}
				s.evts <- SelfPartEvent{Channel: c.Name}
			}
		} else if c, ok := s.channels[channelCf]; ok {
			if u, ok := s.users[nickCf]; ok {
				delete(c.Members, u)
				s.cleanUser(u)
				s.typings.Done(channelCf, nickCf)

				s.evts <- UserPartEvent{
					User:    u.Name.Copy(),
					Channel: c.Name,
					Time:    msg.TimeOrNow(),
				}
			}
		}
	case "QUIT":
		nickCf := s.Casemap(msg.Prefix.Name)

		if u, ok := s.users[nickCf]; ok {
			var channels []string
			for channelCf, c := range s.channels {
				if _, ok := c.Members[u]; ok {
					channels = append(channels, c.Name)
					delete(c.Members, u)
					s.cleanUser(u)
					s.typings.Done(channelCf, nickCf)
				}
			}

			s.evts <- UserQuitEvent{
				User:     msg.Prefix.Copy(),
				Channels: channels,
				Time:     msg.TimeOrNow(),
			}
		}
	case rplNamreply:
		channelCf := s.Casemap(msg.Params[2])

		if c, ok := s.channels[channelCf]; ok {
			c.Secret = msg.Params[1] == "@"

			// TODO compute CHANTYPES
			for _, name := range ParseNameReply(msg.Params[3], "~&@%+") {
				nickCf := s.Casemap(name.Name.Name)

				if _, ok := s.users[nickCf]; !ok {
					s.users[nickCf] = &User{Name: name.Name.Copy()}
				}
				c.Members[s.users[nickCf]] = name.PowerLevel
			}

			s.channels[channelCf] = c
		}
	case rplEndofnames:
		channelCf := s.Casemap(msg.Params[1])
		if c, ok := s.channels[channelCf]; ok && !c.complete {
			c.complete = true
			s.channels[channelCf] = c
			s.evts <- SelfJoinEvent{Channel: c.Name}
		}
	case rplTopic:
		channelCf := s.Casemap(msg.Params[1])
		if c, ok := s.channels[channelCf]; ok {
			c.Topic = msg.Params[2]
			s.channels[channelCf] = c
		}
	case rplTopicwhotime:
		channelCf := s.Casemap(msg.Params[1])
		t, _ := strconv.ParseInt(msg.Params[3], 10, 64)
		if c, ok := s.channels[channelCf]; ok {
			c.TopicWho = ParsePrefix(msg.Params[2])
			c.TopicTime = time.Unix(t, 0)
			s.channels[channelCf] = c
		}
	case rplNotopic:
		channelCf := s.Casemap(msg.Params[1])
		if c, ok := s.channels[channelCf]; ok {
			c.Topic = ""
			s.channels[channelCf] = c
		}
	case "TOPIC":
		channelCf := s.Casemap(msg.Params[0])
		if c, ok := s.channels[channelCf]; ok {
			c.Topic = msg.Params[1]
			c.TopicWho = msg.Prefix.Copy()
			c.TopicTime = msg.TimeOrNow()
			s.channels[channelCf] = c
			s.evts <- TopicChangeEvent{
				User:    msg.Prefix.Copy(),
				Channel: c.Name,
				Topic:   c.Topic,
				Time:    c.TopicTime,
			}
		}
	case "PRIVMSG", "NOTICE":
		s.evts <- s.privmsgToEvent(msg)
	case "TAGMSG":
		nickCf := s.Casemap(msg.Prefix.Name)
		targetCf := s.Casemap(msg.Params[0])

		if nickCf == s.nickCf {
			// TAGMSG from self
			break
		}

		typing := TypingUnspec
		if t, ok := msg.Tags["+typing"]; ok {
			if t == "active" {
				typing = TypingActive
				s.typings.Active(targetCf, nickCf)
			} else if t == "paused" {
				typing = TypingPaused
				s.typings.Active(targetCf, nickCf)
			} else if t == "done" {
				typing = TypingDone
				s.typings.Done(targetCf, nickCf)
			}
		} else {
			break
		}

		ev := TagEvent{
			User:   msg.Prefix.Copy(), // TODO correctly casemap
			Target: msg.Params[0],     // TODO correctly casemap
			Typing: typing,
			Time:   msg.TimeOrNow(),
		}
		if c, ok := s.channels[targetCf]; ok {
			ev.Target = c.Name
			ev.TargetIsChannel = true
		}
		s.evts <- ev
	case "BATCH":
		batchStart := msg.Params[0][0] == '+'
		id := msg.Params[0][1:]

		if batchStart && msg.Params[1] == "chathistory" {
			s.chBatches[id] = HistoryEvent{Target: msg.Params[2]}
		} else if b, ok := s.chBatches[id]; ok {
			s.evts <- b
			delete(s.chBatches, id)
		}
	case "NICK":
		nickCf := s.Casemap(msg.Prefix.Name)
		newNick := msg.Params[0]
		newNickCf := s.Casemap(newNick)
		t := msg.TimeOrNow()

		var u *Prefix
		if formerUser, ok := s.users[nickCf]; ok {
			formerUser.Name.Name = newNick
			delete(s.users, nickCf)
			s.users[newNickCf] = formerUser
			u = formerUser.Name.Copy()
		} else {
			break
		}

		if nickCf == s.nickCf {
			s.evts <- SelfNickEvent{
				FormerNick: s.nick,
				Time:       t,
			}
			s.nick = newNick
			s.nickCf = newNickCf
		} else {
			s.evts <- UserNickEvent{
				User:       u,
				FormerNick: msg.Prefix.Name,
				Time:       t,
			}
		}
	case "FAIL":
		s.evts <- ErrorEvent{
			Severity: SeverityFail,
			Code:     msg.Params[1],
			Message:  msg.Params[len(msg.Params)-1],
		}
	case "WARN":
		s.evts <- ErrorEvent{
			Severity: SeverityWarn,
			Code:     msg.Params[1],
			Message:  msg.Params[len(msg.Params)-1],
		}
	case "NOTE":
		s.evts <- ErrorEvent{
			Severity: SeverityNote,
			Code:     msg.Params[1],
			Message:  msg.Params[len(msg.Params)-1],
		}
	case "PING":
		err = s.send("PONG :%s\r\n", msg.Params[0])
		if err != nil {
			return
		}
	case "ERROR":
		err = errors.New("connection terminated")
		if len(msg.Params) > 0 {
			err = fmt.Errorf("connection terminated: %s", msg.Params[0])
		}
		_ = s.conn.Close()
	default:
		// reply handling
		if ReplySeverity(msg.Command) == SeverityFail {
			s.evts <- ErrorEvent{
				Severity: SeverityFail,
				Code:     msg.Command,
				Message:  msg.Params[len(msg.Params)-1],
			}
		}
	}

	return
}

func (s *Session) privmsgToEvent(msg Message) (ev MessageEvent) {
	targetCf := s.Casemap(msg.Params[0])

	s.typings.Done(targetCf, s.Casemap(msg.Prefix.Name))
	ev = MessageEvent{
		User:    msg.Prefix.Copy(), // TODO correctly casemap
		Target:  msg.Params[0],     // TODO correctly casemap
		Command: msg.Command,
		Content: msg.Params[1],
		Time:    msg.TimeOrNow(),
	}
	if c, ok := s.channels[targetCf]; ok {
		ev.Target = c.Name
		ev.TargetIsChannel = true
	}

	return
}

func (s *Session) cleanUser(parted *User) {
	for _, c := range s.channels {
		if _, ok := c.Members[parted]; ok {
			return
		}
	}
	delete(s.users, s.Casemap(parted.Name.Name))
}

func (s *Session) updateFeatures(features []string) {
	for _, f := range features {
		if f == "" || f == "-" || f == "=" || f == "-=" {
			continue
		}

		var (
			add   bool
			key   string
			value string
		)

		if strings.HasPrefix(f, "-") {
			add = false
			f = f[1:]
		} else {
			add = true
		}

		kv := strings.SplitN(f, "=", 2)
		key = strings.ToUpper(kv[0])
		if len(kv) > 1 {
			value = kv[1]
		}

		if add {
			s.features[key] = value
		} else {
			delete(s.features, key)
		}
	}
}

func (s *Session) send(format string, args ...interface{}) (err error) {
	msg := fmt.Sprintf(format, args...)
	_, err = s.conn.Write([]byte(msg))

	if s.debug {
		for _, line := range strings.Split(msg, "\r\n") {
			if line != "" {
				s.evts <- RawMessageEvent{
					Message:  line,
					Outgoing: true,
				}
			}
		}
	}

	return
}
