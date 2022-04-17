package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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

func (screen *Screen) Bottom() {
	fmt.Fprintf(screen.output, "\r\n\x1b[%dH", screen.height)
}

func (screen *Screen) HideCursor() {
	fmt.Fprint(screen.output, "\x1b[?25l")
}

func (screen *Screen) ShowCursor() {
	fmt.Fprint(screen.output, "\x1b[?25h")
}

func (screen *Screen) Clear() {
	fmt.Fprint(screen.output, "\x1b[2J\x1b[H")
}

func (screen *Screen) Reset() {
	screen.Clear()
	screen.ShowCursor()
	screen.Apply(STYLE_NONE)
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

func (screen *Screen) Prompt(query string) (string, bool) {
	screen.ShowCursor()
	defer screen.HideCursor()

	screen.Bottom()
	input := ""
	for {
		fmt.Fprint(screen.output, "\r\x1b[K")

		screen.Apply(STYLE_NONE, STYLE_BOLD, COLOR_BLUE)
		fmt.Fprint(screen.output, query)

		screen.Apply(STYLE_NONE)
		fmt.Fprint(screen.output, input)

		screen.output.Flush()

		ch := screen.Input()
		if ch == byte(27) {
			return "", false
		} else if ch == byte(13) {
			return input, true
		} else if ch == byte(0x7F) {
			if len(input) > 0 {
				input = input[:len(input)-1]
			}
		} else if strconv.IsPrint(rune(ch)) {
			input = input + string(ch)
		}
	}
}

type Fm struct {
	screen  Screen
	message error

	path   string
	items  []fs.FileInfo
	cursor int

	searchQuery   string
	searchReverse bool
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
		fm.screen.Bottom()
		fmt.Fprint(fm.screen.output, fm.message)
		fm.message = nil
	}

	fm.screen.output.Flush()
}

func (fm *Fm) FindExact(pred string) {
	for i, item := range fm.items {
		if item.Name() == pred {
			fm.cursor = i
			break
		}
	}
}

func (fm *Fm) FindQuery(pred string, from int) {
	pred = strings.ToLower(pred)

	for i := from + 1; i < len(fm.items); i++ {
		if strings.Contains(strings.ToLower(fm.items[i].Name()), pred) {
			fm.cursor = i
			return
		}
	}

	for i := 0; i < from; i++ {
		if strings.Contains(strings.ToLower(fm.items[i].Name()), pred) {
			fm.cursor = i
			return
		}
	}
}

func (fm *Fm) FindQueryReverse(pred string, from int) {
	pred = strings.ToLower(pred)

	for i := from - 1; i >= 0; i-- {
		if strings.Contains(strings.ToLower(fm.items[i].Name()), pred) {
			fm.cursor = i
			return
		}
	}

	for i := len(fm.items) - 1; i > from; i-- {
		if strings.Contains(strings.ToLower(fm.items[i].Name()), pred) {
			fm.cursor = i
			return
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
			fm.FindExact(filepath.Base(fm.path))
			fm.path = newPath
		}
	}
}

func (fm *Fm) Enter(program string) {
	if len(fm.items) > 0 {
		itemPath := filepath.Join(fm.path, fm.items[fm.cursor].Name())

		if fm.items[fm.cursor].IsDir() && len(program) == 0 {
			items, err := listDir(itemPath)

			if err != nil {
				fm.message = err
			} else {
				fm.path = itemPath
				fm.items = items
				fm.cursor = 0
			}
		} else {
			fm.screen.Reset()

			if len(program) == 0 {
				program = os.Getenv("EDITOR")
			}

			cmd := exec.Command(program, itemPath)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			fm.message = cmd.Run()

			fm.screen = screenInit()
		}
	}
}

func main() {
	initPath := "./"
	if len(os.Args) > 1 {
		initPath = os.Args[1]
	}

	fm := fmInit(initPath)
	defer fm.screen.Reset()

	for {
		ch := fm.screen.Input()

		switch ch {
		case 'q':
			return

		case 'j':
			if fm.cursor+1 < len(fm.items) {
				fm.cursor++
			}

		case 'k':
			if fm.cursor > 0 {
				fm.cursor--
			}

		case 'h':
			fm.Back()

		case 'l':
			fm.Enter("")

		case 'o':
			query, ok := fm.screen.Prompt("Open: ")
			if ok {
				fm.Enter(query)
			}

		case '/':
			query, ok := fm.screen.Prompt("/")
			if ok {
				fm.searchQuery = query
				fm.searchReverse = false
				fm.FindQuery(query, fm.cursor)
			}

		case '?':
			query, ok := fm.screen.Prompt("?")
			if ok {
				fm.searchQuery = query
				fm.searchReverse = true
				fm.FindQueryReverse(query, fm.cursor)
			}

		case 'n':
			if len(fm.searchQuery) > 0 {
				if fm.searchReverse {
					fm.FindQueryReverse(fm.searchQuery, fm.cursor)
				} else {
					fm.FindQuery(fm.searchQuery, fm.cursor)
				}
			}

		case 'N':
			if len(fm.searchQuery) > 0 {
				if fm.searchReverse {
					fm.FindQuery(fm.searchQuery, fm.cursor)
				} else {
					fm.FindQueryReverse(fm.searchQuery, fm.cursor)
				}
			}

		case 'd':
			query, ok := fm.screen.Prompt("Create Dir: ")
			if ok {
				fm.message = os.MkdirAll(filepath.Join(fm.path, query), 0750)

				if fm.message == nil {
					items, err := listDir(fm.path)
					handleError(err)
					fm.items = items
				}
			}

		case 'f':
			query, ok := fm.screen.Prompt("Create File: ")
			if ok {
				file, err := os.OpenFile(filepath.Join(fm.path, query), os.O_RDONLY | os.O_CREATE, 0644)
				fm.message = err

				if fm.message == nil {
					file.Close()

					items, err := listDir(fm.path)
					handleError(err)
					fm.items = items
				}
			}
		}

		fm.Render()
	}
}
