package ui

import "testing"

func assertSplitPoints(t *testing.T, line string, expected []Point) {
	l := Line{Content: line}
	l.computeSplitPoints()

	if len(l.SplitPoints) != len(expected) {
		t.Errorf("%q: expected %d split points got %d", line, len(expected), len(l.SplitPoints))
		return
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
		if e.Split != a.Split {
			t.Errorf("%q, point #%d: expected Split=%t got %t", line, i, e.Split, a.Split)
		}
	}
}

func TestLineSplitPoints(t *testing.T) {
	assertSplitPoints(t, "hello", []Point{
		{X: 5, I: 5, Split: true},
	})
	assertSplitPoints(t, "hello world", []Point{
		{X: 5, I: 5, Split: true},
		{X: 6, I: 6, Split: false},
		{X: 11, I: 11, Split: true},
	})
	assertSplitPoints(t, "lorem ipsum dolor shit amet", []Point{
		{X: 5, I: 5, Split: true},
		{X: 6, I: 6, Split: false},
		{X: 11, I: 11, Split: true},
		{X: 12, I: 12, Split: false},
		{X: 17, I: 17, Split: true},
		{X: 18, I: 18, Split: false},
		{X: 22, I: 22, Split: true},
		{X: 23, I: 23, Split: false},
		{X: 27, I: 27, Split: true},
	})
}

func assertRenderedHeight(t *testing.T, line string, width int, expected int) {
	l := Line{Content: line}
	l.computeSplitPoints()
	l.Invalidate()

	actual := l.RenderedHeight(width)

	if actual != expected {
		t.Errorf("%q with width=%d expected to take %d lines, takes %d", line, width, expected, actual)
	}
}

func TestRenderedHeight(t *testing.T) {
	assertRenderedHeight(t, "0123456789", 1, 10)
	assertRenderedHeight(t, "0123456789", 2, 5)
	assertRenderedHeight(t, "0123456789", 3, 4)
	assertRenderedHeight(t, "0123456789", 4, 3)
	assertRenderedHeight(t, "0123456789", 5, 2)
	assertRenderedHeight(t, "0123456789", 6, 2)
	assertRenderedHeight(t, "0123456789", 7, 2)
	assertRenderedHeight(t, "0123456789", 8, 2)
	assertRenderedHeight(t, "0123456789", 9, 2)
	assertRenderedHeight(t, "0123456789", 10, 1)
	assertRenderedHeight(t, "0123456789", 11, 1)

	// LEN=9, WIDTH=9
	assertRenderedHeight(t, "take care", 1, 8)  // |t|a|k|e|c|a|r|e|
	assertRenderedHeight(t, "take care", 2, 4)  // |ta|ke|ca|re|
	assertRenderedHeight(t, "take care", 3, 3)  // |tak|e c|are|
	assertRenderedHeight(t, "take care", 4, 2)  // |take|care|
	assertRenderedHeight(t, "take care", 5, 2)  // |take |care |
	assertRenderedHeight(t, "take care", 6, 2)  // |take  |care  |
	assertRenderedHeight(t, "take care", 7, 2)  // |take   |care   |
	assertRenderedHeight(t, "take care", 8, 2)  // |take    |care    |
	assertRenderedHeight(t, "take care", 9, 1)  // |take care|
	assertRenderedHeight(t, "take care", 10, 1) // |take care |

	// LEN=10, WIDTH=10
	assertRenderedHeight(t, "take  care", 1, 8)  // |t|a|k|e|c|a|r|e|
	assertRenderedHeight(t, "take  care", 2, 4)  // |ta|ke|ca|re|
	assertRenderedHeight(t, "take  care", 3, 4)  // |tak|e  |car|e  |
	assertRenderedHeight(t, "take  care", 4, 2)  // |take|care|
	assertRenderedHeight(t, "take  care", 5, 2)  // |take |care |
	assertRenderedHeight(t, "take  care", 6, 2)  // |take  |care  |
	assertRenderedHeight(t, "take  care", 7, 2)  // |take   |care   |
	assertRenderedHeight(t, "take  care", 8, 2)  // |take    |care    |
	assertRenderedHeight(t, "take  care", 9, 2)  // |take     |care     |
	assertRenderedHeight(t, "take  care", 10, 1) // |take  care|
	assertRenderedHeight(t, "take  care", 11, 1) // |take  care |

	// LEN=16, WIDTH=16
	assertRenderedHeight(t, "have a good day!", 1, 13) // |h|a|v|e|a|g|o|o|d|d|a|y|!|
	assertRenderedHeight(t, "have a good day!", 2, 7)  // |ha|ve|a |go|od|da|y!|
	assertRenderedHeight(t, "have a good day!", 3, 5)  // |hav|e a|goo|d d|ay!|
	assertRenderedHeight(t, "have a good day!", 4, 4)  // |have|a   |good|day!|
	assertRenderedHeight(t, "have a good day!", 5, 3)  // |have |a good|day! |
	assertRenderedHeight(t, "have a good day!", 6, 3)  // |have a|good  |day!  |
	assertRenderedHeight(t, "have a good day!", 7, 3)  // |have a |good   |day!   |
	assertRenderedHeight(t, "have a good day!", 8, 3)  // |have a  |good    |day!    |
	assertRenderedHeight(t, "have a good day!", 9, 2)  // |have a   |good day!|
	assertRenderedHeight(t, "have a good day!", 10, 2) // |have a    |good day! |
	assertRenderedHeight(t, "have a good day!", 11, 2) // |have a good|day!       |
	assertRenderedHeight(t, "have a good day!", 12, 2) // |have a good |day!        |
	assertRenderedHeight(t, "have a good day!", 13, 2) // |have a good  |day!         |
	assertRenderedHeight(t, "have a good day!", 14, 2) // |have a good   |day!          |
	assertRenderedHeight(t, "have a good day!", 15, 2) // |have a good    |day!           |
	assertRenderedHeight(t, "have a good day!", 16, 1) // |have a good day!|
	assertRenderedHeight(t, "have a good day!", 17, 1) // |have a good day! |
}
