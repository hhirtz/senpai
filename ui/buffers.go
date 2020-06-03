package ui

import "time"

var homeMessages = []string{
	"\x1dYou open an IRC client.",
	"Welcome to the Internet Relay Network!",
	"Mentions & cie go here.",
	"May the IRC be with you.",
	"Hey! I'm senpai, you every IRC student!",
	"Student? No, I'm an IRC \x02client\x02!",
}

type Line struct {
	Time     time.Time
	IsStatus bool
	Content  string
}

type Buffer struct {
	Title      string
	Highlights int
	Content    []Line
}

type BufferList struct {
	List    []Buffer
	Current int
}

func (bs *BufferList) Add(title string) (pos int, ok bool) {
	for i, b := range bs.List {
		if b.Title == title {
			pos = i
			return
		}
	}

	pos = len(bs.List)
	ok = true
	bs.List = append(bs.List, Buffer{Title: title})

	return
}

func (bs *BufferList) Remove(title string) (ok bool) {
	for i, b := range bs.List {
		if b.Title == title {
			ok = true
			bs.List = append(bs.List[:i], bs.List[i+1:]...)

			if i == bs.Current {
				bs.Current = 0
			}

			return
		}
	}

	return
}

func (bs *BufferList) Previous() (ok bool) {
	if bs.Current <= 0 {
		ok = false
	} else {
		bs.Current--
		ok = true
	}

	return
}

func (bs *BufferList) Next() (ok bool) {
	if bs.Current+1 < len(bs.List) {
		bs.Current++
		ok = true
	} else {
		ok = false
	}

	return
}

func (bs *BufferList) Idx(title string) (idx int) {
	if title == "" {
		idx = 0
		return
	}

	for pos, b := range bs.List {
		if b.Title == title {
			idx = pos
			return
		}
	}

	idx = -1
	return
}

func (bs *BufferList) AddLine(idx int, line string, t time.Time, isStatus bool) {
	n := len(bs.List[idx].Content)

	if isStatus && n != 0 && bs.List[idx].Content[n-1].IsStatus {
		bs.List[idx].Content[n-1].Content += " " + line
	} else {
		bs.List[idx].Content = append(bs.List[idx].Content, Line{
			Time:     t,
			IsStatus: isStatus,
			Content:  line,
		})
	}
}
