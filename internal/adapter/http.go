package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxBodySize = 1 << 20

type AlertSender interface {
	SendAlerts(ctx context.Context, alerts []AlertmanagerAlert) error
}

type HandlerConfig struct {
	Client       AlertSender
	Token        string
	EndsAfter    time.Duration
	ExternalURL  string
	StaticLabels map[string]string
	Logger       *slog.Logger
	Now          func() time.Time
}

type Handler struct {
	client       AlertSender
	token        string
	endsAfter    time.Duration
	externalURL  string
	staticLabels map[string]string
	logger       *slog.Logger
	now          func() time.Time
}

func NewHandler(cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Handler{
		client:       cfg.Client,
		token:        cfg.Token,
		endsAfter:    cfg.EndsAfter,
		externalURL:  cfg.ExternalURL,
		staticLabels: cfg.StaticLabels,
		logger:       logger,
		now:          now,
	}
}

func (h *Handler) HandleDokploy(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, http.StatusInternalServerError, "alertmanager client is not configured")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !h.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodySize))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	payload, err := DecodeDokployWebhook(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	alert := BuildAlert(payload, h.now(), h.endsAfter, h.externalURL, h.staticLabels)
	if err := h.client.SendAlerts(r.Context(), []AlertmanagerAlert{alert}); err != nil {
		h.logger.Error("failed to send alert", "error", err, "event", alert.Labels["event"])
		writeError(w, http.StatusBadGateway, "failed to send alert")
		return
	}

	h.logger.Info(
		"forwarded dokploy event",
		"event", alert.Labels["event"],
		"event_group", alert.Labels["event_group"],
		"severity", alert.Labels["severity"],
		"project_name", alert.Labels["project_name"],
		"service", alert.Labels["service"],
		"env", alert.Labels["env"],
		"compose_id", alert.Labels["compose_id"],
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

func (h *Handler) authorized(r *http.Request) bool {
	if h.token == "" {
		return true
	}
	if r.Header.Get("X-Webhook-Token") == h.token {
		return true
	}
	auth := r.Header.Get("Authorization")
	return auth == "Bearer "+h.token
}

func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

type AlertmanagerClient struct {
	endpoint string
	client   *http.Client
}

func NewAlertmanagerClient(rawURL string, timeout time.Duration) *AlertmanagerClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &AlertmanagerClient{
		endpoint: alertmanagerEndpoint(rawURL),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *AlertmanagerClient) SendAlerts(ctx context.Context, alerts []AlertmanagerAlert) error {
	data, err := json.Marshal(alerts)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("alertmanager returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func alertmanagerEndpoint(rawURL string) string {
	rawURL = strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if strings.HasSuffix(rawURL, "/api/v2/alerts") {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return rawURL
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/v2/alerts"
	return parsed.String()
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

var ErrNoAlerts = errors.New("no alerts")
