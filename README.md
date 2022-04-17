# Fm
A WIP File Manager

![Fm](img/fm-01.jpeg)

## Quick Start
```console
$ go install
$ fm
```

**NOTE:** Make sure `$EDITOR` is set

## Usage
| Key          | Description                                     |
| ------------ | ----------------------------------------------- |
| <kbd>j</kbd> | Move cursor down                                |
| <kbd>k</kbd> | Move cursor up                                  |
| <kbd>g</kbd> | Move cursor to the top                          |
| <kbd>G</kbd> | Move cursor to the bottom                       |
| <kbd>h</kbd> | Enter Parent Directory                          |
| <kbd>l</kbd> | Enter item under cursor                         |
| <kbd>o</kbd> | Open item under cursor with arbitrary program   |
| <kbd>/</kbd> | Search for items                                |
| <kbd>?</kbd> | Search for items backwards                      |
| <kbd>n</kbd> | Find the next match for the previous search     |
| <kbd>N</kbd> | Find the previous match for the previous search |
| <kbd>d</kbd> | Create a directory                              |
| <kbd>f</kbd> | Create a file                                   |
| <kbd>D</kbd> | Delete item under cursor                        |
