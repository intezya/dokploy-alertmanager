# Dokploy Alertmanager Adapter

Small HTTP adapter that receives Dokploy notification webhooks and forwards them to Alertmanager `POST /api/v2/alerts`.

## Event Mapping

| Dokploy event | Alertmanager labels |
| --- | --- |
| `appDeploy` | `event_group=deployments`, `severity=info` |
| `appBuildError` | `event_group=deployments`, `severity=critical` |
| `databaseBackup` | `event_group=backups`, `severity=info` or `warning` |
| `volumeBackup` | `event_group=backups`, `severity=info` or `warning` |
| `dokployBackup` | `event_group=backups`, `severity=info` or `warning` |
| `dokployRestart` | `event_group=runtime`, `severity=warning` |
| `serverThreshold` | `event_group=capacity`, `severity=warning` |
| `dockerCleanup` | `event_group=maintenance`, `severity=info` |

Unknown events are forwarded as `alertname=DokployNotification`, `event_group=unknown`.

## Configuration

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `ALERTMANAGER_URL` | yes | | Alertmanager base URL or full `/api/v2/alerts` endpoint. |
| `LISTEN_ADDR` | no | `:8080` | HTTP listen address. |
| `WEBHOOK_TOKEN` | no | | Shared secret. If set, requests must send `Authorization: Bearer <token>` or `X-Webhook-Token`. |
| `ALERT_ENDS_AFTER` | no | `5m` | Sets `endsAt` for one-shot Dokploy events. |
| `EXTERNAL_URL` | no | | Value for Alertmanager `generatorURL`. |
| `STATIC_LABELS` | no | | Comma-separated labels, for example `env=prod,service=dokploy`. |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, or `error`. |

## Container Image

GitHub Actions builds and pushes the image to GHCR on every push to `main`:

```text
ghcr.io/intezya/dokploy-alertmanager:v0.1.<run_number>
ghcr.io/intezya/dokploy-alertmanager:latest
```

## Run Locally

```bash
ALERTMANAGER_URL=http://localhost:9093 \
WEBHOOK_TOKEN=change-me \
go run ./cmd/dokploy-alertmanager
```

Test with:

```bash
curl -X POST http://localhost:8080/dokploy \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer change-me' \
  -d '{
    "event": "appBuildError",
    "title": "Build failed",
    "message": "Backend build failed",
    "timestamp": "2026-06-06T10:30:00Z"
  }'
```

## Dokploy Setup

Create a Webhook notification in Dokploy and point it to:

```text
https://your-adapter.example.com/dokploy
```

If `WEBHOOK_TOKEN` is set and Dokploy cannot set custom headers in your version, include the adapter behind a reverse proxy that injects the header or protects the endpoint by IP allowlist/basic auth.

## Alertmanager Routing Example

```yaml
route:
  receiver: default
  group_by: [source, event_group, alertname]
  routes:
    - matchers:
        - source="dokploy"
        - event_group="deployments"
      receiver: dokploy-deployments
    - matchers:
        - source="dokploy"
        - event_group="backups"
      receiver: dokploy-backups
    - matchers:
        - source="dokploy"
        - event_group="runtime"
      receiver: dokploy-runtime
    - matchers:
        - source="dokploy"
        - event_group="capacity"
      receiver: dokploy-capacity

receivers:
  - name: default
  - name: dokploy-deployments
    telegram_configs:
      - bot_token_file: /run/secrets/telegram_bot_token
        chat_id: -1001234567890
        message_thread_id: 11
  - name: dokploy-backups
    telegram_configs:
      - bot_token_file: /run/secrets/telegram_bot_token
        chat_id: -1001234567890
        message_thread_id: 12
  - name: dokploy-runtime
    telegram_configs:
      - bot_token_file: /run/secrets/telegram_bot_token
        chat_id: -1001234567890
        message_thread_id: 13
  - name: dokploy-capacity
    telegram_configs:
      - bot_token_file: /run/secrets/telegram_bot_token
        chat_id: -1001234567890
        message_thread_id: 14
```

## Endpoints

- `POST /dokploy`
- `GET /healthz`
- `GET /readyz`
