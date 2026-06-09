//go:build !unix

package render

// osTermWidth has no portable tty-size syscall here; TermWidth falls back to
// $COLUMNS and then a default.
func osTermWidth() int { return 0 }
