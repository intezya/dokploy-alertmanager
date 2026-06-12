# Agent Instructions

This repository contains the Dokploy-to-Alertmanager webhook adapter.

## Scope

These instructions apply to this repository.

## Project Context

- The adapter receives Dokploy notification webhooks on `POST /dokploy`.
- It forwards normalized alerts to Alertmanager `POST /api/v2/alerts`.
- The production container runs on `de1` in the observability stack at `/home/intezya/observability`.
- The production image is `ghcr.io/intezya/dokploy-alertmanager:latest`.
- Alertmanager routing and Telegram templates are managed separately in the devops repository under `de1/containers/observability/alertmanager/alertmanager.yml`.

## Operating Rules

- Do not print secrets, tokens, webhook credentials, Telegram bot tokens, or real `.env` values.
- Start live-host work with read-only inspection commands.
- Back up remote managed files before replacing them.
- Keep changes focused on adapter behavior, tests, docs, or release plumbing.
- Do not change Alertmanager YAML in this repository; update it in the devops repository and deploy it separately.

## Validation

- After Go code changes, run `go test ./...`.
- For webhook mapping changes, add or update tests in `internal/adapter`.
- Before claiming a deployed fix, verify the container image revision on `de1` and check recent adapter and Alertmanager logs.
