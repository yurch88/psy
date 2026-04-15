#!/usr/bin/env bash
set -euo pipefail

# Заготовка для будущего деплоя в /home/psy/up.sh.
# Когда появится GitHub-репозиторий, задайте REPO_URL и при необходимости SERVICE_NAME.

APP_DIR="${APP_DIR:-/home/psy/app}"
REPO_URL="${REPO_URL:-}"
BRANCH="${BRANCH:-main}"
SERVICE_NAME="${SERVICE_NAME:-psy.service}"

if [[ -z "$REPO_URL" ]]; then
  echo "REPO_URL is not configured yet. Nothing to update."
  echo "Set REPO_URL=https://github.com/<owner>/<repo>.git and run this script again."
  exit 0
fi

if [[ ! -d "$APP_DIR/.git" ]]; then
  mkdir -p "$(dirname "$APP_DIR")"
  git clone --branch "$BRANCH" "$REPO_URL" "$APP_DIR"
else
  git -C "$APP_DIR" fetch origin "$BRANCH"
  git -C "$APP_DIR" checkout "$BRANCH"
  git -C "$APP_DIR" pull --ff-only origin "$BRANCH"
fi

cd "$APP_DIR"
go test ./...
go build -o /home/psy/bin/psy ./cmd/psy

if command -v systemctl >/dev/null 2>&1; then
  sudo systemctl restart "$SERVICE_NAME"
  sudo systemctl status "$SERVICE_NAME" --no-pager
else
  echo "systemctl is not available. Binary built at /home/psy/bin/psy"
fi
