package main

import (
	"slices"
	"unicode"
)

type Line struct {
	buffer []byte
	cursor int
}

func NewLine(s string) Line {
	return Line{
		buffer: []byte(s),
		cursor: len(s),
	}
}

func (l *Line) Insert(ch byte) {
	l.buffer = slices.Insert(l.buffer, l.cursor, ch)
	l.cursor++
}

func (l *Line) Start() {
	l.cursor = 0
}

func (l *Line) End() {
	l.cursor = len(l.buffer)
}

func (l *Line) PrevChar() {
	if l.cursor > 0 {
		l.cursor--
	}
}

func (l *Line) NextChar() {
	if l.cursor < len(l.buffer) {
		l.cursor++
	}
}

func isWord(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
}

func (l *Line) PrevWord() {
	if l.cursor == 0 {
		return
	}

	for l.cursor > 0 && !isWord(rune(l.buffer[l.cursor-1])) {
		l.cursor--
	}

	for l.cursor > 0 && isWord(rune(l.buffer[l.cursor-1])) {
		l.cursor--
	}
}

func (l *Line) NextWord() {
	if l.cursor >= len(l.buffer) {
		return
	}

	for l.cursor < len(l.buffer) && !isWord(rune(l.buffer[l.cursor])) {
		l.cursor++
	}

	for l.cursor < len(l.buffer) && isWord(rune(l.buffer[l.cursor])) {
		l.cursor++
	}
}

func (l *Line) Delete(motion func(*Line)) {
	mark := l.cursor
	motion(l)

	start, end := mark, l.cursor
	if start > end {
		start, end = end, start
	}

	l.buffer = slices.Delete(l.buffer, start, end)
	l.cursor = start
}

func (l Line) String() string {
	return string(l.buffer)
}
