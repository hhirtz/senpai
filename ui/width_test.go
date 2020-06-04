package ui

import "testing"

func assertStringWidth(t *testing.T, input string, expected int) {
	actual := StringWidth(input)
	if actual != expected {
		t.Errorf("%q: expected width of %d got %d", input, expected, actual)
	}
}

func TestStringWidth(t *testing.T) {
	assertStringWidth(t, "", 0)

	assertStringWidth(t, "hello", 5)
	assertStringWidth(t, "\x02hello", 5)
	assertStringWidth(t, "\x035hello", 5)
	assertStringWidth(t, "\x0305hello", 5)
	assertStringWidth(t, "\x0305,0hello", 5)
	assertStringWidth(t, "\x0305,09hello", 5)

	assertStringWidth(t, "\x0305,hello", 6)
	assertStringWidth(t, "\x03050hello", 6)
	assertStringWidth(t, "\x0305,090hello", 6)
}
