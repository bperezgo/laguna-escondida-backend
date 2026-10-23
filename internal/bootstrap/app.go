package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"laguna-escondida/backend/pkg/eventbus"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type scheduler interface {
	Start() error
	Stop() error
}

// namedScheduler is built in Run, not in NewCloud/NewEdge: creating a gocron scheduler
// already starts its goroutine, and a test that only drives the router must fire no job.
type namedScheduler struct {
	name  string
	build func() (scheduler, error)
}

type subscriber interface {
	Start(ctx context.Context) error
	Close() error
}

type namedSubscriber struct {
	name string
	sub  subscriber
}

type closableHub interface {
	Close()
}

// App is an assembled binary. Building one starts nothing; Run serves it.
type App struct {
	Router *gin.Engine

	logger      *slog.Logger
	metricsPort string
	subscribers []namedSubscriber
	schedulers  []namedScheduler
	sseHubs     []closableHub
	eventBus    *eventbus.GoChannelEventBus
}

// Run starts the subscribers, schedulers and HTTP servers, and shuts them down when ctx is done.
func (a *App) Run(ctx context.Context) error {
	port := os.Getenv("PORT")
	if port == "" {
		return errors.New("PORT is not set")
	}

	defer a.closeEventBus()
	for _, s := range a.subscribers {
		defer a.closeSubscriber(s)
	}
	a.startSubscribers()

	for _, ns := range a.schedulers {
		s, err := ns.build()
		if err != nil {
			return fmt.Errorf("create %s: %w", ns.name, err)
		}
		if err = s.Start(); err != nil {
			return fmt.Errorf("start %s: %w", ns.name, err)
		}
		defer func() {
			if stopErr := s.Stop(); stopErr != nil {
				log.Printf("Failed to stop %s: %v", ns.name, stopErr)
			}
		}()
	}

	// Metrics on a separate internal port so /metrics is never exposed via the public server.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsSrv := &http.Server{
		Addr:              ":" + a.metricsPort,
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("Metrics server starting on port %s", a.metricsPort)
		if errMetrics := metricsSrv.ListenAndServe(); errMetrics != nil && errMetrics != http.ErrServerClosed {
			a.logger.Error("metrics server failed", slog.Any("error", errMetrics))
		}
	}()

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           a.Router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
		log.Println("Shutting down server...")
	case err := <-serveErr:
		runErr = fmt.Errorf("start server: %w", err)
	}

	log.Println("Closing SSE connections...")
	for _, h := range a.sseHubs {
		h.Close()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Metrics server forced to shutdown: %v", err)
	}
	return runErr
}

func (a *App) startSubscribers() {
	for _, s := range a.subscribers {
		a.logger.Info("Starting " + s.name)
		go func() {
			if err := s.sub.Start(context.Background()); err != nil {
				a.logger.Error(s.name+" stopped", slog.Any("error", err))
			} else {
				a.logger.Info(s.name + " completed successfully")
			}
		}()
	}
}

func (a *App) closeSubscriber(s namedSubscriber) {
	if err := s.sub.Close(); err != nil {
		log.Printf("Failed to close %s: %v", s.name, err)
	}
}

func (a *App) closeEventBus() {
	if err := a.eventBus.Close(); err != nil {
		log.Printf("Failed to close event bus: %v", err)
	}
}
