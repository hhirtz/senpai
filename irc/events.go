package irc

import (
	"strings"
	"time"
)

type Event interface{}

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

type SelfJoinEvent struct {
	ChannelEvent
}

type QueryMessageEvent struct {
	UserEvent
	Content string
	Time    time.Time
}

type ChannelMessageEvent struct {
	UserEvent
	ChannelEvent
	Content string
	Time    time.Time
}

type HistoryEvent struct {
	Target   string
	Messages []Event
}
