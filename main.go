package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"io/ioutil"
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
		fmt.Println(err)
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

	COLOR_RED  = 31
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
	screen  Screen
	message error

	path   string
	items  []fs.FileInfo
	cursor int
}

func listDir(path string) ([]fs.FileInfo, error) {
	items, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir() != items[j].IsDir() {
			return items[i].IsDir()
		} else {
			return items[i].Name() < items[j].Name()
		}
	})

	return items, nil
}

func fmInit(path string) Fm {
	path, err := filepath.Abs(path)
	handleError(err)

	items, err := listDir(path)
	handleError(err)

	fm := Fm{
		screen: screenInit(),

		path:  path,
		items: items,
	}

	fm.Render()
	return fm
}

func (fm *Fm) Render() {
	fm.screen.Clear()

	fm.screen.Apply(STYLE_NONE, STYLE_BOLD, COLOR_CYAN)
	fmt.Fprint(fm.screen.output, fm.path)

	start := fm.cursor - fm.cursor%(fm.screen.height-2)
	last := min(len(fm.items), fm.screen.height-2+start)

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
	}

	fm.screen.Apply(STYLE_NONE, STYLE_BOLD, COLOR_RED)

	if fm.message != nil {
		fmt.Fprintf(fm.screen.output, "\r\n\x1b[%dH%s", fm.screen.height, fm.message)
		fm.message = nil
	}

	fm.screen.output.Flush()
}

func (fm *Fm) Search(pred string) {
	for i, item := range fm.items {
		if item.Name() == pred {
			fm.cursor = i
			break
		}
	}
}

func (fm *Fm) Back() {
	if fm.path != "/" {
		newPath := filepath.Dir(fm.path)
		items, err := listDir(newPath)

		if err != nil {
			fm.message = err
		} else {
			fm.items = items
			fm.Search(filepath.Base(fm.path))
			fm.path = newPath
		}
	}
}

func (fm *Fm) Enter() {
	if len(fm.items) > 0 && fm.items[fm.cursor].IsDir() {
		newPath := filepath.Join(fm.path, fm.items[fm.cursor].Name())
		items, err := listDir(newPath)

		if err != nil {
			fm.message = err
		} else {
			fm.path = newPath
			fm.items = items
			fm.cursor = 0
		}
	}
}

func main() {
	initPath := "./"
	if len(os.Args) > 1 {
		initPath = os.Args[1]
	}

	fm := fmInit(initPath)
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
		} else if ch == 'h' {
			fm.Back()
		} else if ch == 'l' {
			fm.Enter()
		}

		fm.Render()
	}

	fm.screen.Reset()
}
