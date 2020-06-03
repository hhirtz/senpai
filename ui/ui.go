package ui

import (
	"github.com/gdamore/tcell"
	"github.com/mattn/go-runewidth"
	"sync/atomic"
	"time"
)

type UI struct {
	screen tcell.Screen
	Events chan tcell.Event
	exit   atomic.Value // bool

	bufferList BufferList
	textInput  []rune
	textCursor int
}

func New() (ui *UI, err error) {
	ui = &UI{}

	ui.screen, err = tcell.NewScreen()
	if err != nil {
		return
	}

	err = ui.screen.Init()
	if err != nil {
		return
	}

	_, h := ui.screen.Size()
	ui.screen.Clear()
	ui.screen.ShowCursor(0, h-2)

	ui.Events = make(chan tcell.Event, 128)
	go func() {
		for !ui.ShouldExit() {
			ui.Events <- ui.screen.PollEvent()
		}
	}()

	ui.exit.Store(false)

	ui.bufferList = BufferList{
		List: []Buffer{
			{
				Title:      "home",
				Highlights: 0,
				Content: []Line{{
					Time:    time.Now(),
					Content: homeMessages[0],
				}},
			},
		},
	}

	ui.textInput = []rune{}

	ui.Draw()

	return
}

func (ui *UI) ShouldExit() bool {
	return ui.exit.Load().(bool)
}

func (ui *UI) Exit() {
	ui.exit.Store(true)
}

func (ui *UI) Close() {
	ui.screen.Fini()
}

func (ui *UI) CurrentBuffer() (title string) {
	title = ui.bufferList.List[ui.bufferList.Current].Title
	return
}

func (ui *UI) NextBuffer() {
	ok := ui.bufferList.Next()
	if ok {
		ui.drawBuffer()
		ui.drawStatus()
	}
}

func (ui *UI) PreviousBuffer() {
	ok := ui.bufferList.Previous()
	if ok {
		ui.drawBuffer()
		ui.drawStatus()
	}
}

func (ui *UI) AddBuffer(title string) {
	_, ok := ui.bufferList.Add(title)
	if ok {
		ui.drawStatus()
		ui.drawBuffer() // TODO only invalidate buffer list
	}
}

func (ui *UI) RemoveBuffer(title string) {
	ok := ui.bufferList.Remove(title)
	if ok {
		ui.drawStatus()
		ui.drawBuffer()
	}
}

func (ui *UI) AddLine(buffer string, line string, t time.Time) {
	idx := ui.bufferList.Idx(buffer)
	if idx < 0 {
		return
	}

	ui.bufferList.AddLine(idx, line, t)

	if idx == ui.bufferList.Current {
		ui.drawBuffer()
	}
}

func (ui *UI) Input() string {
	return string(ui.textInput)
}

func (ui *UI) InputLen() int {
	return len(ui.textInput)
}

func (ui *UI) InputRune(r rune) {
	ui.textInput = append(ui.textInput, r)
	ui.textCursor++
	ui.drawEditor()
}

func (ui *UI) InputRight() {
	if ui.textCursor < len(ui.textInput) {
		ui.textCursor++
		ui.drawEditor()
	}
}

func (ui *UI) InputLeft() {
	if 0 < ui.textCursor {
		ui.textCursor--
		ui.drawEditor()
	}
}

func (ui *UI) InputBackspace() (ok bool) {
	ok = 0 < len(ui.textInput)

	if ok {
		ui.textInput = ui.textInput[:len(ui.textInput)-1]
		if len(ui.textInput) < ui.textCursor {
			ui.textCursor = len(ui.textInput)
		}
		ui.drawEditor()
	}

	return
}

func (ui *UI) InputEnter() (content string) {
	content = string(ui.textInput)

	ui.textInput = []rune{}
	ui.textCursor = 0
	ui.drawEditor()

	return
}

func (ui *UI) Draw() {
	ui.drawStatus()
	ui.drawEditor()
	ui.drawBuffer()
}

func (ui *UI) drawEditor() {
	st := tcell.StyleDefault
	w, h := ui.screen.Size()
	if w == 0 {
		return
	}

	s := string(ui.textInput)
	sw := runewidth.StringWidth(s)

	x := 0
	y := h - 2
	i := 0

	for ; w < sw+1 && i < len(ui.textInput); i++ {
		r := ui.textInput[i]
		rw := runewidth.RuneWidth(r)
		sw -= rw
	}

	for ; i < len(ui.textInput); i++ {
		r := ui.textInput[i]
		ui.screen.SetContent(x, y, r, nil, st)
		x += runewidth.RuneWidth(r)
	}

	for ; x < w; x++ {
		ui.screen.SetContent(x, y, ' ', nil, st)
	}

	ui.screen.ShowCursor(ui.textCursor, y)
	ui.screen.Show()
}

func (ui *UI) drawBuffer() {
	st := tcell.StyleDefault
	w, h := ui.screen.Size()
	if h < 2 {
		return
	}

	for x := 0; x < w; x++ {
		for y := 0; y < h-2; y++ {
			ui.screen.SetContent(x, y, ' ', nil, st)
		}
	}

	b := ui.bufferList.List[ui.bufferList.Current]

	if len(b.Content) == 0 {
		return
	}

	y := h - 2
	start := len(b.Content) - 1
	for {
		lw := runewidth.StringWidth(b.Content[start].Content)

		if y-(lw/w+1) < 0 {
			break
		}

		y -= lw/w + 1

		if start <= 0 {
			break
		}
		start--
	}
	if start > 0 {
		start++
	}

	var bold, italic, underline bool
	var colorState int
	var fgColor, bgColor int

	var lastHour, lastMinute int

	for _, line := range b.Content[start:] {
		rs := []rune(line.Content)
		x := 0

		hour := line.Time.Hour()
		minute := line.Time.Minute()

		if hour != lastHour || minute != lastMinute {
			t := []rune{rune(hour/10) + '0', rune(hour%10) + '0', ':', rune(minute/10) + '0', rune(minute%10) + '0', ' '}
			tst := tcell.StyleDefault.Bold(true)

			for _, r := range t {
				if w <= x {
					y++
					x = 0
				}
				if h-2 <= y {
					break
				}

				ui.screen.SetContent(x, y, r, nil, tst)
				x += runewidth.RuneWidth(r)
			}

			lastHour = hour
			lastMinute = minute
		}

		for _, r := range rs {
			if w <= x {
				y++
				x = 0
			}
			if h-2 <= y {
				break
			}

			if colorState == 1 {
				fgColor = 0
				bgColor = 0
				if '0' <= r && r <= '9' {
					fgColor = fgColor*10 + int(r-'0')
					colorState = 2
					continue
				}
				st = st.Foreground(tcell.ColorDefault)
				st = st.Background(tcell.ColorDefault)
				colorState = 0
			} else if colorState == 2 {
				if '0' <= r && r <= '9' {
					fgColor = fgColor*10 + int(r-'0')
					colorState = 3
					continue
				}
				if r == ',' {
					colorState = 4
					continue
				}
				c := colorFromCode(fgColor)
				st = st.Foreground(c)
				colorState = 0
			} else if colorState == 3 {
				if r == ',' {
					colorState = 4
					continue
				}
				c := colorFromCode(fgColor)
				st = st.Foreground(c)
				colorState = 0
			} else if colorState == 4 {
				if '0' <= r && r <= '9' {
					bgColor = bgColor*10 + int(r-'0')
					colorState = 5
					continue
				}

				c := colorFromCode(fgColor)
				st = st.Foreground(c)
				colorState = 0

				ui.screen.SetContent(x, y, ',', nil, st)
				x++
			} else if colorState == 5 {
				if '0' <= r && r <= '9' {
					bgColor = bgColor*10 + int(r-'0')
					colorState = 6
					continue
				}

				st = st.Foreground(colorFromCode(fgColor))
				st = st.Background(colorFromCode(bgColor))
				colorState = 0
			} else if colorState == 6 {
				st = st.Foreground(colorFromCode(fgColor))
				st = st.Background(colorFromCode(bgColor))
				colorState = 0
			}

			if r == 0x00 {
				bold = false
				italic = false
				underline = false
				st = st.Normal()
				continue
			}
			if r == 0x02 {
				bold = !bold
				st = st.Bold(bold)
				continue
			}
			if r == 0x03 {
				colorState = 1
				continue
			}
			if r == 0x1D {
				italic = !italic
				//st = st.Italic(italic)
				continue
			}
			if r == 0x1F {
				underline = !underline
				//st = st.Underline(underline)
				continue
			}

			ui.screen.SetContent(x, y, r, nil, st)
			x += runewidth.RuneWidth(r)
		}

		y++
		st = tcell.StyleDefault
		bold = false
		italic = false
		underline = false
		colorState = 0
		if h-2 <= y {
			break
		}
	}

	ui.screen.Show()
}

func (ui *UI) drawStatus() {
	st := tcell.StyleDefault
	w, h := ui.screen.Size()

	x := 0
	y := h - 1

	l := 0
	start := 0
	for _, b := range ui.bufferList.List {
		if 0 < b.Highlights {
			l += 3 // TODO
		}

		sw := runewidth.StringWidth(b.Title) + 1
		l += sw
	}

	if w < l && ui.bufferList.Current > 0 {
		start = ui.bufferList.Current - 1
	}

	for i := start; i < len(ui.bufferList.List); i++ {
		b := ui.bufferList.List[i]

		if i == ui.bufferList.Current {
			st = st.Underline(true)
		}
		if 0 < b.Highlights {
			st = st.Bold(true)
		}

		rs := []rune(b.Title)
		for _, r := range rs {
			if w <= x {
				break
			}

			ui.screen.SetContent(x, y, r, nil, st)
			x += runewidth.RuneWidth(r)
		}

		if w+1 <= x {
			break
		}

		st = st.Normal()

		ui.screen.SetContent(x, y, ' ', nil, st)
		x++
	}

	for ; x < w; x++ {
		ui.screen.SetContent(x, y, ' ', nil, st)
	}

	ui.screen.Show()
}

func colorFromCode(code int) (color tcell.Color) {
	switch code {
	case 0:
		color = tcell.ColorWhite
	case 1:
		color = tcell.ColorBlack
	case 2:
		color = tcell.ColorBlue
	case 3:
		color = tcell.ColorGreen
	case 4:
		color = tcell.ColorRed
	case 5:
		color = tcell.ColorBrown
	case 6:
		color = tcell.ColorFuchsia
	case 7:
		color = tcell.ColorOrange
	case 8:
		color = tcell.ColorYellow
	case 9:
		color = tcell.ColorLime
	case 10:
		color = tcell.ColorAqua
	case 11:
		color = tcell.ColorLightCyan
	case 12:
		color = tcell.ColorLightBlue
	case 13:
		color = tcell.ColorPink
	case 14:
		color = tcell.ColorGrey
	case 15:
		color = tcell.ColorLightGrey
	default:
		color = tcell.Color(code)
	}

	return
}
