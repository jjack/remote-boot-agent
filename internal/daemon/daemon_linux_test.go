//go:build linux

package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jjack/grubstation/internal/homeassistant"
)

func TestDaemon_FinalPush(t *testing.T) {
	var mu sync.Mutex
	pushCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		pushCalled = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	port := getFreePort(t)
	token := "token"
	haClient := homeassistant.NewClient(ts.URL, "webhook", nil)

	d := New(Config{
		Port:              port,
		APIKey:            token,
		ReportBootOptions: true,
	}, Metadata{}, nil, haClient)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		t.Fatal(err)
	}

	cancel()
	<-done

	mu.Lock()
	if !pushCalled {
		t.Error("final push was not called on linux shutdown")
	}
	mu.Unlock()
}
