package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"

	gc "github.com/vit1251/go-ncursesw"
)

const BRACE_MOVE_COUNT = 10

func handleError(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type Item struct {
	name  string
	path  string
	isDir bool
}

func listDir(path string) ([]Item, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	items := make([]Item, len(entries))
	for index, entry := range entries {
		items[index] = Item{
			name:  entry.Name(),
			path:  filepath.Join(path, entry.Name()),
			isDir: entry.IsDir(),
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].isDir != items[j].isDir {
			return items[i].isDir
		} else {
			return items[i].name < items[j].name
		}
	})

	return items, nil
}

type Fm struct {
	window  *gc.Window
	message error

	path     string
	pathPrev string

	count int // For Vim-esque N-actions

	items   []Item
	cursor  int
	anchor  int
	marked  map[string]struct{}
	history map[string]string

	searchQuery   string
	searchReverse bool
}

const (
	COLOR_DIR = iota + 1
	COLOR_MARK
	COLOR_ERROR
	COLOR_TITLE
)

func windowInit() *gc.Window {
	window, err := gc.Init()
	handleError(err)

	gc.Raw(true)
	gc.Echo(false)
	gc.Cursor(0)
	gc.SetEscDelay(0)
	window.Keypad(true)

	gc.StartColor()
	gc.UseDefaultColors()

	gc.InitPair(COLOR_DIR, gc.C_BLUE, -1)
	gc.InitPair(COLOR_MARK, gc.C_MAGENTA, -1)
	gc.InitPair(COLOR_ERROR, gc.C_RED, -1)
	gc.InitPair(COLOR_TITLE, gc.C_CYAN, -1)

	return window
}

func fmInit(path string) Fm {
	path, err := filepath.Abs(path)
	handleError(err)

	items, err := listDir(path)
	handleError(err)

	fm := Fm{
		window:  windowInit(),
		path:    path,
		items:   items,
		marked:  make(map[string]struct{}),
		history: make(map[string]string),
	}

	fm.Render()
	return fm
}

func (fm *Fm) Render() {
	fm.window.Erase()

	fm.window.AttrOn(gc.A_BOLD)
	fm.window.ColorOn(COLOR_TITLE)
	fm.window.Print(fm.path)
	fm.window.AttrOff(gc.A_BOLD)
	fm.window.ColorOff(COLOR_TITLE)

	height, _ := fm.window.MaxYX()
	rows := height - 2

	if fm.cursor >= fm.anchor+rows {
		fm.anchor = fm.cursor - rows + 1
	}

	if fm.cursor < fm.anchor {
		fm.anchor = fm.cursor
	}

	last := min(len(fm.items), rows+fm.anchor)

	line := 1
	for i := fm.anchor; i < last; i++ {
		if i == fm.cursor {
			fm.window.AttrOn(gc.A_REVERSE)
		}

		if fm.items[i].isDir {
			fm.window.ColorOn(COLOR_DIR)
		}

		fm.window.MovePrint(line, 0, fm.items[i].name)
		line++

		if i == fm.cursor {
			fm.window.AttrOff(gc.A_REVERSE)
		}

		if fm.items[i].isDir {
			fm.window.ColorOff(COLOR_DIR)
		}

		if _, ok := fm.marked[fm.items[i].path]; ok {
			fm.window.AttrOn(gc.A_BOLD)
			fm.window.ColorOn(COLOR_MARK)
			fm.window.Print("*")
			fm.window.AttrOff(gc.A_BOLD)
			fm.window.ColorOff(COLOR_MARK)
		}
	}

	if fm.count != 0 {
		fm.window.MovePrintf(height-1, 0, "%d-", fm.count)
	}

	if fm.message != nil {
		fm.window.AttrOn(gc.A_BOLD)
		fm.window.ColorOn(COLOR_ERROR)
		fm.window.MovePrint(height-1, 0, fm.message)
		fm.window.AttrOff(gc.A_BOLD)
		fm.window.ColorOff(COLOR_ERROR)

		fm.message = nil
	}

	fm.window.Refresh()
}

func (fm *Fm) Confirm(query string) bool {
	query = query + " (y/n): "

	gc.Cursor(1)
	defer gc.Cursor(0)

	for {
		height, _ := fm.window.MaxYX()

		fm.window.AttrOn(gc.A_BOLD)
		fm.window.ColorOn(COLOR_DIR)
		fm.window.MovePrint(height-1, 0, query)
		fm.window.AttrOff(gc.A_BOLD)
		fm.window.ColorOff(COLOR_DIR)
		fm.window.ClearToEOL()

		fm.window.Refresh()

		ch := fm.window.GetChar()
		if ch == 27 || ch == 'n' || ch == 'N' {
			return false
		} else if ch == 'y' || ch == 'Y' {
			return true
		}
	}
}

func (fm *Fm) Prompt(query string, init string, update func(string) bool) (string, bool) {
	gc.Cursor(1)
	defer gc.Cursor(0)

	input := init
	error := false

	for {
		height, _ := fm.window.MaxYX()

		fm.window.AttrOn(gc.A_BOLD)
		fm.window.ColorOn(COLOR_DIR)
		fm.window.MovePrint(height-1, 0, query)
		fm.window.AttrOff(gc.A_BOLD)
		fm.window.ColorOff(COLOR_DIR)
		fm.window.ClearToEOL()

		if error {
			fm.window.AttrOn(gc.A_BOLD | gc.A_REVERSE)
			fm.window.ColorOn(COLOR_ERROR)
		}

		fm.window.Print(input)

		if error {
			fm.window.AttrOff(gc.A_BOLD | gc.A_REVERSE)
			fm.window.ColorOff(COLOR_ERROR)
		}

		fm.window.Refresh()

		ch := fm.window.GetChar()
		if ch == 27 {
			return "", false
		} else if ch == gc.KEY_RETURN {
			return input, true
		} else if ch == gc.KEY_BACKSPACE {
			if len(input) > 0 {
				input = input[:len(input)-1]
			}
		} else if strconv.IsPrint(rune(ch)) {
			input = input + string(byte(ch))
		}

		if update != nil {
			error = !update(input)
			fm.Render()
		}
	}
}

func (fm *Fm) FindExact(pred string) {
	for i, item := range fm.items {
		if item.name == pred {
			fm.cursor = i
			break
		}
	}
}

func (fm *Fm) FindQuery(pred string, from int) bool {
	if pred == "" {
		return false
	}
	pred = strings.ToLower(pred)

	for i := from + 1; i < len(fm.items); i++ {
		if strings.Contains(strings.ToLower(fm.items[i].name), pred) {
			fm.cursor = i
			return true
		}
	}

	for i := 0; i < from; i++ {
		if strings.Contains(strings.ToLower(fm.items[i].name), pred) {
			fm.cursor = i
			return true
		}
	}

	if strings.Contains(strings.ToLower(fm.items[from].name), pred) {
		fm.cursor = from
		return true
	}

	return false
}

func (fm *Fm) FindQueryReverse(pred string, from int) bool {
	if pred == "" {
		return false
	}
	pred = strings.ToLower(pred)

	for i := from - 1; i >= 0; i-- {
		if strings.Contains(strings.ToLower(fm.items[i].name), pred) {
			fm.cursor = i
			return true
		}
	}

	for i := len(fm.items) - 1; i > from; i-- {
		if strings.Contains(strings.ToLower(fm.items[i].name), pred) {
			fm.cursor = i
			return true
		}
	}

	if strings.Contains(strings.ToLower(fm.items[from].name), pred) {
		fm.cursor = from
		return true
	}

	return false
}

func (fm *Fm) HistorySave() {
	if fm.cursor < len(fm.items) {
		fm.history[fm.path] = fm.items[fm.cursor].name
	}
}

func (fm *Fm) HistoryRestore() {
	if save, ok := fm.history[fm.path]; ok {
		fm.FindExact(save)
	}
}

func (fm *Fm) BeginSwitchDir(path string, items []Item) {
	fm.HistorySave()
	fm.pathPrev = fm.path
	fm.path = path
	fm.items = items
	fm.cursor = 0
}

func (fm *Fm) PrevDir() {
	if fm.pathPrev == "" {
		fm.message = errors.New("no previous directory to switch to")
		return
	}

	items, err := listDir(fm.pathPrev)
	if err != nil {
		fm.message = err
		return
	}

	fm.BeginSwitchDir(fm.pathPrev, items)
	fm.HistoryRestore()
}

func (fm *Fm) Home() {
	homePath, err := os.UserHomeDir()
	if err != nil {
		fm.message = err
		return
	}

	items, err := listDir(homePath)
	if err != nil {
		fm.message = err
		return
	}

	fm.BeginSwitchDir(homePath, items)
	fm.HistoryRestore()
}

func (fm *Fm) Back() {
	if fm.path != "/" {
		newPath := filepath.Dir(fm.path)
		items, err := listDir(newPath)

		if err != nil {
			fm.message = err
		} else {
			fm.BeginSwitchDir(newPath, items)
			fm.FindExact(filepath.Base(fm.pathPrev))
		}
	}
}

func (fm *Fm) Enter(program string) {
	if len(fm.items) > 0 {
		if fm.items[fm.cursor].isDir && len(program) == 0 {
			items, err := listDir(fm.items[fm.cursor].path)
			if err != nil {
				fm.message = err
			} else {
				fm.BeginSwitchDir(fm.items[fm.cursor].path, items)
				fm.HistoryRestore()
			}
		} else {
			if len(program) == 0 {
				switch runtime.GOOS {
				case "linux":
					program = "xdg-open"
				case "darwin":
					program = "open"
				default:
					fm.message = errors.New("unsupported platform")
					return
				}
			}
			gc.End()

			cmd := exec.Command(program, fm.items[fm.cursor].path)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			fm.message = cmd.Run()

			fm.window = windowInit()
		}
	}
}

func (fm *Fm) Refresh() {
	items, err := listDir(fm.path)
	handleError(err)
	fm.items = items
}

func (fm *Fm) ToggleMark(index int) {
	item := &fm.items[index]

	if _, ok := fm.marked[item.path]; ok {
		delete(fm.marked, item.path)
	} else {
		fm.marked[item.path] = struct{}{}
	}
}

func (fm *Fm) ToggleAndMoveDown() {
	if len(fm.items) > 0 {
		count := max(1, fm.count)
		for i := 0; i < count; i++ {
			fm.ToggleMark(fm.cursor)
			if fm.cursor+1 < len(fm.items) {
				fm.cursor++
			} else {
				break
			}
		}
	}
}

func copyFile(srcpath, dstpath string) error {
	src, err := os.Open(srcpath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstpath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func main() {
	initPath := flag.String("init-path", "./", "The path to start the application in")
	lastPath := flag.String("last-path", "", "The path of the file to output the last directory location into")
	flag.Parse()

	fm := fmInit(*initPath)
	defer func() {
		gc.End()
		if *lastPath != "" {
			handleError(os.WriteFile(*lastPath, []byte(fm.path), 0644))
		}
	}()

	for {
		ch := fm.window.GetChar()

		switch ch {
		case 'q':
			return

		case 'j':
			fm.cursor += max(1, fm.count)
			if fm.cursor >= len(fm.items) {
				if fm.count == 0 || len(fm.items) == 0 {
					fm.cursor = 0
				} else {
					fm.cursor = len(fm.items) - 1
				}
			}

		case 'k':
			fm.cursor -= max(1, fm.count)
			if fm.cursor < 0 {
				if fm.count != 0 || len(fm.items) == 0 {
					fm.cursor = 0
				} else {
					fm.cursor = len(fm.items) - 1
				}
			}

		case '}':
			if len(fm.items) > 0 {
				if fm.cursor+1 == len(fm.items) {
					fm.cursor = 0
				} else {
					fm.cursor += BRACE_MOVE_COUNT * max(1, fm.count)
					if fm.cursor >= len(fm.items) {
						fm.cursor = len(fm.items) - 1
					}
				}
			}

		case '{':
			if len(fm.items) > 0 {
				if fm.cursor == 0 {
					fm.cursor = len(fm.items) - 1
				} else {
					fm.cursor -= BRACE_MOVE_COUNT * max(1, fm.count)
					if fm.cursor < 0 {
						fm.cursor = 0
					}
				}
			}

		case 'g':
			fm.cursor = 0

		case 'G':
			if len(fm.items) > 0 {
				fm.cursor = len(fm.items) - 1
			}

		case 'h', gc.KEY_BACKSPACE:
			fm.Back()

		case 'l', gc.KEY_RETURN:
			fm.Enter("")

		case 'e':
			fm.Enter(os.Getenv("EDITOR"))

		case '~':
			fm.Home()

		case '-':
			fm.PrevDir()

		case 'o':
			query, ok := fm.Prompt("Open: ", "", nil)
			if ok {
				fm.Enter(query)
			}

		case '/':
			cursor := fm.cursor
			_, ok := fm.Prompt("/", "", func(query string) bool {
				fm.cursor = cursor
				fm.searchQuery = query
				fm.searchReverse = false
				return fm.FindQuery(query, cursor)
			})

			if !ok {
				fm.cursor = cursor
			}

		case '?':
			cursor := fm.cursor
			_, ok := fm.Prompt("?", "", func(query string) bool {
				fm.cursor = cursor
				fm.searchQuery = query
				fm.searchReverse = true
				return fm.FindQueryReverse(query, fm.cursor)
			})

			if !ok {
				fm.cursor = cursor
			}

		case 'n':
			if len(fm.searchQuery) > 0 {
				first := -1
				count := max(1, fm.count)
				for i := 0; i < count; i++ {
					if fm.searchReverse {
						fm.FindQueryReverse(fm.searchQuery, fm.cursor)
					} else {
						fm.FindQuery(fm.searchQuery, fm.cursor)
					}

					if i == 0 {
						first = fm.cursor
					} else if fm.cursor == first {
						n := i + 1
						left := count - n
						count = n + left%i
					}
				}
			}

		case 'N':
			if len(fm.searchQuery) > 0 {
				first := -1
				count := max(1, fm.count)
				for i := 0; i < count; i++ {
					if fm.searchReverse {
						fm.FindQuery(fm.searchQuery, fm.cursor)
					} else {
						fm.FindQueryReverse(fm.searchQuery, fm.cursor)
					}

					if i == 0 {
						first = fm.cursor
					} else if fm.cursor == first {
						n := i + 1
						left := count - n
						count = n + left%i
					}
				}
			}

		case 'd':
			query, ok := fm.Prompt("Create Dir: ", "", nil)
			if ok {
				fm.message = os.MkdirAll(filepath.Join(fm.path, query), 0750)

				if fm.message == nil {
					fm.Refresh()
					fm.FindExact(query)
				}
			}

		case 'f':
			query, ok := fm.Prompt("Create File: ", "", nil)
			if ok {
				file, err := os.OpenFile(filepath.Join(fm.path, query), os.O_RDONLY|os.O_CREATE, 0644)
				fm.message = err

				if fm.message == nil {
					file.Close()
					fm.Refresh()
					fm.FindExact(query)
				}
			}

		case 'x':
			fm.ToggleAndMoveDown()

		case 'X':
			for index := range fm.items {
				fm.ToggleMark(index)
			}

		case 'D':
			deleted := false
			toggleStart := -1

			if len(fm.marked) == 0 && fm.count != 0 {
				toggleStart = fm.cursor
				fm.ToggleAndMoveDown()
				fm.cursor = toggleStart
				fm.Render()
			}

			if len(fm.marked) > 0 {
				var prompt string
				if len(fm.marked) == 1 {
					for item := range fm.marked {
						prefix := fm.path
						if prefix != "/" {
							prefix += "/"
						}
						prompt = "Delete '" + strings.TrimPrefix(item, prefix) + "'"
						break
					}
				} else {
					prompt = "Delete " + strconv.Itoa(len(fm.marked)) + " item(s)"
				}

				if fm.Confirm(prompt) {
					deleted = true
					for item := range fm.marked {
						fm.message = os.RemoveAll(item)
						if fm.message != nil {
							break
						}
					}
				} else if toggleStart != -1 {
					fm.ToggleAndMoveDown()
					fm.cursor = toggleStart
				}
			} else if len(fm.items) > 0 {
				if fm.Confirm("Delete '" + fm.items[fm.cursor].name + "'") {
					deleted = true
					fm.message = os.RemoveAll(fm.items[fm.cursor].path)
				}
			}

			if deleted {
				fm.Refresh()
				fm.cursor = min(fm.cursor, len(fm.items)-1)
				fm.marked = make(map[string]struct{})
			}

		case 'm':
			if len(fm.marked) > 0 {
				if fm.Confirm("Move " + strconv.Itoa(len(fm.marked)) + " item(s)") {
					for item := range fm.marked {
						fm.message = os.Rename(item, filepath.Join(fm.path, filepath.Base(item)))
						if fm.message != nil {
							break
						}
					}

					fm.Refresh()
					fm.marked = make(map[string]struct{})
				}
			}

		case 'c':
			if len(fm.marked) > 0 {
				if fm.Confirm("Copy " + strconv.Itoa(len(fm.marked)) + " item(s)") {
					for item := range fm.marked {
						fm.message = copyFile(item, filepath.Join(fm.path, filepath.Base(item)))
						if fm.message != nil {
							break
						}
					}

					fm.Refresh()
					fm.marked = make(map[string]struct{})
				}
			}

		case 'r':
			if len(fm.items) > 0 {
				finalName, ok := fm.Prompt("Rename: ", fm.items[fm.cursor].name, nil)

				if ok {
					fm.message = os.Rename(
						fm.items[fm.cursor].path,
						filepath.Join(fm.path, finalName),
					)
				}

				fm.Refresh()
				fm.FindExact(finalName)
			}
		}

		if unicode.IsDigit(rune(ch)) {
			fm.count = fm.count*10 + int(ch) - '0'
		} else {
			fm.count = 0
		}

		fm.Render()
	}
}
