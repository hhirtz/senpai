package irc

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

func word(s string) (w, rest string) {
	split := strings.SplitN(s, " ", 2)

	if len(split) < 2 {
		w = split[0]
		rest = ""
	} else {
		w = split[0]
		rest = split[1]
	}

	return
}

func tagEscape(c rune) (escape rune) {
	switch c {
	case ':':
		escape = ';'
	case 's':
		escape = ' '
	case 'r':
		escape = '\r'
	case 'n':
		escape = '\n'
	default:
		escape = c
	}

	return
}

func unescapeTagValue(escaped string) (unescaped string) {
	var builder strings.Builder
	builder.Grow(len(escaped))
	escape := false

	for _, c := range escaped {
		if c == '\\' && !escape {
			escape = true
		} else {
			var cpp rune

			if escape {
				cpp = tagEscape(c)
			} else {
				cpp = c
			}

			builder.WriteRune(cpp)
			escape = false
		}
	}

	unescaped = builder.String()
	return
}

func parseTags(s string) (tags map[string]string) {
	s = s[1:]
	tags = map[string]string{}

	for _, item := range strings.Split(s, ";") {
		if item == "" || item == "=" || item == "+" || item == "+=" {
			continue
		}

		kv := strings.SplitN(item, "=", 2)
		if len(kv) < 2 {
			tags[kv[0]] = ""
		} else {
			tags[kv[0]] = unescapeTagValue(kv[1])
		}
	}

	return
}

var (
	errEmptyMessage      = errors.New("empty message")
	errIncompleteMessage = errors.New("message is incomplete")
)

var (
	errEmptyBatchID    = errors.New("empty BATCH ID")
	errNoPrefix        = errors.New("missing prefix")
	errNotEnoughParams = errors.New("not enough params")
	errUnknownCommand  = errors.New("unknown command")
)

type Message struct {
	Tags    map[string]string
	Prefix  string
	Command string
	Params  []string
}

func Tokenize(line string) (msg Message, err error) {
	line = strings.TrimLeft(line, " ")
	if line == "" {
		err = errEmptyMessage
		return
	}

	if line[0] == '@' {
		var tags string

		tags, line = word(line)
		msg.Tags = parseTags(tags)
	}

	line = strings.TrimLeft(line, " ")
	if line == "" {
		err = errIncompleteMessage
		return
	}

	if line[0] == ':' {
		var prefix string

		prefix, line = word(line)
		msg.Prefix = prefix[1:]
	}

	line = strings.TrimLeft(line, " ")
	if line == "" {
		err = errIncompleteMessage
		return
	}

	msg.Command, line = word(line)
	msg.Command = strings.ToUpper(msg.Command)

	msg.Params = make([]string, 0, 15)
	for line != "" {
		if line[0] == ':' {
			msg.Params = append(msg.Params, line[1:])
			break
		}

		var param string
		param, line = word(line)
		msg.Params = append(msg.Params, param)
	}

	return
}

func (msg *Message) Validate() (err error) {
	switch msg.Command {
	case "001":
		if len(msg.Params) < 1 {
			err = errNotEnoughParams
		}
	case "005":
		if len(msg.Params) < 3 {
			err = errNotEnoughParams
		}
	case "352":
		if len(msg.Params) < 8 {
			err = errNotEnoughParams
		}
	case "372":
		if len(msg.Params) < 2 {
			err = errNotEnoughParams
		}
	case "AUTHENTICATE":
		if len(msg.Params) < 1 {
			err = errNotEnoughParams
		}
	case "900":
		if len(msg.Params) < 3 {
			err = errNotEnoughParams
		}
	case "901":
		if len(msg.Params) < 2 {
			err = errNotEnoughParams
		}
	case "CAP":
		if len(msg.Params) < 3 {
			err = errNotEnoughParams
		} else if msg.Params[1] == "LS" {
		} else if msg.Params[1] == "LIST" {
		} else if msg.Params[1] == "ACK" {
		} else if msg.Params[1] == "NAK" {
		} else if msg.Params[1] == "NEW" {
		} else if msg.Params[1] == "DEL" {
		} else {
			err = errUnknownCommand
		}
	case "JOIN":
		if len(msg.Params) < 1 {
			err = errNotEnoughParams
		} else if msg.Prefix == "" {
			err = errNoPrefix
		}
	case "PART":
		if len(msg.Params) < 1 {
			err = errNotEnoughParams
		} else if msg.Prefix == "" {
			err = errNoPrefix
		}
	case "QUIT":
		if msg.Prefix == "" {
			err = errNoPrefix
		}
	case "353":
		if len(msg.Params) < 4 {
			err = errNotEnoughParams
		}
	case "332":
		if len(msg.Params) < 3 {
			err = errNotEnoughParams
		}
	case "PRIVMSG":
		fallthrough
	case "NOTICE":
		if len(msg.Params) < 2 {
			err = errNotEnoughParams
		} else if msg.Prefix == "" {
			err = errNoPrefix
		}
	case "TAGMSG":
		if len(msg.Params) < 1 {
			err = errNotEnoughParams
		} else if msg.Prefix == "" {
			err = errNoPrefix
		}
	case "TOPIC":
		if len(msg.Params) < 2 {
			err = errNotEnoughParams
		}
	case "BATCH":
		if len(msg.Params) < 1 {
			err = errNotEnoughParams
			break
		}
		if len(msg.Params[0]) < 2 {
			err = errEmptyBatchID
			break
		}
		if msg.Params[0][0] == '+' {
			if len(msg.Params) < 2 {
				err = errNotEnoughParams
				break
			}
			if msg.Params[1] == "chathistory" && len(msg.Params) < 3 {
				err = errNotEnoughParams
			}
		} else if msg.Params[0][0] != '-' {
			err = errEmptyBatchID
		}
	case "PING":
		if len(msg.Params) < 1 {
			err = errNotEnoughParams
		}
	case "PONG":
		if len(msg.Params) < 1 {
			err = errNotEnoughParams
		}
	default:
	}
	return
}

func (msg *Message) Time() (t time.Time, ok bool) {
	var tag string
	var year, month, day, hour, minute, second, millis int

	tag, ok = msg.Tags["time"]
	if !ok {
		return
	}

	tag = strings.TrimSuffix(tag, "Z")

	_, err := fmt.Sscanf(tag, "%4d-%2d-%2dT%2d:%2d:%2d.%3d", &year, &month, &day, &hour, &minute, &second, &millis)
	if err != nil || month < 1 || 12 < month {
		ok = false
		return
	}

	t = time.Date(year, time.Month(month), day, hour, minute, second, millis*1e6, time.UTC)
	t = t.Local()

	return
}

func FullMask(s string) (nick, user, host string) {
	if s == "" {
		return
	}

	spl0 := strings.Split(s, "@")
	if 1 < len(spl0) {
		host = spl0[1]
	}

	spl1 := strings.Split(spl0[0], "!")
	if 1 < len(spl1) {
		user = spl1[1]
	}

	nick = spl1[0]

	return
}

type Cap struct {
	Name   string
	Value  string
	Enable bool
}

func TokenizeCaps(caps string) (diff []Cap) {
	for _, c := range strings.Split(caps, " ") {
		if c == "" || c == "-" || c == "=" || c == "-=" {
			continue
		}

		var item Cap

		if strings.HasPrefix(c, "-") {
			item.Enable = false
			c = c[1:]
		} else {
			item.Enable = true
		}

		kv := strings.SplitN(c, "=", 2)
		item.Name = strings.ToLower(kv[0])
		if len(kv) > 1 {
			item.Value = kv[1]
		}

		diff = append(diff, item)
	}

	return
}

type Name struct {
	PowerLevel string
	Nick       string
	User       string
	Host       string
}

func TokenizeNames(trailing string, prefixes string) (names []Name) {
	for _, name := range strings.Split(trailing, " ") {
		if name == "" {
			continue
		}

		var item Name

		mask := strings.TrimLeft(name, prefixes)
		item.Nick, item.User, item.Host = FullMask(mask)
		item.PowerLevel = name[:len(name)-len(mask)]

		names = append(names, item)
	}

	return
}
