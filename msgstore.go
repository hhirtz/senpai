package senpai

type Buffer struct {
}

type BufferID int

var nextBufferID BufferID = 0

type MsgStore struct {
	buffers map[BufferID]*Buffer
}
