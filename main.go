package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/shoumodip/screen-go"
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

func gotoBottom(s *screen.Screen) {
	s.MoveCursor(0, s.Height-1)
}

func confirmPrompt(s *screen.Screen, query string) bool {
	query = query + " (y/n): "

	s.ShowCursor()
	defer s.HideCursor()

	gotoBottom(s)
	for {
		fmt.Fprint(s, "\r\x1b[K")

		s.Apply(screen.STYLE_NONE, screen.STYLE_BOLD, screen.COLOR_BLUE)
		fmt.Fprint(s, query)

		s.Flush()

		ch, err := s.Input()
		handleError(err)
		if ch == byte(27) || ch == 'n' || ch == 'N' {
			return false
		} else if ch == 'y' || ch == 'Y' {
			return true
		}
	}
}

type Item struct {
	name  string
	path  string
	isDir bool
}

type Fm struct {
	screen  *screen.Screen
	message error

	path   string
	items  []Item
	cursor int
	marked map[string]struct{}

	searchQuery   string
	searchReverse bool
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

func fmInit(path string) Fm {
	path, err := filepath.Abs(path)
	handleError(err)

	items, err := listDir(path)
	handleError(err)

	screen, err := screen.New()
	handleError(err)

	fm := Fm{
		screen: &screen,
		path:   path,
		items:  items,
		marked: make(map[string]struct{}),
	}

	fm.Render()
	return fm
}

func (fm *Fm) Render() {
	fm.screen.Clear()

	fm.screen.Apply(screen.STYLE_NONE, screen.STYLE_BOLD, screen.COLOR_CYAN)
	fmt.Fprint(fm.screen, fm.path)

	start := fm.cursor - fm.cursor%(fm.screen.Height-2)
	last := min(len(fm.items), fm.screen.Height-2+start)

	for i := start; i < last; i++ {
		fm.screen.Apply(screen.STYLE_NONE)
		fmt.Fprint(fm.screen, "\r\n")

		if i == fm.cursor {
			fm.screen.Apply(screen.STYLE_REVERSE)
		}

		if fm.items[i].isDir {
			fm.screen.Apply(screen.STYLE_BOLD, screen.COLOR_BLUE)
		}

		fmt.Fprint(fm.screen, fm.items[i].name)

		if _, ok := fm.marked[fm.items[i].path]; ok {
			fm.screen.Apply(screen.STYLE_NONE, screen.STYLE_BOLD, screen.COLOR_MAGENTA)
			fmt.Fprint(fm.screen, "*")
		}
	}

	fm.screen.Apply(screen.STYLE_NONE, screen.STYLE_BOLD, screen.COLOR_RED)

	if fm.message != nil {
		gotoBottom(fm.screen)
		fmt.Fprint(fm.screen, fm.message)
		fm.message = nil
	}

	fm.screen.Flush()
}

func (fm *Fm) ShowPrompt(query string, init string, update func(string) bool) (string, bool) {
	fm.screen.ShowCursor()
	defer fm.screen.HideCursor()

	input := init
	color := screen.STYLE_NONE
	for {
		gotoBottom(fm.screen)
		fmt.Fprint(fm.screen, "\r\x1b[K")

		fm.screen.Apply(screen.STYLE_NONE, screen.STYLE_BOLD, screen.COLOR_BLUE)
		fmt.Fprint(fm.screen, query)

		fm.screen.Apply(color)
		fmt.Fprint(fm.screen, input)

		fm.screen.Flush()

		ch, err := fm.screen.Input()
		handleError(err)
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

		if update != nil {
			if update(input) {
				color = screen.STYLE_NONE
			} else {
				color = screen.COLOR_RED
			}
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

	return false
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
		if fm.items[fm.cursor].isDir && len(program) == 0 {
			items, err := listDir(fm.items[fm.cursor].path)

			if err != nil {
				fm.message = err
			} else {
				fm.path = fm.items[fm.cursor].path
				fm.items = items
				fm.cursor = 0
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
			fm.screen.Reset()

			cmd := exec.Command(program, fm.items[fm.cursor].path)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			fm.message = cmd.Run()

			screen, err := screen.New()
			handleError(err)

			screen.HideCursor()
			fm.screen = &screen
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
	initPath := "./"
	if len(os.Args) > 1 {
		initPath = os.Args[1]
	}

	fm := fmInit(initPath)
	defer fm.screen.Reset()

	fm.screen.HideCursor()
	fm.screen.Flush()
	for {
		ch, err := fm.screen.Input()
		handleError(err)

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

		case 'g':
			fm.cursor = 0

		case 'G':
			if len(fm.items) > 0 {
				fm.cursor = len(fm.items) - 1
			}

		case 'h', byte(0x7F):
			fm.Back()

		case 'l', byte(13):
			fm.Enter("")

		case 'e':
			fm.Enter(os.Getenv("EDITOR"))

		case 'o':
			query, ok := fm.ShowPrompt("Open: ", "", nil)
			if ok {
				fm.Enter(query)
			}

		case '/':
			cursor := fm.cursor
			_, ok := fm.ShowPrompt("/", "", func(query string) bool {
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
			_, ok := fm.ShowPrompt("?", "", func(query string) bool {
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
			query, ok := fm.ShowPrompt("Create Dir: ", "", nil)
			if ok {
				fm.message = os.MkdirAll(filepath.Join(fm.path, query), 0750)

				if fm.message == nil {
					fm.Refresh()
					fm.FindQuery(query, fm.cursor)
				}
			}

		case 'f':
			query, ok := fm.ShowPrompt("Create File: ", "", nil)
			if ok {
				file, err := os.OpenFile(filepath.Join(fm.path, query), os.O_RDONLY|os.O_CREATE, 0644)
				fm.message = err

				if fm.message == nil {
					file.Close()
					fm.Refresh()
					fm.FindQuery(query, fm.cursor)
				}
			}

		case 'x':
			if len(fm.items) > 0 {
				fm.ToggleMark(fm.cursor)
				if fm.cursor+1 < len(fm.items) {
					fm.cursor++
				}
			}

		case 'X':
			for index := range fm.items {
				fm.ToggleMark(index)
			}

		case 'D':
			deleted := false

			if len(fm.marked) > 0 {
				if confirmPrompt(fm.screen, "Delete "+strconv.Itoa(len(fm.marked))+" item(s)") {
					deleted = true
					for item := range fm.marked {
						fm.message = os.RemoveAll(item)
						if fm.message != nil {
							break
						}
					}
				}
			} else if len(fm.items) > 0 {
				if confirmPrompt(fm.screen, "Delete '"+fm.items[fm.cursor].name+"'") {
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
				if confirmPrompt(fm.screen, "Move "+strconv.Itoa(len(fm.marked))+" item(s)") {
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
				if confirmPrompt(fm.screen, "Copy "+strconv.Itoa(len(fm.marked))+" item(s)") {
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
				finalName, ok := fm.ShowPrompt("Rename: ", fm.items[fm.cursor].name, nil)

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

		fm.Render()
	}
}
