package irc

import (
	"time"
)

type Event interface{}

type RawMessageEvent struct {
	Message  string
	Outgoing bool
	IsValid  bool
}

type RegisteredEvent struct{}

type SelfNickEvent struct {
	FormerNick string
	Time       time.Time
}

type UserNickEvent struct {
	User       *Prefix
	FormerNick string
	Time       time.Time
}

type SelfJoinEvent struct {
	Channel string
}

type UserJoinEvent struct {
	User    *Prefix
	Channel string
	Time    time.Time
}

type SelfPartEvent struct {
	Channel string
}

type UserPartEvent struct {
	User    *Prefix
	Channel string
	Time    time.Time
}

type UserQuitEvent struct {
	User     *Prefix
	Channels []string
	Time     time.Time
}

type MessageEvent struct {
	User            *Prefix
	Target          string
	TargetIsChannel bool
	Command         string
	Content         string
	Time            time.Time
}

type TagEvent struct {
	User            *Prefix
	Target          string
	TargetIsChannel bool
	Typing          int
	Time            time.Time
}

type HistoryEvent struct {
	Target   string
	Messages []Event
}
