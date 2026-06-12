# Release Pipeline

This project publishes a Docker image for the Dokploy Alertmanager adapter.

## Normal Flow

1. Make the local code or documentation change.
2. Run focused validation:

```bash
go test ./...
```

3. Commit the change on a branch.
4. Open a pull request for review unless the change is urgent operational repair.
5. Wait for CI to pass.
6. Merge to `main`.
7. GitHub Actions workflow `Docker image` builds and pushes:

```text
ghcr.io/intezya/dokploy-alertmanager:v0.1.<run_number>
ghcr.io/intezya/dokploy-alertmanager:latest
```

8. Deploy on `de1` from the observability stack:

```bash
ssh de1 'cd /home/intezya/observability && docker compose pull dokploy-alertmanager && docker compose up -d dokploy-alertmanager'
```

## Post-Deploy Verification

Verify the running image revision:

```bash
ssh de1 'docker image inspect ghcr.io/intezya/dokploy-alertmanager:latest --format "revision={{index .Config.Labels \"org.opencontainers.image.revision\"}} version={{index .Config.Labels \"org.opencontainers.image.version\"}}"'
ssh de1 'docker inspect -f "started={{.State.StartedAt}} image={{.Image}}" observability-dokploy-alertmanager'
```

Check the service and recent logs:

```bash
ssh de1 'cd /home/intezya/observability && docker compose ps dokploy-alertmanager'
ssh de1 'cd /home/intezya/observability && docker compose logs --since 10m dokploy-alertmanager'
ssh de1 'cd /home/intezya/observability && docker compose logs --since 10m alertmanager'
```

For routing/template changes, send one test event through the adapter and confirm there are no Alertmanager notification errors.

## Alertmanager Config

Alertmanager YAML is not released by this repository. It lives in the devops repository:

```text
/Users/kurumi/Projects/my/devops/de1/containers/observability/alertmanager/alertmanager.yml
```

Deploy Alertmanager config changes separately to:

```text
de1:/home/intezya/observability/alertmanager/alertmanager.yml
```

Before replacing the remote file, create a timestamped backup. After replacing it, run:

```bash
ssh de1 'cd /home/intezya/observability && docker compose exec -T alertmanager amtool check-config /etc/alertmanager/alertmanager.yml'
ssh de1 'cd /home/intezya/observability && docker compose restart alertmanager'
```

## Emergency Flow

For urgent production fixes:

1. Make the smallest possible change.
2. Run `go test ./...`.
3. Commit directly to `main` only if waiting for PR review would prolong an active incident.
4. Watch the `Docker image` workflow until it publishes successfully.
5. Pull and restart `dokploy-alertmanager` on `de1`.
6. Verify logs and send a test event.
7. Open a follow-up PR or issue if the emergency path skipped normal review.
