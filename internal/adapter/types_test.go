package adapter

import (
	"testing"
	"time"
)

func TestBuildAlertMapsDeploymentEvents(t *testing.T) {
	payload := DokployWebhook{
		Title:     "Deployment Success",
		Message:   "Application 'api' deployed successfully",
		Timestamp: "2026-06-06T10:30:00Z",
		Event:     "appDeploy",
		Metadata: map[string]any{
			"applicationId": "app-123",
			"deploymentId":  "deploy-456",
			"status":        "success",
		},
	}

	alert := BuildAlert(payload, time.Date(2026, 6, 6, 10, 31, 0, 0, time.UTC), 5*time.Minute, "https://dokploy.example.com", map[string]string{"env": "prod"})

	if got := alert.Labels["alertname"]; got != "DokployAppDeploy" {
		t.Fatalf("alertname = %q", got)
	}
	if got := alert.Labels["event_group"]; got != "deployments" {
		t.Fatalf("event_group = %q", got)
	}
	if got := alert.Labels["severity"]; got != "info" {
		t.Fatalf("severity = %q", got)
	}
	if got := alert.Labels["deployment_id"]; got != "deploy-456" {
		t.Fatalf("deployment_id = %q", got)
	}
	if got := alert.Labels["env"]; got != "prod" {
		t.Fatalf("env = %q", got)
	}
	if alert.StartsAt.Format(time.RFC3339) != "2026-06-06T10:30:00Z" {
		t.Fatalf("startsAt = %s", alert.StartsAt.Format(time.RFC3339))
	}
}

func TestBuildAlertInfersBuildError(t *testing.T) {
	payload := DokployWebhook{
		Title:   "Build failed",
		Message: "Build failed for backend",
	}

	alert := BuildAlert(payload, time.Date(2026, 6, 6, 10, 31, 0, 0, time.UTC), 5*time.Minute, "", nil)

	if got := alert.Labels["event"]; got != "appBuildError" {
		t.Fatalf("event = %q", got)
	}
	if got := alert.Labels["event_group"]; got != "deployments" {
		t.Fatalf("event_group = %q", got)
	}
	if got := alert.Labels["severity"]; got != "critical" {
		t.Fatalf("severity = %q", got)
	}
}

func TestBuildAlertMapsThreshold(t *testing.T) {
	payload := DokployWebhook{
		Event:   "serverThreshold",
		Title:   "CPU threshold exceeded",
		Message: "CPU usage exceeded threshold",
		Metadata: map[string]any{
			"ServerName": "ru4",
			"Type":       "CPU",
		},
	}

	alert := BuildAlert(payload, time.Date(2026, 6, 6, 10, 31, 0, 0, time.UTC), 5*time.Minute, "", nil)

	if got := alert.Labels["event_group"]; got != "capacity" {
		t.Fatalf("event_group = %q", got)
	}
	if got := alert.Labels["server"]; got != "ru4" {
		t.Fatalf("server = %q", got)
	}
	if got := alert.Labels["threshold_type"]; got != "CPU" {
		t.Fatalf("threshold_type = %q", got)
	}
}
