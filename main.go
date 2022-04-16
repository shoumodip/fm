package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/term"
)

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

type Screen struct {
	orig   *term.State
	input  []byte
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
		orig:   orig,
		input:  make([]byte, 1),
		output: bufio.NewWriter(os.Stdout),

		width:  width,
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

const (
	STYLE_NONE    = 0
	STYLE_BOLD    = 1
	STYLE_REVERSE = 7

	COLOR_BLUE = 34
	COLOR_CYAN = 36
)

func (screen *Screen) Apply(effects ...int) {
	fmt.Fprint(screen.output, "\x1b[")

	for i, effect := range effects {
		if i > 0 {
			fmt.Fprint(screen.output, ";")
		}

		fmt.Fprint(screen.output, effect)
	}

	fmt.Fprint(screen.output, "m")
}

type Fm struct {
	screen Screen

	path        string
	pathChanged bool

	items  []fs.FileInfo
	cursor int
}

func (fm *Fm) Refresh() {
	if fm.pathChanged {
		var err error

		fm.items, err = ioutil.ReadDir(fm.path)
		handleError(err)

		sort.Slice(fm.items, func(i, j int) bool {
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

	realPath, err := filepath.Abs(fm.path)
	handleError(err)

	fm.screen.Apply(STYLE_NONE, STYLE_BOLD, COLOR_CYAN)
	fmt.Fprint(fm.screen.output, realPath)
	fm.screen.Apply(STYLE_NONE)

	start := fm.cursor - fm.cursor%(fm.screen.height-1)
	last := min(len(fm.items), fm.screen.height-1+start)

	for i := start; i < last; i++ {
		fm.screen.Apply(STYLE_NONE)
		fmt.Fprint(fm.screen.output, "\r\n")

		if i == fm.cursor {
			fm.screen.Apply(STYLE_REVERSE)
		}

		if fm.items[i].IsDir() {
			fm.screen.Apply(STYLE_BOLD, COLOR_BLUE)
		}

		fmt.Fprint(fm.screen.output, fm.items[i].Name())

		if fm.items[i].IsDir() {
			fmt.Fprint(fm.screen.output, "/")
		}
	}

	fm.screen.output.Flush()
}

func main() {
	fm := Fm{
		screen: screenInit(),

		path:        "./",
		pathChanged: true,
	}

	fm.Refresh()
	fm.Render()
	for {
		ch := fm.screen.Input()

		if ch == 'q' {
			break
		} else if ch == 'j' {
			if fm.cursor+1 < len(fm.items) {
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
