// Package dockerutil contains small helpers for working with the
// Docker CLI's human-readable output. Currently scoped to pull progress
// rendering; may grow to cover inspect/image-list parsing if/when the
// init flow needs it.
package dockerutil

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
)

// SpinnerFrames is a braille-dot spinner used by RenderPullProgress
// and (by import) by cmd/server's downloadReporter. Exported so
// callers that render their own progress lines can stay visually
// consistent without redefining the rotation.
//
// Braille dots are monospace and render crisply in every terminal the
// server targets (Terminal.app, iTerm2, Windows Terminal, VS Code,
// etc.); an ASCII fallback ("/-\|") was considered and rejected
// because every supported platform has shipped Unicode-capable
// defaults for over a decade.
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SpinnerTick is the repaint interval. 100 ms is fast enough for the
// dot sequence to look alive, slow enough to not flood a serial tty.
const SpinnerTick = 100 * time.Millisecond

// RenderPullProgress collapses Docker's verbose non-TTY pull output
// (one line per layer × five events per layer = a wall of text) into a
// single in-place progress line with an animated spinner:
//
//	⠙ Layers 5/8 (62%)
//
// The spinner ticks independently of event arrival so the line keeps
// animating during a large silent layer download — the exact moment
// the user wonders "is this hung?".
//
// Event parsing:
//
//	"Pulling fs layer"   increments the total layer count
//	"Pull complete"      increments the completed count
//
// The running total adapts as new layers are announced; when Docker
// staggers announcements the display may briefly show e.g. 3/5 then
// 3/8, which is honest reporting of what's actually known at each
// moment rather than a falsified precomputed total.
//
// If the transcript yields zero "Pulling fs layer" lines (image was
// already up to date — common when the caller didn't bother to guard
// with an images-ls check), the function prints nothing and the
// caller's success message takes over the line.
//
// Concurrency: RenderPullProgress is a blocking call that owns `out`
// for the duration of the pull. It spawns one internal producer
// goroutine to drain the scanner; all writes to `out` happen from the
// calling goroutine. Safe to run concurrently with unrelated writers
// to a different io.Writer.
func RenderPullProgress(in io.Reader, out io.Writer) {
	// Producer goroutine: drain the scanner into a buffered channel so
	// the ticker-driven renderer can select between new events and
	// spinner ticks without blocking.
	events := make(chan string, 128)
	go func() {
		defer close(events)
		scanner := bufio.NewScanner(in)
		// Docker can emit long lines on some statuses; bump the buffer
		// from the default 64 KiB to 1 MiB to be safe.
		scanner.Buffer(make([]byte, 1<<16), 1<<20)
		for scanner.Scan() {
			events <- scanner.Text()
		}
	}()

	ticker := time.NewTicker(SpinnerTick)
	defer ticker.Stop()

	var total, pulled, frame int
	repaint := func() {
		if total == 0 {
			return
		}
		pct := 100 * pulled / total
		// Trailing spaces clear any leftover characters from a longer
		// previous line (single-digit → double-digit count transitions).
		fmt.Fprintf(out, "\r  %s Layers %d/%d (%d%%)   ",
			SpinnerFrames[frame%len(SpinnerFrames)], pulled, total, pct)
	}

	for {
		select {
		case line, ok := <-events:
			if !ok {
				if total > 0 {
					// Close out the in-place line with a real newline
					// so whatever prints next doesn't overwrite our
					// final status.
					fmt.Fprintln(out)
				}
				return
			}
			switch {
			case strings.Contains(line, "Pulling fs layer"):
				total++
			case strings.Contains(line, "Pull complete"):
				pulled++
			}
			repaint()
		case <-ticker.C:
			frame++
			repaint()
		}
	}
}
