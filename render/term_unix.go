//go:build unix

package render

import (
	"os"

	"golang.org/x/sys/unix"
)

// osTermWidth returns the terminal column count from the stdout tty, or 0 when
// stdout isn't a terminal (pipe, redirect) so TermWidth can fall back.
func osTermWidth() int {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0
	}
	return int(ws.Col)
}
