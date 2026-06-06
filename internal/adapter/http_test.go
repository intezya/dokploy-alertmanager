package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeSender struct {
	alerts []AlertmanagerAlert
	err    error
}

func (f *fakeSender) SendAlerts(_ context.Context, alerts []AlertmanagerAlert) error {
	f.alerts = append(f.alerts, alerts...)
	return f.err
}

func TestHandleDokployRequiresToken(t *testing.T) {
	sender := &fakeSender{}
	handler := NewHandler(HandlerConfig{
		Client: sender,
		Token:  "secret",
		Now:    func() time.Time { return time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC) },
	})

	req := httptest.NewRequest(http.MethodPost, "/dokploy", strings.NewReader(`{"event":"appDeploy"}`))
	res := httptest.NewRecorder()

	handler.HandleDokploy(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.Code)
	}
	if len(sender.alerts) != 0 {
		t.Fatalf("sent %d alerts", len(sender.alerts))
	}
}

func TestHandleDokployForwardsAlert(t *testing.T) {
	sender := &fakeSender{}
	handler := NewHandler(HandlerConfig{
		Client:    sender,
		Token:     "secret",
		EndsAfter: 5 * time.Minute,
		Now:       func() time.Time { return time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC) },
	})

	req := httptest.NewRequest(http.MethodPost, "/dokploy", strings.NewReader(`{"event":"appBuildError","title":"Build failed","message":"backend failed"}`))
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()

	handler.HandleDokploy(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	if len(sender.alerts) != 1 {
		t.Fatalf("sent %d alerts", len(sender.alerts))
	}
	if got := sender.alerts[0].Labels["alertname"]; got != "DokployAppBuildError" {
		t.Fatalf("alertname = %q", got)
	}
}

func TestAlertmanagerClientPostsAlerts(t *testing.T) {
	var posted []AlertmanagerAlert
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/alerts" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAlertmanagerClient(server.URL, time.Second)
	err := client.SendAlerts(context.Background(), []AlertmanagerAlert{{
		Labels: map[string]string{"alertname": "DokployAppDeploy"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(posted) != 1 {
		t.Fatalf("posted %d alerts", len(posted))
	}
}
