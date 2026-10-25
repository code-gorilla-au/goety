package spinner

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/code-gorilla-au/odize"
)

// syncBuffer is safe to read while the spinner goroutine writes to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func TestSpinner(t *testing.T) {
	var out *syncBuffer
	var spin *Spinner

	group := odize.NewGroup(t, nil)

	group.BeforeEach(func() {
		out = &syncBuffer{}
		spin = New(out)
		spin.frameDuration = time.Millisecond
	})

	err := group.
		Test("should not write to the output when the message is updated", func(t *testing.T) {
			spin.Update("scanning table")

			odize.AssertEqual(t, "", out.String())
		}).
		Test("should render the latest message", func(t *testing.T) {
			spin.Update("scanning table")
			spin.render(0)

			odize.AssertEqual(t, "\r\x1b[2K⣾  scanning table", out.String())
		}).
		Test("should redraw the latest message until stopped", func(t *testing.T) {
			spin.Start("starting")
			spin.Update("scanning table")

			waitForOutput(t, out, "scanning table")
			spin.Stop("")

			odize.AssertTrue(t, strings.Contains(out.String(), "scanning table"))
		}).
		Test("should clear the line and print the final message when stopped", func(t *testing.T) {
			spin.Start("starting")
			spin.Stop("dump complete")

			odize.AssertTrue(t, strings.HasSuffix(out.String(), "\r\x1b[2Kdump complete\n"))
		}).
		Test("should stop cleanly when never started", func(t *testing.T) {
			spin.Stop("")

			odize.AssertEqual(t, "\r\x1b[2K", out.String())
		}).
		Run()

	odize.AssertNoError(t, err)
}

func waitForOutput(t *testing.T, out *syncBuffer, want string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), want) {
			return
		}
		time.Sleep(time.Millisecond)
	}

	t.Fatalf("timed out waiting for %q in spinner output %q", want, out.String())
}
