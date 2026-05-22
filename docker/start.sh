#!/bin/sh
set -eu

PIXIU_CONFIG_PATH="/etc/pixiu/config.yaml"
NGINX_HTTP_PORT=80
NGINX_HTTPS_PORT=443

log() {
    echo "[docker-entrypoint] $*"
}

is_true() {
    value="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')"
    case "$value" in
        1|true|yes|on)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

read_yaml_value() {
    path="$1"
    file="$2"

    awk -v path="$path" '
        function trim(value) {
            sub(/^[[:space:]]+/, "", value)
            sub(/[[:space:]]+$/, "", value)
            return value
        }

        /^[[:space:]]*#/ || /^[[:space:]]*$/ {
            next
        }

        {
            match($0, /^[[:space:]]*/)
            indent = RLENGTH
            level = int(indent / 2)

            line = substr($0, indent + 1)
            separator = index(line, ":")
            if (separator == 0) {
                next
            }

            key = trim(substr(line, 1, separator - 1))
            value = substr(line, separator + 1)

            sub(/[[:space:]]*#.*$/, "", value)
            value = trim(value)
            gsub(/^["'"'"']|["'"'"']$/, "", value)

            if (value == "null" || value == "~") {
                value = ""
            }

            stack[level] = key
            for (i in stack) {
                if (i > level) {
                    delete stack[i]
                }
            }

            current = stack[0]
            for (i = 1; i <= level; i++) {
                current = current "." stack[i]
            }

            if (current == path) {
                print value
                exit
            }
        }
    ' "$file"
}

write_proxy_locations() {
    cat <<'EOF'
        location / {
            proxy_pass http://127.0.0.1:8091;
            proxy_http_version 1.1;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection $connection_upgrade;
        }
EOF
}

load_config() {
    if [ ! -f "$PIXIU_CONFIG_PATH" ]; then
        log "pixiu config file not found: $PIXIU_CONFIG_PATH"
        exit 1
    fi

    NGINX_ENABLE_SSL="$(read_yaml_value tls.enable "$PIXIU_CONFIG_PATH" || true)"
    NGINX_SSL_CERT_PATH="$(read_yaml_value tls.cert_file "$PIXIU_CONFIG_PATH" || true)"
    NGINX_SSL_KEY_PATH="$(read_yaml_value tls.key_file "$PIXIU_CONFIG_PATH" || true)"

    # tls 没写、tls 被注释、tls.enable 没写时，默认关闭 HTTPS
    if [ -z "${NGINX_ENABLE_SSL:-}" ]; then
        NGINX_ENABLE_SSL="false"
    fi

    NGINX_SSL_CERT_PATH="${NGINX_SSL_CERT_PATH:-}"
    NGINX_SSL_KEY_PATH="${NGINX_SSL_KEY_PATH:-}"
}

validate_config() {
    if is_true "$NGINX_ENABLE_SSL"; then
        if [ -z "${NGINX_SSL_CERT_PATH:-}" ]; then
            log "https is enabled, but tls.cert_file is empty or missing"
            exit 1
        fi

        if [ -z "${NGINX_SSL_KEY_PATH:-}" ]; then
            log "https is enabled, but tls.key_file is empty or missing"
            exit 1
        fi

        if [ ! -f "$NGINX_SSL_CERT_PATH" ]; then
            log "certificate file not found: $NGINX_SSL_CERT_PATH"
            exit 1
        fi

        if [ ! -f "$NGINX_SSL_KEY_PATH" ]; then
            log "key file not found: $NGINX_SSL_KEY_PATH"
            exit 1
        fi
    fi
}

generate_nginx_config() {
    cat > /etc/nginx/nginx.conf <<EOF
worker_processes auto;

events {
    worker_connections 1024;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;

    map \$http_upgrade \$connection_upgrade {
        default upgrade;
        '' close;
    }

    server {
        listen ${NGINX_HTTP_PORT};
        server_name _;

$(write_proxy_locations)
    }
EOF

    if is_true "$NGINX_ENABLE_SSL"; then
        cat >> /etc/nginx/nginx.conf <<EOF

    server {
        listen ${NGINX_HTTPS_PORT} ssl;
        server_name _;

        ssl_certificate ${NGINX_SSL_CERT_PATH};
        ssl_certificate_key ${NGINX_SSL_KEY_PATH};
        ssl_session_cache shared:SSL:10m;
        ssl_session_timeout 10m;
        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_prefer_server_ciphers on;

$(write_proxy_locations)
    }
EOF
    fi

    cat >> /etc/nginx/nginx.conf <<EOF
}
EOF
}

start_services() {
    /usr/local/bin/pixiu-server --configfile "$PIXIU_CONFIG_PATH" &
    pixiu_pid=$!

    nginx -g "daemon off;" &
    nginx_pid=$!

    cleanup() {
        trap - INT TERM EXIT
        kill -TERM "$pixiu_pid" "$nginx_pid" 2>/dev/null || true
        wait "$pixiu_pid" 2>/dev/null || true
        wait "$nginx_pid" 2>/dev/null || true
    }

    trap cleanup INT TERM EXIT

    while kill -0 "$pixiu_pid" 2>/dev/null && kill -0 "$nginx_pid" 2>/dev/null; do
        sleep 1
    done

    status=0

    if ! kill -0 "$pixiu_pid" 2>/dev/null; then
        pixiu_status=0
        wait "$pixiu_pid" || pixiu_status=$?
        if [ "$pixiu_status" -ne 0 ]; then
            status="$pixiu_status"
        fi
        log "pixiu-server exited"
    fi

    if ! kill -0 "$nginx_pid" 2>/dev/null; then
        nginx_status=0
        wait "$nginx_pid" || nginx_status=$?
        if [ "$nginx_status" -ne 0 ] && [ "$status" -eq 0 ]; then
            status="$nginx_status"
        fi
        log "nginx exited"
    fi

    cleanup
    exit "$status"
}

if [ $# -gt 0 ]; then
    exec "$@"
fi

load_config
validate_config
generate_nginx_config
nginx -t -c /etc/nginx/nginx.conf

if is_true "$NGINX_ENABLE_SSL"; then
    log "starting services with http=${NGINX_HTTP_PORT}, https=${NGINX_HTTPS_PORT}"
else
    log "starting services with http=${NGINX_HTTP_PORT}, https=disabled"
fi

start_services