package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/mattn/go-isatty"
)

// startSpinner draws an animated braille spinner on stderr until the returned
// stop function is called. The first frame is delayed by 200ms so fast cache
// hits don't flash a spinner that's gone before the user can see it.
// No-op when stderr isn't a TTY.
func startSpinner(msg string) func() {
	if !isatty.IsTerminal(os.Stderr.Fd()) {
		return func() {}
	}

	done := make(chan struct{})
	finished := make(chan struct{})

	go func() {
		defer close(finished)

		select {
		case <-done:
			return
		case <-time.After(200 * time.Millisecond):
		}

		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		i := 0
		started := time.Now()
		for {
			select {
			case <-done:
				fmt.Fprint(os.Stderr, "\r\033[K")
				return
			case <-ticker.C:
				elapsed := time.Since(started)
				suffix := ""
				if elapsed > 5*time.Second {
					suffix = fmt.Sprintf(" (%ds — npx warmup is slow on first run)", int(elapsed.Seconds()))
				}
				fmt.Fprintf(os.Stderr, "\r\033[K%s %s%s", frames[i%len(frames)], msg, suffix)
				i++
			}
		}
	}()

	return func() {
		close(done)
		<-finished
	}
}
