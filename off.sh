#!/usr/bin/env bash
set -Eeuo pipefail

# Stop script for kudinovanatalya-psy.ru.
# Put this file next to up.sh, then run:
#   sudo bash /home/off.sh

# =========================
# CONFIG
# =========================
DEPLOY_ROOT="${DEPLOY_ROOT:-/home/psy}"
SERVICE_NAME="${SERVICE_NAME:-psy.service}"
NGINX_SERVICE_NAME="${NGINX_SERVICE_NAME:-nginx}"
CERTBOT_TIMER_NAME="${CERTBOT_TIMER_NAME:-certbot.timer}"
STOP_NGINX="${STOP_NGINX:-1}"
STOP_CERTBOT_TIMER="${STOP_CERTBOT_TIMER:-0}"

# =========================
# LOGGING
# =========================
ts() { date -u '+%Y-%m-%d %H:%M:%S UTC' 2>/dev/null || date; }
log() { local lvl="$1"; shift || true; printf '[%s] %s %s\n' "$lvl" "$(ts)" "$*"; }
info() { log INFO "$*"; }
warn() { log WARN "$*"; }
err() { log ERROR "$*"; }

trap 'rc=$?; err "failed at line $LINENO: $BASH_COMMAND"; exit "$rc"' ERR
trap 'rc=$?; info "exit rc=$rc"' EXIT

# =========================
# HELPERS
# =========================
have() { command -v "$1" >/dev/null 2>&1; }

require_root() {
  if [[ "$(id -u)" -ne 0 ]]; then
    err "run as root: sudo bash /home/off.sh"
    exit 1
  fi
}

require_cmd() {
  local cmd="$1"
  have "$cmd" || { err "$cmd not found"; exit 1; }
}

service_exists() {
  local name="$1"
  systemctl list-unit-files "$name" --no-legend 2>/dev/null | grep -q "^$name"
}

stop_service_if_exists() {
  local name="$1"
  if service_exists "$name"; then
    if systemctl is-active --quiet "$name"; then
      info "stopping $name"
      systemctl stop "$name"
    else
      info "$name already stopped"
    fi
  else
    warn "$name not found; skipping"
  fi
}

print_summary() {
  info "================ STOP SUMMARY ================"
  info "deploy root: $DEPLOY_ROOT"
  info "app service: $SERVICE_NAME"
  info "nginx stop: $STOP_NGINX"
  info "certbot timer stop: $STOP_CERTBOT_TIMER"
  info "---------------------------------------------"
  systemctl --no-pager --full status "$SERVICE_NAME" || true
  if [[ "$STOP_NGINX" == "1" ]]; then
    systemctl --no-pager --full status "$NGINX_SERVICE_NAME" || true
  fi
  if [[ "$STOP_CERTBOT_TIMER" == "1" ]]; then
    systemctl --no-pager --full status "$CERTBOT_TIMER_NAME" || true
  fi
}

# =========================
# MAIN
# =========================
main() {
  require_root
  require_cmd systemctl

  stop_service_if_exists "$SERVICE_NAME"

  if [[ "$STOP_NGINX" == "1" ]]; then
    stop_service_if_exists "$NGINX_SERVICE_NAME"
  else
    info "nginx stop skipped (STOP_NGINX=$STOP_NGINX)"
  fi

  if [[ "$STOP_CERTBOT_TIMER" == "1" ]]; then
    stop_service_if_exists "$CERTBOT_TIMER_NAME"
  else
    info "certbot timer stop skipped (STOP_CERTBOT_TIMER=$STOP_CERTBOT_TIMER)"
  fi

  print_summary
  info "portal services stopped"
}

main "$@"
