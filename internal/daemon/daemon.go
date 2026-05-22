package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/jjack/grubstation/internal/grub"
	"github.com/jjack/grubstation/internal/homeassistant"
)

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Config holds the daemon configuration.
type Config struct {
	Port              int
	ReportBootOptions bool
	APIKey            string
	RetryInterval     time.Duration
	ShutdownDelay     time.Duration

	// Reporting fields
	MACAddress          string
	HostAddress         string
	WolBroadcastAddress string
	WolBroadcastPort    int
}

// Metadata holds system information.
type Metadata struct {
	OS             string `json:"os"`
	Version        string `json:"version"`
	ServiceManager string `json:"service_manager"`
}

// Daemon represents the background service.
type Daemon struct {
	Config          Config
	Metadata        Metadata
	Grub            *grub.Grub
	HAClient        *homeassistant.Client
	ShutdownHandler func() error

	mu sync.Mutex
}

func New(cfg Config, meta Metadata, g *grub.Grub, haClient *homeassistant.Client) *Daemon {
	return &Daemon{
		Config:   cfg,
		Metadata: meta,
		Grub:     g,
		HAClient: haClient,
		ShutdownHandler: func() error {
			return getShutdownCommand().Run()
		},
	}
}

// TriggerUpdate performs a boot options push to Home Assistant.
func (d *Daemon) TriggerUpdate(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.Config.ReportBootOptions {
		return nil
	}

	if d.HAClient == nil {
		return fmt.Errorf("homeassistant client not configured")
	}

	var bootOptions []string
	if d.Grub != nil {
		var err error
		bootOptions, err = d.Grub.GetBootOptions(ctx)
		if err != nil {
			return fmt.Errorf("failed to get boot options: %w", err)
		}
	}

	slog.Debug("Triggering boot options update to Home Assistant")
	if err := d.HAClient.UpdateBootOptions(ctx, d.Config.MACAddress, d.Config.HostAddress, bootOptions, d.Config.WolBroadcastAddress, d.Config.WolBroadcastPort); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	slog.Debug("Update successful")
	return nil
}

// run contains the core daemon logic.
func (d *Daemon) run(ctx context.Context) error {
	token := d.Config.APIKey
	if token == "" {
		var err error
		token, err = generateToken()
		if err != nil {
			return fmt.Errorf("failed to generate dynamic token: %w", err)
		}
		slog.Info("Using dynamically generated TOFU token")
	} else {
		slog.Info("Using configured API key")
	}

	// 1. Initial Handshake (Register + First Update) - PREREQUISITE
	if d.HAClient != nil {
		backoff := d.Config.RetryInterval
		if backoff == 0 {
			backoff = 5 * time.Second
		}
		maxBackoff := 5 * time.Minute

		slog.Info("Starting initial registration with Home Assistant")
	registrationLoop:
		for {
			if err := d.HAClient.RegisterAgent(ctx, d.Config.MACAddress, d.Config.HostAddress, token, d.Config.Port); err != nil {
				slog.Warn("Initial registration failed, retrying...", "error", err, "retry_in", backoff)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}
			slog.Info("Initial registration successful")
			break registrationLoop
		}

		// Initial update after successful registration
		if err := d.TriggerUpdate(ctx); err != nil {
			slog.Error("Initial update failed", "error", err)
		} else {
			slog.Info("Initial update successful")
		}
	}

	// 2. Start Listeners
	go d.listenUnixSocket(ctx, token)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", d.Config.Port),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/status" {
				status := struct {
					Status string `json:"status"`
					Metadata
				}{
					Status:   "ok",
					Metadata: d.Metadata,
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(status)
				return
			}

			if r.URL.Path == "/shutdown" {
				if r.Method != http.MethodPost {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusMethodNotAllowed)
					_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "Method not allowed"})
					return
				}

				auth := r.Header.Get("Authorization")
				if auth != "Bearer "+token {
					slog.Warn("Unauthorized shutdown request", "remote_addr", r.RemoteAddr)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "Forbidden"})
					return
				}

				slog.Info("Shutdown requested via HTTP")

				// Perform pre-shutdown push (synchronous)
				if err := d.TriggerUpdate(ctx); err != nil {
					slog.Error("Pre-shutdown push failed", "error", err)
				}

				if err := d.performOSShutdown(); err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": err.Error()})
					return
				}

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			http.NotFound(w, r)
		}),
	}

	go func() {
		slog.Info("Starting HTTP listener", "port", d.Config.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
		}
	}()

	slog.Info("Daemon is running and waiting for termination")

	// 3. Finalization logic when context is cancelled
	<-ctx.Done()
	slog.Info("Shutting down daemon...")

	if runtime.GOOS == "linux" {
		slog.Info("Performing final GRUB report push")
		pushCtx, pushCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer pushCancel()
		if err := d.TriggerUpdate(pushCtx); err != nil {
			slog.Error("Final push failed", "error", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func (d *Daemon) performOSShutdown() error {
	if d.ShutdownHandler != nil {
		return d.ShutdownHandler()
	}
	return nil
}
