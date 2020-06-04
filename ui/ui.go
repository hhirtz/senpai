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

	ui.Resize()

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

func (ui *UI) AddLine(buffer string, line string, t time.Time, isStatus bool) {
	idx := ui.bufferList.Idx(buffer)
	if idx < 0 {
		return
	}

	ui.bufferList.AddLine(idx, line, t, isStatus)

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

func (ui *UI) Resize() {
	ui.bufferList.Invalidate()
	ui.draw()
}

func (ui *UI) draw() {
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
	if h < 3 {
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

	var bold, italic, underline bool
	var colorState int
	var fgColor, bgColor int

	y0 := h - 2

	for i := len(b.Content) - 1; 0 <= i; i-- {
		line := &b.Content[i]

		lineHeight := line.RenderedHeight(w)
		y0 -= lineHeight
		y := y0

		rs := []rune(line.Content)
		x := 0

		for _, r := range rs {
			if w <= x {
				y++
				x = 0
			}

			if x == 0 && IsSplitRune(r) {
				continue
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
				colorState = 0
				st = st.Foreground(colorFromCode(fgColor))

				if '0' <= r && r <= '9' {
					bgColor = bgColor*10 + int(r-'0')
					st = st.Background(colorFromCode(bgColor))
					continue
				}

				st = st.Background(colorFromCode(bgColor))
			}

			if r == 0x00 {
				bold = false
				italic = false
				underline = false
				colorState = 0
				st = tcell.StyleDefault
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
				st = st.Underline(underline)
				continue
			}

			if 0 <= y {
				ui.screen.SetContent(x, y, r, nil, st)
			}
			x += runewidth.RuneWidth(r)
		}

		if y0 < 0 {
			break
		}

		st = tcell.StyleDefault
		bold = false
		italic = false
		underline = false
		colorState = 0
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
	colors := [...]int32{15, 0, 1, 2, 12, 4, 5, 6, 14, 10, 3, 11, 9, 13, 8, 7,
		/* 16-27 */ 52, 94, 100, 58, 22, 29, 23, 24, 17, 54, 53, 89,
		/* 28-39 */ 88, 130, 142, 64, 28, 35, 30, 25, 18, 91, 90, 125,
		/* 40-51 */ 124, 166, 184, 106, 34, 49, 37, 33, 19, 129, 127, 161,
		/* 52-63 */ 196, 208, 226, 154, 46, 86, 51, 75, 21, 171, 201, 198,
		/* 64-75 */ 203, 215, 227, 191, 83, 122, 87, 111, 63, 177, 207, 205,
		/* 76-87 */ 217, 223, 229, 193, 157, 158, 159, 153, 147, 183, 219, 212,
		/* 88-98 */ 16, 233, 235, 237, 239, 241, 244, 247, 250, 254, 231, -1}

	if 0 <= code && code < len(colors) {
		color = tcell.Color(colors[code])
	} else {
		color = tcell.ColorDefault
	}

	return
}
