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
			"applicationId":   "app-123",
			"applicationName": "api",
			"projectId":       "project-123",
			"projectName":     "paravoz-taxi",
			"deploymentId":    "deploy-456",
			"logUrl":          "https://dokploy.example.com/logs/deploy-456",
			"status":          "success",
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
	if got := alert.Labels["application_name"]; got != "api" {
		t.Fatalf("application_name = %q", got)
	}
	if got := alert.Labels["project_name"]; got != "paravoz-taxi" {
		t.Fatalf("project_name = %q", got)
	}
	if got := alert.Labels["project_id"]; got != "project-123" {
		t.Fatalf("project_id = %q", got)
	}
	if got := alert.Labels["log_url"]; got != "https://dokploy.example.com/logs/deploy-456" {
		t.Fatalf("log_url = %q", got)
	}
	if got := alert.Labels["env"]; got != "prod" {
		t.Fatalf("env = %q", got)
	}
	if alert.StartsAt.Format(time.RFC3339) != "2026-06-06T10:30:00Z" {
		t.Fatalf("startsAt = %s", alert.StartsAt.Format(time.RFC3339))
	}
}

func TestBuildAlertMapsEnvironmentFromPayloadLabels(t *testing.T) {
	payload := DokployWebhook{
		Title: "Build completed successfully",
		Event: "build",
		Labels: map[string]string{
			"env":          "qa",
			"project_name": "paravoz-taxi",
			"service":      "frontend",
		},
	}

	alert := BuildAlert(payload, time.Date(2026, 6, 8, 13, 2, 0, 0, time.UTC), 5*time.Minute, "", nil)

	if got := alert.Labels["env"]; got != "qa" {
		t.Fatalf("env = %q", got)
	}
	if got := alert.Labels["project_name"]; got != "paravoz-taxi" {
		t.Fatalf("project_name = %q", got)
	}
	if got := alert.Labels["service"]; got != "frontend" {
		t.Fatalf("service = %q", got)
	}
}

func TestBuildAlertInfersParavozQAEnvironmentFromComposeID(t *testing.T) {
	payload := DokployWebhook{
		Title: "Build completed successfully",
		Event: "build",
		Metadata: map[string]any{
			"composeId":       "Hyi2uWC8LJanL3N_hxqeJ",
			"projectName":     "paravoz-taxi",
			"applicationName": "frontend",
		},
	}

	alert := BuildAlert(payload, time.Date(2026, 6, 8, 13, 2, 0, 0, time.UTC), 5*time.Minute, "", nil)

	if got := alert.Labels["env"]; got != "qa" {
		t.Fatalf("env = %q", got)
	}
}

func TestBuildAlertMapsNestedDokployResources(t *testing.T) {
	payload := DokployWebhook{
		Title: "Build completed successfully",
		Event: "build",
		Metadata: map[string]any{
			"compose": map[string]any{
				"id":   "Hyi2uWC8LJanL3N_hxqeJ",
				"name": "frontend",
			},
			"project": map[string]any{
				"id":   "project-123",
				"name": "paravoz-taxi",
			},
			"application": map[string]any{
				"id":   "application-123",
				"name": "paravoz-frontend-qa",
			},
		},
	}

	alert := BuildAlert(payload, time.Date(2026, 6, 12, 7, 32, 0, 0, time.UTC), 5*time.Minute, "", nil)

	if got := alert.Labels["compose_id"]; got != "Hyi2uWC8LJanL3N_hxqeJ" {
		t.Fatalf("compose_id = %q", got)
	}
	if got := alert.Labels["project_name"]; got != "paravoz-taxi" {
		t.Fatalf("project_name = %q", got)
	}
	if got := alert.Labels["service"]; got != "paravoz-frontend-qa" {
		t.Fatalf("service = %q", got)
	}
	if got := alert.Labels["env"]; got != "qa" {
		t.Fatalf("env = %q", got)
	}
}

func TestBuildAlertInfersParavozProdEnvironmentFromComposeID(t *testing.T) {
	payload := DokployWebhook{
		Title: "Build completed successfully",
		Event: "build",
		Metadata: map[string]any{
			"composeId":       "1Wx5RsbIDrb69IsLdBjae",
			"projectName":     "paravoz-taxi",
			"applicationName": "frontend",
		},
	}

	alert := BuildAlert(payload, time.Date(2026, 6, 8, 13, 2, 0, 0, time.UTC), 5*time.Minute, "", nil)

	if got := alert.Labels["env"]; got != "prod" {
		t.Fatalf("env = %q", got)
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

func TestBuildAlertInfersVolumeBackupFromGenericSuccess(t *testing.T) {
	payload := DokployWebhook{
		Event:   "success",
		Title:   "Volume Backup Successful",
		Message: "Volume backup completed successfully",
	}

	alert := BuildAlert(payload, time.Date(2026, 6, 6, 10, 31, 0, 0, time.UTC), 5*time.Minute, "", nil)

	if got := alert.Labels["event"]; got != "volumeBackup" {
		t.Fatalf("event = %q", got)
	}
	if got := alert.Labels["event_group"]; got != "backups" {
		t.Fatalf("event_group = %q", got)
	}
	if got := alert.Labels["alertname"]; got != "DokployVolumeBackup" {
		t.Fatalf("alertname = %q", got)
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
