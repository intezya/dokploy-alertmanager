package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/intezya/dokploy-alertmanager/internal/adapter"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.logLevel,
	}))
	slog.SetDefault(logger)

	client := adapter.NewAlertmanagerClient(cfg.alertmanagerURL, 10*time.Second)
	handler := adapter.NewHandler(adapter.HandlerConfig{
		Client:       client,
		Token:        cfg.webhookToken,
		EndsAfter:    cfg.endsAfter,
		ExternalURL:  cfg.externalURL,
		StaticLabels: cfg.staticLabels,
		Logger:       logger,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", adapter.HealthHandler)
	mux.HandleFunc("GET /readyz", adapter.HealthHandler)
	mux.HandleFunc("POST /dokploy", handler.HandleDokploy)

	server := &http.Server{
		Addr:              cfg.listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting dokploy alertmanager adapter", "listen_addr", cfg.listenAddr)
		errCh <- server.ListenAndServe()
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stopCh:
		logger.Info("shutdown requested", "signal", sig.String())
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}

type config struct {
	listenAddr      string
	alertmanagerURL string
	webhookToken    string
	endsAfter       time.Duration
	externalURL     string
	staticLabels    map[string]string
	logLevel        slog.Level
}

func loadConfig() (config, error) {
	alertmanagerURL := strings.TrimSpace(os.Getenv("ALERTMANAGER_URL"))
	if alertmanagerURL == "" {
		return config{}, errors.New("ALERTMANAGER_URL is required")
	}

	endsAfter := 5 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("ALERT_ENDS_AFTER")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return config{}, err
		}
		endsAfter = parsed
	}

	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	listenAddr := strings.TrimSpace(os.Getenv("LISTEN_ADDR"))
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	return config{
		listenAddr:      listenAddr,
		alertmanagerURL: alertmanagerURL,
		webhookToken:    strings.TrimSpace(os.Getenv("WEBHOOK_TOKEN")),
		endsAfter:       endsAfter,
		externalURL:     strings.TrimSpace(os.Getenv("EXTERNAL_URL")),
		staticLabels:    parseLabels(os.Getenv("STATIC_LABELS")),
		logLevel:        level,
	}, nil
}

func parseLabels(raw string) map[string]string {
	labels := make(map[string]string)
	for _, item := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(item), "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		labels[key] = value
	}
	return labels
}
