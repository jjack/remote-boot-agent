package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jjack/grubstation/internal/grub"
	"github.com/jjack/grubstation/internal/homeassistant"
)

func getFreePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func waitForServer(port int) error {
	for i := 0; i < 20; i++ {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("server at port %d never became ready", port)
}

func getTestClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
}

func TestDaemonStatusEndpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := getFreePort(t)
	cfg := Config{
		Port:   port,
		APIKey: "test-key",
	}
	meta := Metadata{
		OS:             "linux",
		Version:        "1.2.3",
		ServiceManager: "systemd",
	}
	d := New(cfg, meta, nil, nil)

	done := make(chan error, 1)
	go func() {
		done <- d.run(ctx)
	}()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	resp, err := getTestClient().Get(fmt.Sprintf("http://localhost:%d/status", port))
	if err != nil {
		t.Fatalf("failed to call status endpoint: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json, got %s", resp.Header.Get("Content-Type"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var status struct {
		Status string `json:"status"`
		Metadata
	}
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if status.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", status.Status)
	}
	if status.OS != meta.OS {
		t.Errorf("expected OS %q, got %q", meta.OS, status.OS)
	}
	if status.Version != meta.Version {
		t.Errorf("expected Version %q, got %q", meta.Version, status.Version)
	}
	if status.ServiceManager != meta.ServiceManager {
		t.Errorf("expected ServiceManager %q, got %q", meta.ServiceManager, status.ServiceManager)
	}

	cancel()
	<-done
}

func TestDaemon_Shutdown_Unauthorized(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := getFreePort(t)
	token := "secret-token"
	d := New(Config{
		Port:   port,
		APIKey: token,
	}, Metadata{}, nil, nil)

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/shutdown", port), nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err := getTestClient().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}

	cancel()
	<-done
}

func TestDaemon_Run_HandshakeSuccess(t *testing.T) {
	var registerPayload homeassistant.RegistrationPayload
	var updatePayload homeassistant.UpdatePayload

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "webhook123") {
			var p map[string]any
			_ = json.NewDecoder(r.Body).Decode(&p)
			if p["action"] == string(homeassistant.ActionRegisterAction) {
				registerPayload.Action = homeassistant.ActionRegisterAction
				registerPayload.AgentToken = p["agent_token"].(string)
			} else if p["action"] == string(homeassistant.ActionUpdateAction) {
				updatePayload.Action = homeassistant.ActionUpdateAction
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := getFreePort(t)
	token := "secret"
	haClient := homeassistant.NewClient(ts.URL, "webhook123", nil)

	d := New(Config{
		Port:              port,
		APIKey:            token,
		ReportBootOptions: true,
	}, Metadata{}, nil, haClient)

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	// Wait for registration and update
	time.Sleep(100 * time.Millisecond)

	if registerPayload.Action != homeassistant.ActionRegisterAction {
		t.Error("registration not called")
	}
	if registerPayload.AgentToken != token {
		t.Errorf("expected token %s, got %s", token, registerPayload.AgentToken)
	}
	if updatePayload.Action != homeassistant.ActionUpdateAction {
		t.Error("initial update not called")
	}

	cancel()
	<-done
}

func TestDaemon_Shutdown_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	port := getFreePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmdCalled := make(chan bool, 1)
	token := "token"
	haClient := homeassistant.NewClient(ts.URL, "webhook", nil)

	d := New(Config{
		Port:              port,
		APIKey:            token,
		ReportBootOptions: true,
		ShutdownDelay:     time.Millisecond,
	}, Metadata{}, nil, haClient)

	d.ShutdownHandler = func() error {
		cmdCalled <- true
		return nil
	}

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/shutdown", port), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := getTestClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case <-cmdCalled:
		// success
	case <-time.After(2 * time.Second):
		t.Error("shutdown command not called")
	}

	cancel()
	<-done
}

func TestDaemon_PerformOSShutdown_Success(t *testing.T) {
	d := &Daemon{
		ShutdownHandler: func() error {
			return nil
		},
	}
	if err := d.performOSShutdown(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDaemon_PerformOSShutdown_Error(t *testing.T) {
	d := &Daemon{
		ShutdownHandler: func() error {
			return errors.New("fail")
		},
	}
	if err := d.performOSShutdown(); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestDaemon_New_ShutdownHandler(t *testing.T) {
	d := New(Config{}, Metadata{}, nil, nil)
	// We can't safely run the REAL shutdown command in tests.
	// But we can verify that ShutdownHandler is not nil.
	if d.ShutdownHandler == nil {
		t.Error("expected ShutdownHandler to be set by New")
	}
}

func TestDaemon_Shutdown_CommandError(t *testing.T) {
	port := getFreePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	token := "token"
	d := New(Config{
		Port:   port,
		APIKey: token,
	}, Metadata{}, nil, nil)
	d.ShutdownHandler = func() error {
		return errors.New("shutdown error")
	}

	done := make(chan error, 1)
	go func() { done <- d.run(ctx) }()

	if err := waitForServer(port); err != nil {
		cancel()
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/shutdown", port), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := getTestClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	cancel()
	<-done
}

func TestDaemon_TriggerUpdate_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("DisabledReporting", func(t *testing.T) {
		d := &Daemon{Config: Config{ReportBootOptions: false}}
		if err := d.TriggerUpdate(ctx); err != nil {
			t.Errorf("expected no error when reporting is disabled, got %v", err)
		}
	})

	t.Run("MissingHAClient", func(t *testing.T) {
		d := &Daemon{Config: Config{ReportBootOptions: true}}
		if err := d.TriggerUpdate(ctx); err == nil || !strings.Contains(err.Error(), "home assistant client not configured") {
			t.Errorf("expected missing client error, got %v", err)
		}
	})

	t.Run("GrubError", func(t *testing.T) {
		g := grub.NewGrub()
		g.ConfigPath = "/non/existent"
		haClient := homeassistant.NewClient("http://ha", "webhook", nil)
		d := New(Config{ReportBootOptions: true}, Metadata{}, g, haClient)

		if err := d.TriggerUpdate(ctx); err == nil || !strings.Contains(err.Error(), "failed to get boot options") {
			t.Errorf("expected grub error, got %v", err)
		}
	})
}

func TestDaemon_Run_RegistrationCancel(t *testing.T) {
	// Test cancellation during registration retry sleep
	ctx, cancel := context.WithCancel(context.Background())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	haClient := homeassistant.NewClient(ts.URL, "webhook", nil)
	d := New(Config{
		Port:          0,
		RetryInterval: 50 * time.Millisecond,
	}, Metadata{}, nil, haClient)

	done := make(chan error, 1)
	go func() {
		done <- d.run(ctx)
	}()

	// Wait for at least one failed attempt
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("daemon did not stop after registration cancellation")
	}
}

func TestDaemon_ListenAndServe_Error(t *testing.T) {
	// Use an already taken port to trigger ListenAndServe error
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	port := l.Addr().(*net.TCPAddr).Port
	defer func() { _ = l.Close() }()

	d := New(Config{Port: port}, Metadata{}, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// This should log an error but d.run continues until ctx is cancelled
	done := make(chan error, 1)
	go func() {
		done <- d.run(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done
}
