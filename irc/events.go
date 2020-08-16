package irc

import (
	"time"
)

type Event interface{}

type RawMessageEvent struct {
	Message  string
	Outgoing bool
}

type RegisteredEvent struct{}

type SelfNickEvent struct {
	FormerNick string
	NewNick    string
	Time       time.Time
}

type UserNickEvent struct {
	FormerNick string
	NewNick    string
	Time       time.Time
}

type SelfJoinEvent struct {
	Channel string
}

type UserJoinEvent struct {
	Nick    string
	Channel string
	Time    time.Time
}

type SelfPartEvent struct {
	Channel string
}

type UserPartEvent struct {
	Nick     string
	Channels []string
	Time     time.Time
}

type QueryMessageEvent struct {
	Nick    string
	Command string
	Content string
	Time    time.Time
}

type ChannelMessageEvent struct {
	Nick    string
	Channel string
	Command string
	Content string
	Time    time.Time
}

type QueryTagEvent struct {
	Nick   string
	Typing int
	Time   time.Time
}

type ChannelTagEvent struct {
	Nick    string
	Channel string
	Typing  int
	Time    time.Time
}

type HistoryEvent struct {
	Target   string
	Messages []Event
}
