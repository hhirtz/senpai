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

	bufferList  BufferList
	scrollAmt   int
	scrollAtTop bool

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
				Content:    []Line{NewLineNow(homeMessages[0])},
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

func (ui *UI) CurrentBufferOldestTime() (t time.Time) {
	b := ui.bufferList.List[ui.bufferList.Current].Content
	if len(b) == 0 {
		t = time.Now()
	} else {
		t = b[0].Time
	}
	return
}

func (ui *UI) NextBuffer() (ok bool) {
	ok = ui.bufferList.Next()
	if ok {
		ui.scrollAmt = 0
		ui.scrollAtTop = false
		ui.drawBuffer()
		ui.drawStatus()
	}
	return
}

func (ui *UI) PreviousBuffer() (ok bool) {
	ok = ui.bufferList.Previous()
	if ok {
		ui.scrollAmt = 0
		ui.scrollAtTop = false
		ui.drawBuffer()
		ui.drawStatus()
	}
	return
}

func (ui *UI) ScrollUp() {
	if ui.scrollAtTop {
		return
	}

	_, h := ui.screen.Size()
	ui.scrollAmt += h / 2
	ui.drawBuffer()
}

func (ui *UI) ScrollDown() {
	if ui.scrollAmt == 0 {
		return
	}

	_, h := ui.screen.Size()
	ui.scrollAmt -= h / 2
	if ui.scrollAmt < 0 {
		ui.scrollAmt = 0
	}
	ui.scrollAtTop = false

	ui.drawBuffer()
}

func (ui *UI) IsAtTop() bool {
	return ui.scrollAtTop
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
		if 0 < ui.scrollAmt {
			ui.scrollAmt++
		} else {
			ui.drawBuffer()
		}
	}
}

func (ui *UI) AddHistoryLines(buffer string, lines []Line) {
	idx := ui.bufferList.Idx(buffer)
	if idx < 0 {
		return
	}

	ui.bufferList.AddHistoryLines(idx, lines)

	if idx == ui.bufferList.Current {
		ui.scrollAtTop = false
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
	ui.scrollAmt = 0
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
		ui.scrollAtTop = true
		return
	}

	var bold, italic, underline bool
	var colorState int
	var fgColor, bgColor int

	yEnd := h - 2
	y0 := ui.scrollAmt + h - 2

	for i := len(b.Content) - 1; 0 <= i; i-- {
		line := &b.Content[i]

		if y0 < 0 {
			break
		}

		lineHeight := line.RenderedHeight(w)
		y0 -= lineHeight
		if yEnd <= y0 {
			continue
		}

		rs := []rune(line.Content)
		x := 0
		y := y0
		var lastSP Point
		spIdx := 0

		for i, r := range rs {
			if i == line.SplitPoints[spIdx].I {
				lastSP = line.SplitPoints[spIdx]
				spIdx++

				l := line.SplitPoints[spIdx].X - lastSP.X

				if w < l {
				} else if w == l {
					if x == 0 {
						y++
					}
				} else if w < x+l {
					y++
					x = 0
				}
			}
			if !line.SplitPoints[spIdx].Split && x == 0 {
				continue
			}
			if w <= x {
				y++
				x = 0
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
				if w <= x {
					y++
					x = 0
				}
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

		st = tcell.StyleDefault
		bold = false
		italic = false
		underline = false
		colorState = 0
	}

	ui.scrollAtTop = 0 <= y0
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
		color = tcell.ColorPurple
	case 7:
		color = tcell.ColorOrange
	case 8:
		color = tcell.ColorYellow
	case 9:
		color = tcell.ColorLightGreen
	case 10:
		color = tcell.ColorTeal
	case 11:
		color = tcell.ColorFuchsia
	case 12:
		color = tcell.ColorLightBlue
	case 13:
		color = tcell.ColorPink
	case 14:
		color = tcell.ColorGrey
	case 15:
		color = tcell.ColorLightGrey
	case 99:
		color = tcell.ColorDefault
	default:
		color = tcell.Color(code)
	}
	return
}
