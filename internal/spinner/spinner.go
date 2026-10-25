package spinner

import (
	"io"
	"sync"
	"time"
)

const defaultFrameDuration = 150 * time.Millisecond

const (
	clearLineSequence = "\r\x1b[2K"
	messageSeparator  = "  "
)

// Spinner renders a single-line progress indicator to an io.Writer.
//
// Update only stores the latest message, so callers never block on terminal
// I/O. The draw loop reads that message on every tick.
type Spinner struct {
	out           io.Writer
	sprite        []string
	frameDuration time.Duration

	mu      sync.Mutex
	message string

	done     chan struct{}
	stopOnce sync.Once
	finished sync.WaitGroup
}

// New creates a spinner that writes to out. Use io.Discard to disable output
// while keeping the same call sites.
func New(out io.Writer) *Spinner {
	return &Spinner{
		out:           out,
		sprite:        brailleDots,
		frameDuration: defaultFrameDuration,
		done:          make(chan struct{}),
	}
}

// Start begins drawing message and keeps redrawing it in the background.
func (s *Spinner) Start(message string) {
	s.Update(message)

	s.finished.Add(1)
	go s.draw()
}

// Update records the message shown on the next frame. It does not block on
// files, the terminal, or other goroutines.
func (s *Spinner) Update(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.message = message
}

// Stop ends the draw loop, clears the line, and, when message is not empty,
// prints it in place of the spinner. Stop is safe to call more than once.
func (s *Spinner) Stop(message string) {
	s.stopOnce.Do(func() {
		close(s.done)
		s.finished.Wait()

		s.write(clearLineSequence)

		if message != "" {
			s.write(message + "\n")
		}
	})
}

// draw renders frames until Stop closes done.
func (s *Spinner) draw() {
	defer s.finished.Done()

	ticker := time.NewTicker(s.frameDuration)
	defer ticker.Stop()

	for frame := 0; ; frame = (frame + 1) % len(s.sprite) {
		s.render(frame)

		select {
		case <-s.done:
			return
		case <-ticker.C:
		}
	}
}

// render draws one frame followed by the current message, replacing whatever
// was on the line before.
func (s *Spinner) render(frame int) {
	s.mu.Lock()
	message := s.message
	s.mu.Unlock()

	s.write(clearLineSequence + s.sprite[frame] + messageSeparator + message)
}

// write renders text to the output. Write errors are ignored on purpose:
// progress reporting is best-effort and must never fail the running command.
func (s *Spinner) write(text string) {
	_, _ = io.WriteString(s.out, text)
}
