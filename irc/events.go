package irc

import (
	"strings"
	"time"
)

type Event interface{}

type RawMessageEvent struct {
	Message  string
	Outgoing bool
}

type RegisteredEvent struct{}

type UserEvent struct {
	Nick string
	User string
	Host string
}

func (u UserEvent) NickMapped() (nick string) {
	nick = strings.ToLower(u.Nick)
	return
}

type ChannelEvent struct {
	Channel string
}

func (c ChannelEvent) ChannelMapped() (channel string) {
	channel = strings.ToLower(c.Channel)
	return
}

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
	ChannelEvent
}

type UserJoinEvent struct {
	UserEvent
	ChannelEvent
	Time time.Time
}

type SelfPartEvent struct {
	ChannelEvent
}

type UserPartEvent struct {
	UserEvent
	ChannelEvent
	Time time.Time
}

type QueryMessageEvent struct {
	UserEvent
	Command string
	Content string
	Time    time.Time
}

type ChannelMessageEvent struct {
	UserEvent
	ChannelEvent
	Command string
	Content string
	Time    time.Time
}

type QueryTypingEvent struct {
	UserEvent
	State int
	Time  time.Time
}

type ChannelTypingEvent struct {
	UserEvent
	ChannelEvent
	State int
	Time  time.Time
}

type HistoryEvent struct {
	Target   string
	Messages []Event
}
