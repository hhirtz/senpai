package ui

import "testing"

func assertSplitPoints(t *testing.T, line string, expected []Point) {
	l := Line{Content: line}
	l.computeSplitPoints()

	if len(l.SplitPoints) != len(expected) {
		t.Errorf("%q: expected %d split points got %d", line, len(expected), len(l.SplitPoints))
	}

	for i := 0; i < len(expected); i++ {
		e := expected[i]
		a := l.SplitPoints[i]

		if e.X != a.X {
			t.Errorf("%q, point #%d: expected X=%d got %d", line, i, e.X, a.X)
		}
		if e.I != a.I {
			t.Errorf("%q, point #%d: expected I=%d got %d", line, i, e.I, a.I)
		}
	}
}

func TestLineSplitPoints(t *testing.T) {
	assertSplitPoints(t, "hello", []Point{
		{X: 5, I: 5},
	})
	assertSplitPoints(t, "hello world", []Point{
		{X: 5, I: 5},
		{X: 11, I: 11},
	})
	assertSplitPoints(t, "lorem ipsum dolor shit amet", []Point{
		{X: 5, I: 5},
		{X: 11, I: 11},
		{X: 17, I: 17},
		{X: 22, I: 22},
		{X: 27, I: 27},
	})
}

func assertRenderedHeight(t *testing.T, line string, width int, expected int) {
	l := Line{Content: line}
	l.computeSplitPoints()
	l.Invalidate()

	actual := l.RenderedHeight(width)

	if actual != expected {
		t.Errorf("%q (width=%d) expected to take %d lines, takes %d", line, width, expected, actual)
	}
}

func TestRenderedHeight(t *testing.T) {
	assertRenderedHeight(t, "hello world", 100, 1)
	assertRenderedHeight(t, "hello world", 10, 2)

	assertRenderedHeight(t, "have a good day!", 100, 1)
	assertRenderedHeight(t, "have a good day!", 10, 2)
	assertRenderedHeight(t, "have a good day!", 6, 3)
	assertRenderedHeight(t, "have a good day!", 4, 4)
}
