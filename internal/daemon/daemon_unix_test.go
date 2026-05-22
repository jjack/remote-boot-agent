//go:build !windows

package daemon

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/jjack/grubstation/internal/homeassistant"
)

func TestUnixSocketPush(t *testing.T) {
	oldPath := SocketPath
	tempDir := t.TempDir()
	SocketPath = filepath.Join(tempDir, "grubstation.sock")
	defer func() { SocketPath = oldPath }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()
	haClient := homeassistant.NewClient(ts.URL, "webhook", nil)

	d := New(Config{ReportBootOptions: true}, Metadata{}, nil, haClient)

	go d.listenUnixSocket(ctx, "test-token")
	time.Sleep(50 * time.Millisecond)

	err := RequestPushViaSocket(ctx)
	if err != nil {
		t.Fatalf("RequestPushViaSocket failed: %v", err)
	}
}

func TestUnixSocket_NoResponse(t *testing.T) {
	oldPath := SocketPath
	tempDir := t.TempDir()
	SocketPath = filepath.Join(tempDir, "grubstation_no_resp.sock")
	defer func() { SocketPath = oldPath }()

	l, err := net.Listen("unix", SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	go func() {
		conn, _ := l.Accept()
		if conn != nil {
			_ = conn.Close()
		}
	}()

	err = RequestPushViaSocket(context.Background())
	if err == nil {
		t.Error("expected error from RequestPushViaSocket on no response")
	}
}

func TestDaemon_Run(t *testing.T) {
	d := New(Config{ReportBootOptions: true}, Metadata{}, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- d.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Send signal
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("daemon did not stop after signal")
	}
}
