package main

import (
	"os"
	"fmt"
	"log"
	"sort"
	"io/fs"
	"io/ioutil"
	"bufio"

	"golang.org/x/term"
)

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

type Screen struct {
	orig *term.State
	input []byte
	output *bufio.Writer

	width, height int 
}

func screenInit() Screen {
	orig, err := term.MakeRaw(0)
	handleError(err)

	width, height, err := term.GetSize(0)
	handleError(err)

	fmt.Fprint(os.Stdout, "\x1b[?25l")
	return Screen{
		orig: orig,
		input: make([]byte, 1),
		output: bufio.NewWriter(os.Stdout),

		width: width,
		height: height,
	}
}

func (screen *Screen) Clear() {
	fmt.Fprint(screen.output, "\x1b[2J\x1b[H")
}

func (screen *Screen) Reset() {
	screen.Clear()
	fmt.Fprint(screen.output, "\x1b[?25h")
	screen.output.Flush()

	term.Restore(0, screen.orig)
}

func (screen *Screen) Input() byte {
	_, err := os.Stdin.Read(screen.input)
	handleError(err)
	return screen.input[0]
}

type Fm struct {
	screen Screen

	path string
	pathChanged bool

	items []fs.FileInfo
	cursor int
}

func (fm *Fm) Refresh() {
	if fm.pathChanged {
		var err error

		fm.items, err = ioutil.ReadDir(fm.path)
		handleError(err)

		sort.Slice(fm.items, func (i, j int) bool {
			if fm.items[i].IsDir() != fm.items[j].IsDir() {
				return fm.items[i].IsDir()
			} else {
				return fm.items[i].Name() < fm.items[j].Name()
			}
		})

		fm.cursor = 0
		fm.pathChanged = false
	}
}

func (fm *Fm) Render() {
	fm.screen.Clear()

	for i, item := range fm.items {
		if i == fm.cursor {
			fmt.Fprint(fm.screen.output, "\x1b[7m")
		}

		if item.IsDir() {
			fmt.Fprint(fm.screen.output, "\x1b[1;34m")
		}

		fmt.Fprint(fm.screen.output, item.Name())

		if item.IsDir() {
			fmt.Fprint(fm.screen.output, "/")
		}

		fmt.Fprint(fm.screen.output, "\x1b[0m\r\n")
	}

	fm.screen.output.Flush()
}

func main() {
	fm := Fm{
		screen: screenInit(),

		path: "./",
		pathChanged: true,
	}

	fm.Refresh()
	fm.Render()
	for {
		ch := fm.screen.Input()

		if ch == 'q' {
			break
		} else if ch == 'j' {
			if fm.cursor + 1 < len(fm.items) {
				fm.cursor++
			}
		} else if ch == 'k' {
			if fm.cursor > 0 {
				fm.cursor--
			}
		}

		fm.Render()
	}

	fm.screen.Reset()
}
