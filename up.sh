#!/usr/bin/env bash
set -Eeuo pipefail

# Deploy script for kudinovanatalya-psy.ru.
# Put this file at /home/up.sh, then run:
#   sudo bash /home/up.sh

# =========================
# CONFIG
# =========================
DOMAIN="${DOMAIN:-kudinovanatalya-psy.ru}"
SERVER_IP="${SERVER_IP:-46.149.70.190}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-natalia.kudinova.psy@gmail.com}"

DEPLOY_ROOT="${DEPLOY_ROOT:-/home/psy}"
APP_DIR="${APP_DIR:-$DEPLOY_ROOT/app}"
BIN_DIR="${BIN_DIR:-$DEPLOY_ROOT/bin}"
DATA_DIR="${DATA_DIR:-$DEPLOY_ROOT/data}"
ENV_FILE="${ENV_FILE:-$DEPLOY_ROOT/.env}"

REPO_URL="${REPO_URL:-https://github.com/yurch88/psy.git}"
BRANCH="${BRANCH:-main}"
REMOTE="${REMOTE:-origin}"

APP_USER="${APP_USER:-psy}"
APP_GROUP="${APP_GROUP:-psy}"
SERVICE_NAME="${SERVICE_NAME:-psy.service}"
APP_BIN="${APP_BIN:-$BIN_DIR/psy}"
APP_HOST="${APP_HOST:-127.0.0.1}"
APP_PORT="${APP_PORT:-8080}"
APP_ADDR="${APP_ADDR:-$APP_HOST:$APP_PORT}"
APP_HEALTH_URL="${APP_HEALTH_URL:-http://$APP_ADDR/healthz}"

GO_VERSION="${GO_VERSION:-1.26.1}"
GO_ROOT="${GO_ROOT:-/usr/local/go}"

NGINX_SITE_NAME="${NGINX_SITE_NAME:-psy}"
NGINX_AVAILABLE="${NGINX_AVAILABLE:-/etc/nginx/sites-available/$NGINX_SITE_NAME}"
NGINX_ENABLED="${NGINX_ENABLED:-/etc/nginx/sites-enabled/$NGINX_SITE_NAME}"
CERTBOT_WEBROOT="${CERTBOT_WEBROOT:-/var/www/certbot}"
REQUIRE_HTTPS="${REQUIRE_HTTPS:-1}"
CERTBOT_TEST_RENEW="${CERTBOT_TEST_RENEW:-0}"

BACKUP_ENABLE="${BACKUP_ENABLE:-1}"
BACKUP_DIR="${BACKUP_DIR:-/home/backups/psy}"
BACKUP_KEEP="${BACKUP_KEEP:-20}"

RUN_TESTS="${RUN_TESTS:-1}"

# Optional contact/link overrides for the app.
CONTACT_EMAIL="${CONTACT_EMAIL:-natalia.kudinova.psy@gmail.com}"
CONTACT_PHONE="${CONTACT_PHONE:-+7 (965) 260-50-32}"
CONTACT_LOCATION="${CONTACT_LOCATION:-Онлайн, Россия и другие страны}"
TELEGRAM_URL="${TELEGRAM_URL:-https://t.me/NatalyaPoetry}"
TG_BOT_TOKEN="${TG_BOT_TOKEN:-}"
TG_NOTIFY_CHAT_IDS="${TG_NOTIFY_CHAT_IDS:-}"
MAX_URL="${MAX_URL:-#contacts}"
CALENDAR_URL="${CALENDAR_URL:-/booking}"
USD_RATE_URL="${USD_RATE_URL:-https://www.cbr-xml-daily.ru/daily_json.js}"

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
    err "run as root: sudo bash /home/up.sh"
    exit 1
  fi
}

require_cmd() {
  local cmd="$1"
  have "$cmd" || { err "$cmd not found"; exit 1; }
}

detect_arch() {
  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) printf 'amd64' ;;
    aarch64|arm64) printf 'arm64' ;;
    *) err "unsupported architecture: $arch"; exit 1 ;;
  esac
}

version_number() {
  "$1" version 2>/dev/null | awk '{print $3}' | sed 's/^go//' || true
}

http_ok() {
  local url="$1"
  curl -fsS --max-time 5 -o /dev/null "$url" >/dev/null 2>&1
}

wait_http() {
  local url="$1" seconds="$2"
  local i
  for i in $(seq 1 "$seconds"); do
    if http_ok "$url"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

backup_rotate() {
  local keep="$1" dir="$2"
  [[ "$keep" -le 0 ]] && return 0
  mapfile -t files < <(ls -1t "$dir"/backup_*.tar.gz 2>/dev/null || true)
  [[ "${#files[@]}" -le "$keep" ]] && return 0

  local file
  for file in "${files[@]:$keep}"; do
    rm -f "$file" || true
  done
}

domain_points_to_server() {
  local resolved
  resolved="$(getent ahostsv4 "$DOMAIN" 2>/dev/null | awk '{print $1}' | sort -u | tr '\n' ' ' | sed 's/[[:space:]]*$//')"
  [[ -z "$resolved" ]] && return 1
  grep -qw "$SERVER_IP" <<<"$resolved"
}

# =========================
# INSTALL PREREQS
# =========================
install_packages() {
  info "installing system packages"

  export DEBIAN_FRONTEND=noninteractive

  apt-get update
  apt-get install -y \
    ca-certificates \
    curl \
    git \
    nginx \
    certbot \
    python3-certbot-nginx \
    tar \
    gzip \
    openssl

  systemctl enable --now nginx
}

install_go() {
  local current go_arch tmp

  if [[ -x "$GO_ROOT/bin/go" ]]; then
    current="$("$GO_ROOT/bin/go" version | awk '{print $3}' | sed 's/^go//')"
  elif have go; then
    current="$(version_number go)"
  else
    current=""
  fi

  if [[ "$current" == "$GO_VERSION" ]]; then
    info "Go $GO_VERSION already installed"
    export PATH="$GO_ROOT/bin:$PATH"
    return 0
  fi

  info "installing Go $GO_VERSION"
  go_arch="$(detect_arch)"
  tmp="$(mktemp -d /tmp/go-install.XXXXXX)"

  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz" -o "$tmp/go.tgz"
  rm -rf "$GO_ROOT"
  tar -C /usr/local -xzf "$tmp/go.tgz"
  rm -rf "$tmp"

  export PATH="$GO_ROOT/bin:$PATH"
  "$GO_ROOT/bin/go" version
}

ensure_user_and_dirs() {
  info "ensuring runtime user and dirs"

  if ! getent group "$APP_GROUP" >/dev/null 2>&1; then
    groupadd --system "$APP_GROUP"
  fi

  if ! id -u "$APP_USER" >/dev/null 2>&1; then
    useradd --system --gid "$APP_GROUP" --home-dir "$DEPLOY_ROOT" --shell /usr/sbin/nologin "$APP_USER"
  fi

  mkdir -p "$DEPLOY_ROOT" "$APP_DIR" "$BIN_DIR" "$DATA_DIR" "$BACKUP_DIR" "$CERTBOT_WEBROOT"
  touch "$DATA_DIR/bookings.jsonl"

  chown root:"$APP_GROUP" "$DEPLOY_ROOT"
  chown -R "$APP_USER:$APP_GROUP" "$DATA_DIR" "$BIN_DIR"
  chmod 750 "$DEPLOY_ROOT" "$DATA_DIR" "$BIN_DIR"
  chmod 640 "$DATA_DIR/bookings.jsonl"
}

ensure_env_file() {
  if [[ -f "$ENV_FILE" ]]; then
    info "env file exists: $ENV_FILE"
    if grep -q '^TELEGRAM_URL=https://t.me/NatalyaBKudinova$' "$ENV_FILE"; then
      info "updating TELEGRAM_URL in $ENV_FILE"
      sed -i 's|^TELEGRAM_URL=https://t.me/NatalyaBKudinova$|TELEGRAM_URL=https://t.me/NatalyaPoetry|' "$ENV_FILE"
    elif ! grep -q '^TELEGRAM_URL=' "$ENV_FILE"; then
      printf '\nTELEGRAM_URL=%s\n' "$TELEGRAM_URL" >>"$ENV_FILE"
    fi
    if grep -q '^CALENDAR_URL=/booking#calendar$' "$ENV_FILE"; then
      info "updating CALENDAR_URL in $ENV_FILE to /booking"
      sed -i 's|^CALENDAR_URL=/booking#calendar$|CALENDAR_URL=/booking|' "$ENV_FILE"
    elif ! grep -q '^CALENDAR_URL=' "$ENV_FILE"; then
      printf '\nCALENDAR_URL=%s\n' "$CALENDAR_URL" >>"$ENV_FILE"
    fi
    if ! grep -q '^TG_BOT_TOKEN=' "$ENV_FILE"; then
      printf '\nTG_BOT_TOKEN=%s\n' "$TG_BOT_TOKEN" >>"$ENV_FILE"
    fi
    if ! grep -q '^TG_NOTIFY_CHAT_IDS=' "$ENV_FILE"; then
      printf 'TG_NOTIFY_CHAT_IDS=%s\n' "$TG_NOTIFY_CHAT_IDS" >>"$ENV_FILE"
    fi
    return 0
  fi

  info "creating env file: $ENV_FILE"
  cat >"$ENV_FILE" <<EOF
ADDR=$APP_ADDR
DATA_DIR=$DATA_DIR
CONTACT_EMAIL=$CONTACT_EMAIL
CONTACT_PHONE=$CONTACT_PHONE
CONTACT_LOCATION=$CONTACT_LOCATION
TELEGRAM_URL=$TELEGRAM_URL
TG_BOT_TOKEN=$TG_BOT_TOKEN
TG_NOTIFY_CHAT_IDS=$TG_NOTIFY_CHAT_IDS
MAX_URL=$MAX_URL
CALENDAR_URL=$CALENDAR_URL
USD_RATE_URL=$USD_RATE_URL
EOF
  chmod 640 "$ENV_FILE"
  chown root:"$APP_GROUP" "$ENV_FILE"
}

# =========================
# BACKUP
# =========================
create_backup() {
  [[ "$BACKUP_ENABLE" == "1" ]] || return 0

  mkdir -p "$BACKUP_DIR"
  chmod 700 "$BACKUP_DIR" >/dev/null 2>&1 || true

  local stamp backup_file tmp meta_file current_commit
  stamp="$(date -u +%Y%m%d_%H%M%S)"
  backup_file="$BACKUP_DIR/backup_${DOMAIN}_${stamp}.tar.gz"
  tmp="$(mktemp -d /tmp/psy-backup.XXXXXX)"
  meta_file="$tmp/DEPLOY_META.txt"

  info "creating backup: $backup_file"

  mkdir -p "$tmp/home/psy"

  if [[ -d "$DATA_DIR" ]]; then
    mkdir -p "$tmp/home/psy/data"
    cp -a "$DATA_DIR"/. "$tmp/home/psy/data"/
  fi

  if [[ -f "$ENV_FILE" ]]; then
    cp -a "$ENV_FILE" "$tmp/home/psy/.env"
  fi

  if [[ -f "$APP_BIN" ]]; then
    mkdir -p "$tmp/home/psy/bin"
    cp -a "$APP_BIN" "$tmp/home/psy/bin/psy"
  fi

  current_commit=""
  if [[ -d "$APP_DIR/.git" ]]; then
    current_commit="$(git -C "$APP_DIR" rev-parse HEAD 2>/dev/null || true)"
  fi

  {
    echo "domain=$DOMAIN"
    echo "server_ip=$SERVER_IP"
    echo "created_at=$(ts)"
    echo "repo=$REPO_URL"
    echo "branch=$BRANCH"
    echo "commit=$current_commit"
    echo "data_dir=$DATA_DIR"
  } >"$meta_file"

  tar -C "$tmp" -czf "$backup_file" .
  rm -rf "$tmp"

  backup_rotate "$BACKUP_KEEP" "$BACKUP_DIR"
  info "backup OK: $backup_file"
}

# =========================
# REPO / BUILD
# =========================
sync_repo() {
  info "sync repo: $REPO_URL branch=$BRANCH"

  if [[ ! -d "$APP_DIR/.git" ]]; then
    rm -rf "$APP_DIR"
    git clone --branch "$BRANCH" "$REPO_URL" "$APP_DIR"
  fi

  git -C "$APP_DIR" remote set-url "$REMOTE" "$REPO_URL" >/dev/null 2>&1 || true
  git -C "$APP_DIR" fetch "$REMOTE" --prune
  git -C "$APP_DIR" checkout "$BRANCH"
  git -C "$APP_DIR" reset --hard "$REMOTE/$BRANCH"
  git -C "$APP_DIR" clean -fd
}

build_app() {
  info "building app"
  export PATH="$GO_ROOT/bin:$PATH"

  if [[ "$RUN_TESTS" == "1" ]]; then
    (cd "$APP_DIR" && go test ./...)
  fi

  (cd "$APP_DIR" && go build -trimpath -ldflags="-s -w" -o "$APP_BIN" ./cmd/psy)
  chown "$APP_USER:$APP_GROUP" "$APP_BIN"
  chmod 750 "$APP_BIN"
}

# =========================
# SYSTEMD
# =========================
install_systemd_service() {
  info "installing systemd service: $SERVICE_NAME"

  cat >"/etc/systemd/system/$SERVICE_NAME" <<EOF
[Unit]
Description=Psychologist website Go application
After=network.target

[Service]
Type=simple
User=$APP_USER
Group=$APP_GROUP
WorkingDirectory=$APP_DIR
EnvironmentFile=$ENV_FILE
ExecStart=$APP_BIN
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=false
ProtectSystem=full
ReadWritePaths=$DATA_DIR

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME"
}

restart_app() {
  info "restarting app"
  systemctl restart "$SERVICE_NAME"

  info "waiting app health: $APP_HEALTH_URL"
  if ! wait_http "$APP_HEALTH_URL" 60; then
    journalctl -u "$SERVICE_NAME" --no-pager -n 120 || true
    err "app did not become healthy"
    exit 1
  fi

  info "app health OK"
}

# =========================
# NGINX / CERTBOT
# =========================
write_nginx_http_config() {
  info "writing nginx HTTP config"

  cat >"$NGINX_AVAILABLE" <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name $DOMAIN;

    location /.well-known/acme-challenge/ {
        root $CERTBOT_WEBROOT;
        try_files \$uri =404;
    }

    location / {
        proxy_pass http://$APP_ADDR;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
EOF

  ln -sfn "$NGINX_AVAILABLE" "$NGINX_ENABLED"
  rm -f /etc/nginx/sites-enabled/default
  nginx -t
  systemctl restart nginx
}

write_nginx_https_config() {
  local ssl_options="" ssl_dhparam=""

  [[ -f /etc/letsencrypt/options-ssl-nginx.conf ]] && ssl_options="    include /etc/letsencrypt/options-ssl-nginx.conf;"
  [[ -f /etc/letsencrypt/ssl-dhparams.pem ]] && ssl_dhparam="    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;"

  info "writing nginx HTTPS config"

  cat >"$NGINX_AVAILABLE" <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name $DOMAIN;

    location /.well-known/acme-challenge/ {
        root $CERTBOT_WEBROOT;
        try_files \$uri =404;
    }

    location / {
        return 301 https://\$host\$request_uri;
    }
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name $DOMAIN;

    ssl_certificate /etc/letsencrypt/live/$DOMAIN/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/$DOMAIN/privkey.pem;
$ssl_options
$ssl_dhparam

    client_max_body_size 20m;

    location / {
        proxy_pass http://$APP_ADDR;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
EOF

  ln -sfn "$NGINX_AVAILABLE" "$NGINX_ENABLED"
  rm -f /etc/nginx/sites-enabled/default
  nginx -t
  systemctl reload nginx
}

ensure_firewall() {
  if have ufw; then
    ufw allow 80/tcp >/dev/null 2>&1 || true
    ufw allow 443/tcp >/dev/null 2>&1 || true
  fi
}

verify_local_http_challenge() {
  local token challenge_dir challenge_file url body

  token="upsh-$(date +%s)-$RANDOM"
  challenge_dir="$CERTBOT_WEBROOT/.well-known/acme-challenge"
  challenge_file="$challenge_dir/$token"
  url="http://$DOMAIN/.well-known/acme-challenge/$token"

  mkdir -p "$challenge_dir"
  printf '%s' "$token" >"$challenge_file"

  body="$(curl -fsS --max-time 5 --resolve "$DOMAIN:80:127.0.0.1" "$url" 2>/dev/null || true)"
  rm -f "$challenge_file" || true

  if [[ "$body" != "$token" ]]; then
    err "nginx local ACME challenge check failed"
    err "expected to read token from $url via $CERTBOT_WEBROOT"
    return 1
  fi

  info "nginx local ACME challenge check OK"
}

ensure_certificate() {
  if [[ -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" && -f "/etc/letsencrypt/live/$DOMAIN/privkey.pem" ]]; then
    info "certificate already exists for $DOMAIN"
    return 0
  fi

  if ! domain_points_to_server; then
    warn "DNS check did not confirm $DOMAIN -> $SERVER_IP"
    warn "certbot may fail until A-record points to this server"
  fi

  verify_local_http_challenge

  info "issuing Let's Encrypt certificate for $DOMAIN with nginx authenticator"
  if certbot certonly \
    --nginx \
    --domain "$DOMAIN" \
    --email "$CERTBOT_EMAIL" \
    --agree-tos \
    --non-interactive \
    --keep-until-expiring; then
    return 0
  fi

  warn "certbot nginx authenticator failed; trying webroot authenticator"
  certbot certonly \
    --webroot \
    --webroot-path "$CERTBOT_WEBROOT" \
    --domain "$DOMAIN" \
    --email "$CERTBOT_EMAIL" \
    --agree-tos \
    --non-interactive \
    --keep-until-expiring
}

setup_nginx_and_ssl() {
  ensure_firewall
  write_nginx_http_config

  if ensure_certificate; then
    write_nginx_https_config
    systemctl enable --now certbot.timer >/dev/null 2>&1 || true
    if [[ "$CERTBOT_TEST_RENEW" == "1" ]]; then
      certbot renew --dry-run || warn "certbot dry-run failed; certificate may still be valid, check certbot logs"
    fi
  else
    if [[ "$REQUIRE_HTTPS" == "1" ]]; then
      err "failed to issue certificate for $DOMAIN"
      exit 1
    fi
    warn "HTTPS not configured; app is available over HTTP"
  fi
}

# =========================
# SUMMARY
# =========================
print_summary() {
  local commit
  commit="$(git -C "$APP_DIR" rev-parse --short HEAD 2>/dev/null || true)"

  info "================ DEPLOY SUMMARY ================"
  info "domain:      $DOMAIN"
  info "ip:          $SERVER_IP"
  info "repo:        $REPO_URL"
  info "branch:      $BRANCH"
  info "commit:      $commit"
  info "app dir:     $APP_DIR"
  info "data dir:    $DATA_DIR"
  info "backup dir:  $BACKUP_DIR"
  info "service:     $SERVICE_NAME"
  info "local:       $APP_HEALTH_URL"
  info "public:      https://$DOMAIN/"
  info "================================================"

  systemctl --no-pager --full status "$SERVICE_NAME" || true
}

# =========================
# MAIN
# =========================
main() {
  require_root

  info "deploy start | domain=$DOMAIN | ip=$SERVER_IP | repo=$REPO_URL | branch=$BRANCH"

  install_packages
  install_go

  require_cmd git
  require_cmd curl
  require_cmd nginx
  require_cmd certbot
  require_cmd systemctl

  ensure_user_and_dirs
  ensure_env_file
  create_backup

  sync_repo
  build_app

  install_systemd_service
  restart_app

  setup_nginx_and_ssl

  print_summary
  info "deploy complete"
}

main "$@"
