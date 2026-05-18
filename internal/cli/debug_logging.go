package cli

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sync"
)

// memHandler is a thread-safe slog handler that writes to a buffer.
type memHandler struct {
	mu     *sync.Mutex
	buf    *bytes.Buffer
	parent slog.Handler
}

func newMemHandler(buf *bytes.Buffer, parent slog.Handler) *memHandler {
	return &memHandler{
		mu:     &sync.Mutex{},
		buf:    buf,
		parent: parent,
	}
}

func (h *memHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// We always want to capture DEBUG logs in the buffer,
	// even if the parent handler filters them out.
	return true
}

func (h *memHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	// 1. Log to the in-memory buffer (always DEBUG level)
	// We use a simple text format for the debug dump
	fmt.Fprintf(h.buf, "[%s] %s: %s", r.Time.Format("15:04:05.000"), r.Level, r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.buf, " %s=%v", a.Key, a.Value)
		return true
	})
	fmt.Fprintln(h.buf)
	h.mu.Unlock()

	// 2. Delegate to the parent (which might be the default stdout handler)
	if h.parent.Enabled(ctx, r.Level) {
		return h.parent.Handle(ctx, r)
	}
	return nil
}

func (h *memHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &memHandler{
		mu:     h.mu,
		buf:    h.buf,
		parent: h.parent.WithAttrs(attrs),
	}
}

func (h *memHandler) WithGroup(name string) slog.Handler {
	return &memHandler{
		mu:     h.mu,
		buf:    h.buf,
		parent: h.parent.WithGroup(name),
	}
}

// setupDebugLogging sets up an in-memory logger and returns a function to dump logs on error.
func setupDebugLogging() (dumpFunc func(err error)) {
	var buf bytes.Buffer
	originalHandler := slog.Default().Handler()

	// If the current handler is the default one from the slog package, wrapping it
	// and then calling slog.SetDefault will cause infinite recursion and deadlock
	// because SetDefault redirects the standard log package back to the new default,
	// while the old default handler delegates its work to the standard log package.
	if reflect.TypeOf(originalHandler).String() == "*slog.defaultHandler" {
		originalHandler = slog.NewTextHandler(os.Stderr, nil)
	}

	// We wrap the current handler. This ensures that:
	// 1. The user still sees what they usually see on the console.
	// 2. We capture EVERYTHING (all levels) in our buffer.
	mem := newMemHandler(&buf, originalHandler)
	slog.SetDefault(slog.New(mem))

	return func(err error) {
		// Restore original handler
		slog.SetDefault(slog.New(originalHandler))

		if err == nil {
			return
		}

		// If there was an error, dump the buffer to a temp file
		tmpFile, createErr := os.CreateTemp("", "grubstation-setup-*.log")
		if createErr != nil {
			fmt.Fprintf(os.Stderr, "\nFailed to create debug log file: %v\n", createErr)
			return
		}
		defer func() { _ = tmpFile.Close() }()

		if _, writeErr := tmpFile.Write(buf.Bytes()); writeErr != nil {
			fmt.Fprintf(os.Stderr, "\nFailed to write to debug log file: %v\n", writeErr)
			return
		}

		fmt.Fprintf(os.Stderr, "\nAn error occurred during setup. Detailed debug logs have been saved to:\n%s\n", tmpFile.Name())
	}
}
