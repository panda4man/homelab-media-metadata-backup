package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/config"
	"github.com/panda4man/homelab-media-metadata-backup/internal/orchestrator"
)

// Config configures the HTTP API handler built by New. Ctx is the
// server-scoped context handed to RunFn's background goroutine: it must
// outlive any single request, so it's supplied at construction rather
// than taken from the triggering request.
type Config struct {
	Token string

	Ctx      context.Context
	Cfg      config.Config
	RunFn    func(context.Context, config.Config) (orchestrator.Result, error)
	LockPath string
	NewID    func() string
	Now      func() time.Time
	Logger   *slog.Logger
}

const healthPath = "/v1/health"

// New builds the API handler: GET /v1/health is unauthenticated, everything
// else under /v1/ requires the bearer token. Health gets its own mux so a
// wrong-method request there returns 405 instead of being swallowed by the
// authenticated catch-all.
func New(cfg Config) http.Handler {
	health := http.NewServeMux()
	health.HandleFunc("GET "+healthPath, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	api := http.NewServeMux()
	runs := newRunsHandler(cfg)
	api.HandleFunc("POST /v1/runs", runs.create)
	api.HandleFunc("GET /v1/runs/{id}", runs.get)

	authenticated := requireToken(cfg.Token, api)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == healthPath {
			health.ServeHTTP(w, r)
			return
		}
		authenticated.ServeHTTP(w, r)
	})
}

const shutdownGracePeriod = 5 * time.Second

// Serve runs an *http.Server on ln until ctx is cancelled, then shuts it
// down within a bounded grace period. It returns nil on a clean stop rather
// than the http.ErrServerClosed net/http normally reports.
func Serve(ctx context.Context, ln net.Listener, h http.Handler) error {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		<-serveErr
		return nil
	}
}
