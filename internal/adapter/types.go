package adapter

import (
	"encoding/json"
	"strings"
	"time"
)

type DokployWebhook struct {
	Title     string            `json:"title"`
	Message   string            `json:"message"`
	Timestamp string            `json:"timestamp"`
	Event     string            `json:"event"`
	Type      string            `json:"type"`
	Action    string            `json:"action"`
	Labels    map[string]string `json:"labels"`
	Metadata  map[string]any    `json:"metadata"`
	Raw       map[string]any    `json:"-"`
}

func DecodeDokployWebhook(data []byte) (DokployWebhook, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return DokployWebhook{}, err
	}

	var payload DokployWebhook
	if err := json.Unmarshal(data, &payload); err != nil {
		return DokployWebhook{}, err
	}
	payload.Raw = raw
	if payload.Metadata == nil {
		payload.Metadata = extractMetadata(raw)
	}
	return payload, nil
}

type AlertmanagerAlert struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt,omitempty"`
	EndsAt       time.Time         `json:"endsAt,omitempty"`
	GeneratorURL string            `json:"generatorURL,omitempty"`
}

type EventMapping struct {
	Event      string
	Group      string
	AlertName  string
	Severity   string
	Resolvable bool
}

func MapDokployEvent(payload DokployWebhook) EventMapping {
	inferredEvent := normalizeEvent(inferEvent(payload))
	event := normalizeEvent(firstNonEmpty(payload.Event, payload.Type, payload.Action, metadataString(payload.Metadata, "event"), inferredEvent))
	if isGenericStatusEvent(event) && inferredEvent != "" {
		event = inferredEvent
	}

	switch event {
	case "appdeploy", "appdeployed", "deployment-success", "deploy-success":
		return EventMapping{Event: "appDeploy", Group: "deployments", AlertName: "DokployAppDeploy", Severity: "info", Resolvable: true}
	case "appbuilderror", "build-error", "builderror", "deploy-error", "deployment-error", "deployment-failed", "appdeployerror":
		return EventMapping{Event: "appBuildError", Group: "deployments", AlertName: "DokployAppBuildError", Severity: "critical", Resolvable: false}
	case "databasebackup", "database-backup", "dbbackup":
		return backupMapping("databaseBackup", "DokployDatabaseBackup", payload)
	case "volumebackup", "volume-backup":
		return backupMapping("volumeBackup", "DokployVolumeBackup", payload)
	case "dokploybackup", "dokploy-backup":
		return backupMapping("dokployBackup", "DokployBackup", payload)
	case "dockercleanup", "docker-cleanup":
		return EventMapping{Event: "dockerCleanup", Group: "maintenance", AlertName: "DokployDockerCleanup", Severity: "info", Resolvable: true}
	case "dokployrestart", "dokploy-restart", "restart":
		return EventMapping{Event: "dokployRestart", Group: "runtime", AlertName: "DokployRestart", Severity: "warning", Resolvable: true}
	case "serverthreshold", "server-threshold", "threshold", "cpu", "memory":
		return EventMapping{Event: "serverThreshold", Group: "capacity", AlertName: "DokployServerThreshold", Severity: "warning", Resolvable: false}
	default:
		return EventMapping{Event: firstNonEmpty(payload.Event, payload.Type, payload.Action, "unknown"), Group: "unknown", AlertName: "DokployNotification", Severity: "info", Resolvable: true}
	}
}

func BuildAlert(payload DokployWebhook, now time.Time, endsAfter time.Duration, externalURL string, staticLabels map[string]string) AlertmanagerAlert {
	mapping := MapDokployEvent(payload)
	startsAt := parseTimestamp(payload.Timestamp, now)
	endsAt := startsAt.Add(endsAfter)
	if !mapping.Resolvable && endsAfter <= 0 {
		endsAt = time.Time{}
	}

	labels := map[string]string{
		"alertname":   mapping.AlertName,
		"source":      "dokploy",
		"event":       mapping.Event,
		"event_group": mapping.Group,
		"severity":    mapping.Severity,
	}
	for key, value := range payload.Labels {
		if key != "" && value != "" {
			labels[key] = value
		}
	}
	for key, value := range staticLabels {
		if key != "" && value != "" {
			labels[key] = value
		}
	}

	addLabelFromMetadata(labels, payload.Metadata, "applicationId", "application_id")
	addLabelFromMetadata(labels, payload.Metadata, "composeId", "compose_id")
	addLabelFromMetadata(labels, payload.Metadata, "deploymentId", "deployment_id")
	addLabelFromMetadata(labels, payload.Metadata, "status", "status")
	addLabelFromMetadata(labels, payload.Metadata, "serverName", "server")
	addLabelFromMetadata(labels, payload.Metadata, "ServerName", "server")
	addLabelFromMetadata(labels, payload.Metadata, "Type", "threshold_type")
	addLabelFromMetadata(labels, payload.Metadata, "projectName", "project_name")
	addLabelFromMetadata(labels, payload.Metadata, "project_name", "project_name")
	addLabelFromMetadata(labels, payload.Metadata, "applicationName", "service")
	addLabelFromMetadata(labels, payload.Metadata, "application_name", "service")
	addLabelFromMetadata(labels, payload.Metadata, "service", "service")
	addLabelFromMetadata(labels, payload.Metadata, "appName", "service")
	addLabelFromMetadata(labels, payload.Metadata, "app_name", "service")
	addEnvironmentLabel(labels, payload.Metadata)

	title := firstNonEmpty(payload.Title, mapping.AlertName)
	message := firstNonEmpty(payload.Message, title)

	return AlertmanagerAlert{
		Labels: labels,
		Annotations: map[string]string{
			"summary":     title,
			"description": message,
		},
		StartsAt:     startsAt,
		EndsAt:       endsAt,
		GeneratorURL: externalURL,
	}
}

func addEnvironmentLabel(labels map[string]string, metadata map[string]any) {
	if labels["env"] != "" {
		return
	}
	env := firstNonEmpty(
		metadataString(metadata, "env"),
		metadataString(metadata, "environment"),
		metadataString(metadata, "environmentName"),
		metadataString(metadata, "environment_name"),
	)
	if env == "" {
		env = inferEnvironment(labels, metadata)
	}
	if env != "" {
		labels["env"] = env
	}
}

func inferEnvironment(labels map[string]string, metadata map[string]any) string {
	projectName := strings.ToLower(firstNonEmpty(
		labels["project_name"],
		metadataString(metadata, "projectName"),
		metadataString(metadata, "project_name"),
	))
	service := strings.ToLower(firstNonEmpty(
		labels["service"],
		metadataString(metadata, "applicationName"),
		metadataString(metadata, "application_name"),
		metadataString(metadata, "service"),
		metadataString(metadata, "appName"),
		metadataString(metadata, "app_name"),
	))
	composeID := firstNonEmpty(
		labels["compose_id"],
		metadataString(metadata, "composeId"),
		metadataString(metadata, "compose_id"),
	)

	switch composeID {
	case "R6jEpEob4kvnZ0KvspKYJ", "Hyi2uWC8LJanL3N_hxqeJ":
		return "qa"
	case "kAfIToEfQJ5yF8M0eCrN4", "1Wx5RsbIDrb69IsLdBjae":
		return "prod"
	}

	if projectName == "paravoz-taxi" {
		switch service {
		case "paravoz-qa", "paravoz-frontend-qa", "backend-qa", "frontend-qa":
			return "qa"
		case "paravoz", "paravoz-frontend", "backend", "frontend":
			return "prod"
		}
	}

	return ""
}

func backupMapping(event, alertName string, payload DokployWebhook) EventMapping {
	severity := "info"
	resolvable := true
	text := strings.ToLower(payload.Title + " " + payload.Message + " " + metadataString(payload.Metadata, "status"))
	if strings.Contains(text, "fail") || strings.Contains(text, "error") {
		severity = "warning"
		resolvable = false
	}
	return EventMapping{Event: event, Group: "backups", AlertName: alertName, Severity: severity, Resolvable: resolvable}
}

func inferEvent(payload DokployWebhook) string {
	text := strings.ToLower(payload.Title + " " + payload.Message)
	switch {
	case strings.Contains(text, "build") && (strings.Contains(text, "fail") || strings.Contains(text, "error")):
		return "appBuildError"
	case strings.Contains(text, "deploy") && (strings.Contains(text, "success") || strings.Contains(text, "deployed")):
		return "appDeploy"
	case strings.Contains(text, "database") && strings.Contains(text, "backup"):
		return "databaseBackup"
	case strings.Contains(text, "volume") && strings.Contains(text, "backup"):
		return "volumeBackup"
	case strings.Contains(text, "dokploy") && strings.Contains(text, "restart"):
		return "dokployRestart"
	case strings.Contains(text, "cpu") || strings.Contains(text, "memory") || strings.Contains(text, "threshold"):
		return "serverThreshold"
	default:
		return ""
	}
}

func isGenericStatusEvent(event string) bool {
	switch event {
	case "success", "successful", "done", "failed", "failure", "error":
		return true
	default:
		return false
	}
}

func normalizeEvent(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "_", "-")
	return strings.ToLower(value)
}

func parseTimestamp(value string, fallback time.Time) time.Time {
	if value == "" {
		return fallback.UTC()
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return fallback.UTC()
	}
	return parsed.UTC()
}

func addLabelFromMetadata(labels map[string]string, metadata map[string]any, key, label string) {
	value := metadataString(metadata, key)
	if value != "" {
		labels[label] = value
	}
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case float64, bool:
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(jsonNumberString(typed), ".0"), "."))
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

func jsonNumberString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func extractMetadata(raw map[string]any) map[string]any {
	metadata := make(map[string]any)
	for key, value := range raw {
		switch key {
		case "title", "message", "timestamp", "event", "type", "action", "metadata":
			continue
		default:
			metadata[key] = value
		}
	}
	return metadata
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
