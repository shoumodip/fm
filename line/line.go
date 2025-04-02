package line

import (
	"slices"
	"unicode"
)

type Line struct {
	buffer []byte
	Cursor int
}

func New(s string) Line {
	return Line{
		buffer: []byte(s),
		Cursor: len(s),
	}
}

func (l *Line) Insert(ch byte) {
	l.buffer = slices.Insert(l.buffer, l.Cursor, ch)
	l.Cursor++
}

func (l *Line) Start() {
	l.Cursor = 0
}

func (l *Line) End() {
	l.Cursor = len(l.buffer)
}

func (l *Line) PrevChar() {
	if l.Cursor > 0 {
		l.Cursor--
	}
}

func (l *Line) NextChar() {
	if l.Cursor < len(l.buffer) {
		l.Cursor++
	}
}

func isWord(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
}

func (l *Line) PrevWord() {
	if l.Cursor == 0 {
		return
	}

	for l.Cursor > 0 && !isWord(rune(l.buffer[l.Cursor-1])) {
		l.Cursor--
	}

	for l.Cursor > 0 && isWord(rune(l.buffer[l.Cursor-1])) {
		l.Cursor--
	}
}

func (l *Line) NextWord() {
	if l.Cursor >= len(l.buffer) {
		return
	}

	for l.Cursor < len(l.buffer) && !isWord(rune(l.buffer[l.Cursor])) {
		l.Cursor++
	}

	for l.Cursor < len(l.buffer) && isWord(rune(l.buffer[l.Cursor])) {
		l.Cursor++
	}
}

func (l *Line) Delete(motion func(*Line)) {
	mark := l.Cursor
	motion(l)

	start, end := mark, l.Cursor
	if start > end {
		start, end = end, start
	}

	l.buffer = slices.Delete(l.buffer, start, end)
	l.Cursor = start
}

func (l Line) String() string {
	return string(l.buffer)
}
