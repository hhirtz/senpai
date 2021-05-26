package senpai

import (
	"strings"

	"git.sr.ht/~taiite/senpai/irc"
	"git.sr.ht/~taiite/senpai/ui"
)

func (app *App) completionsChannelMembers(cs []ui.Completion, cursorIdx int, text []rune, s *irc.Session) []ui.Completion {
	var start int
	for start = cursorIdx - 1; 0 <= start; start-- {
		if text[start] == ' ' {
			break
		}
	}
	start++
	word := text[start:cursorIdx]
	if len(word) == 0 {
		return cs
	}
	wordCf := s.Casemap(string(word))
	_, buffer := app.win.CurrentBuffer()
	for _, name := range s.Names(buffer) {
		if strings.HasPrefix(s.Casemap(name.Name.Name), wordCf) {
			nickComp := []rune(name.Name.Name)
			if start == 0 {
				nickComp = append(nickComp, ':')
			}
			nickComp = append(nickComp, ' ')
			c := make([]rune, len(text)+len(nickComp)-len(word))
			copy(c[:start], text[:start])
			if cursorIdx < len(text) {
				copy(c[start+len(nickComp):], text[cursorIdx:])
			}
			copy(c[start:], nickComp)
			cs = append(cs, ui.Completion{
				Text:      c,
				CursorIdx: start + len(nickComp),
			})
		}
	}
	return cs
}

func (app *App) completionsChannelTopic(cs []ui.Completion, cursorIdx int, text []rune, s *irc.Session) []ui.Completion {
	if !hasPrefix(text, []rune("/topic ")) {
		return cs
	}
	_, buffer := app.win.CurrentBuffer()
	topic, _, _ := s.Topic(buffer)
	if cursorIdx == len(text) {
		compText := append(text, []rune(topic)...)
		cs = append(cs, ui.Completion{
			Text:      compText,
			CursorIdx: len(compText),
		})
	}
	return cs
}

func (app *App) completionsMsg(cs []ui.Completion, cursorIdx int, text []rune, s *irc.Session) []ui.Completion {
	if !hasPrefix(text, []rune("/msg ")) {
		return cs
	}
	// Check if the first word (target) is already written and complete (in
	// which case we don't have completions to provide).
	var word string
	hasMetALetter := false
	for i := 5; i < cursorIdx; i += 1 {
		if hasMetALetter && text[i] == ' ' {
			return cs
		}
		if !hasMetALetter && text[i] != ' ' {
			word = s.Casemap(string(text[i:cursorIdx]))
			hasMetALetter = true
		}
	}
	if word == "" {
		return cs
	}
	for _, user := range s.Users() {
		if strings.HasPrefix(s.Casemap(user), word) {
			nickComp := append([]rune(user), ' ')
			c := make([]rune, len(text)+5+len(nickComp)-cursorIdx)
			copy(c[:5], []rune("/msg "))
			copy(c[5:], nickComp)
			if cursorIdx < len(text) {
				copy(c[5+len(nickComp):], text[cursorIdx:])
			}
			cs = append(cs, ui.Completion{
				Text:      c,
				CursorIdx: 5 + len(nickComp),
			})
		}
	}
	return cs
}

func hasPrefix(s, prefix []rune) bool {
	return len(prefix) <= len(s) && equal(prefix, s[:len(prefix)])
}

func equal(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
