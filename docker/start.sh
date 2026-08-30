#!/bin/sh
set -eu

PIXIU_CONFIG_PATH="/etc/pixiu/config.yaml"
NGINX_HTTP_PORT=80
NGINX_HTTPS_PORT=443
NGINX_LOG_DIR="${NGINX_LOG_DIR:-/etc/pixiu/nginx}"

# 限流默认值（可用环境变量覆盖）
NGINX_LOGIN_RATE="${NGINX_LOGIN_RATE:-1r/s}"
NGINX_LOGIN_BURST="${NGINX_LOGIN_BURST:-5}"
NGINX_LOGIN_CONN="${NGINX_LOGIN_CONN:-10}"
NGINX_API_RATE="${NGINX_API_RATE:-30r/s}"
NGINX_API_BURST="${NGINX_API_BURST:-60}"
NGINX_API_CONN="${NGINX_API_CONN:-50}"
NGINX_REAL_IP_FROM="${NGINX_REAL_IP_FROM:-127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16}"

# IP 白名单：无开关。存在且非空的白名单文件则生效，否则不限制。
# 默认路径：/etc/pixiu/nginx/ip-whitelist.txt（可用 NGINX_IP_WHITELIST_FILE 覆盖）
# 格式：每行一个 IPv4 CIDR；生效时仍放行本机/RFC1918；/healthz 不受限。
# 文件增删改后约数秒内热更新（见 NGINX_IP_LIST_WATCH_INTERVAL）。
NGINX_IP_WHITELIST_FILE="${NGINX_IP_WHITELIST_FILE:-${NGINX_LOG_DIR}/ip-whitelist.txt}"
NGINX_IP_WHITELIST_ON=false

# IP 黑名单：无开关。存在且非空的黑名单文件则拒绝其中 IP。
# 默认路径：/etc/pixiu/nginx/ip-blacklist.txt（可用 NGINX_IP_BLACKLIST_FILE 覆盖）
# 与白名单可同时生效：先判黑名单，再判白名单；/healthz 不受限；支持热更新。
NGINX_IP_BLACKLIST_FILE="${NGINX_IP_BLACKLIST_FILE:-${NGINX_LOG_DIR}/ip-blacklist.txt}"
NGINX_IP_BLACKLIST_ON=false

# 白/黑名单热更新轮询间隔（秒）；文件增删改后自动 reload nginx
NGINX_IP_LIST_WATCH_INTERVAL="${NGINX_IP_LIST_WATCH_INTERVAL:-5}"

log() {
    echo "[docker-entrypoint] $*"
}

trim() {
    printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

is_true() {
    value="$(trim "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')")"
    case "$value" in
        1|true|yes|on)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

read_section_value() {
    section="$1"
    key="$2"
    file="$3"

    awk -v section="$section" -v key="$key" '
        $0 ~ "^" section ":[[:space:]]*$" {
            in_section = 1
            next
        }

        in_section && /^[^[:space:]]/ {
            in_section = 0
        }

        in_section && $0 ~ "^[[:space:]]+" key ":[[:space:]]*" {
            sub("^[[:space:]]+" key ":[[:space:]]*", "", $0)
            sub(/[[:space:]]+#.*$/, "", $0)
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0)
            gsub(/^["'"'"']|["'"'"']$/, "", $0)
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0)
            print
            exit
        }
    ' "$file"
}

prepare_log_dir() {
    mkdir -p "$NGINX_LOG_DIR"
    # nginx worker 用户写入 access/error 等日志
    if id nginx >/dev/null 2>&1; then
        chown -R nginx:nginx "$NGINX_LOG_DIR" 2>/dev/null || true
    fi
    chmod 755 "$NGINX_LOG_DIR" 2>/dev/null || true
    : >>"$NGINX_LOG_DIR/access.log"
    : >>"$NGINX_LOG_DIR/client-ip.log"
    : >>"$NGINX_LOG_DIR/error.log"
    : >>"$NGINX_LOG_DIR/ip-whitelist.log"
    : >>"$NGINX_LOG_DIR/ip-blacklist.log"
}

# 将 CIDR 列表文件转为 nginx geo 片段（每行 "CIDR 1;"）
cidr_file_to_geo() {
    src="$1"
    dst="$2"
    awk '
        /^[[:space:]]*#/ { next }
        /^[[:space:]]*$/ { next }
        {
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", $1)
            if ($1 ~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+(\/[0-9]+)?$/) {
                print $1, "1;"
            }
        }
    ' "$src" >"$dst"
}

# 将名单启用状态写入对应日志
write_ip_list_status_log() {
    kind="$1"          # whitelist | blacklist
    enabled="$2"       # true | false
    count="${3:-0}"
    list_file="$4"
    geo_file="$5"
    status_log="${NGINX_LOG_DIR}/ip-${kind}.log"
    ts="$(date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date '+%Y-%m-%dT%H:%M:%S%z')"

    {
        echo "-----"
        echo "[${ts}] ip_${kind}_enabled=${enabled}"
        echo "[${ts}] ip_${kind}_file=${list_file}"
        if [ "$enabled" = "true" ]; then
            if [ "$kind" = "whitelist" ]; then
                echo "[${ts}] always_allow=127.0.0.1/32,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
            fi
            echo "[${ts}] effective_cidrs_count=${count}"
            echo "[${ts}] effective_cidrs_begin"
            if [ -f "$geo_file" ]; then
                awk '{ print $1 }' "$geo_file"
            fi
            echo "[${ts}] effective_cidrs_end"
        else
            echo "[${ts}] reason=file_missing_or_empty"
            echo "[${ts}] effective_cidrs_count=0"
        fi
    } >>"$status_log"

    if [ "$enabled" = "true" ]; then
        echo "[${ts}] [pixiu] ip ${kind} enabled, cidrs=${count}, file=${list_file}" >>"${NGINX_LOG_DIR}/error.log"
    else
        echo "[${ts}] [pixiu] ip ${kind} disabled (no file: ${list_file})" >>"${NGINX_LOG_DIR}/error.log"
    fi
}

# 生成白名单 geo：有文件则启用
prepare_ip_whitelist() {
    geo_file="${NGINX_LOG_DIR}/ip-whitelist.geo"
    NGINX_IP_WHITELIST_ON=false

    if [ ! -f "$NGINX_IP_WHITELIST_FILE" ] || [ ! -s "$NGINX_IP_WHITELIST_FILE" ]; then
        printf '# ip whitelist disabled (file missing)\n' >"$geo_file"
        write_ip_list_status_log whitelist false 0 "$NGINX_IP_WHITELIST_FILE" "$geo_file"
        log "IP 白名单未启用（文件不存在或为空）: ${NGINX_IP_WHITELIST_FILE}"
        return 0
    fi

    log "使用 IP 白名单文件: ${NGINX_IP_WHITELIST_FILE}"
    cidr_file_to_geo "$NGINX_IP_WHITELIST_FILE" "$geo_file"
    lines="$(wc -l <"$geo_file" | tr -d ' ')"
    if [ "${lines:-0}" -lt 1 ]; then
        log "IP 白名单文件无有效 IPv4 CIDR: ${NGINX_IP_WHITELIST_FILE}"
        exit 1
    fi
    NGINX_IP_WHITELIST_ON=true
    write_ip_list_status_log whitelist true "$lines" "$NGINX_IP_WHITELIST_FILE" "$geo_file"
    log "IP 白名单已加载: ${geo_file} (${lines} 条)，详情见 ${NGINX_LOG_DIR}/ip-whitelist.log"
}

# 生成黑名单 geo：有文件则启用
prepare_ip_blacklist() {
    geo_file="${NGINX_LOG_DIR}/ip-blacklist.geo"
    NGINX_IP_BLACKLIST_ON=false

    if [ ! -f "$NGINX_IP_BLACKLIST_FILE" ] || [ ! -s "$NGINX_IP_BLACKLIST_FILE" ]; then
        printf '# ip blacklist disabled (file missing)\n' >"$geo_file"
        write_ip_list_status_log blacklist false 0 "$NGINX_IP_BLACKLIST_FILE" "$geo_file"
        log "IP 黑名单未启用（文件不存在或为空）: ${NGINX_IP_BLACKLIST_FILE}"
        return 0
    fi

    log "使用 IP 黑名单文件: ${NGINX_IP_BLACKLIST_FILE}"
    cidr_file_to_geo "$NGINX_IP_BLACKLIST_FILE" "$geo_file"
    lines="$(wc -l <"$geo_file" | tr -d ' ')"
    if [ "${lines:-0}" -lt 1 ]; then
        log "IP 黑名单文件无有效 IPv4 CIDR: ${NGINX_IP_BLACKLIST_FILE}"
        exit 1
    fi
    NGINX_IP_BLACKLIST_ON=true
    write_ip_list_status_log blacklist true "$lines" "$NGINX_IP_BLACKLIST_FILE" "$geo_file"
    log "IP 黑名单已加载: ${geo_file} (${lines} 条)，详情见 ${NGINX_LOG_DIR}/ip-blacklist.log"
}

write_real_ip_directives() {
    if [ -z "${NGINX_REAL_IP_FROM:-}" ]; then
        return 0
    fi
    old_ifs="$IFS"
    IFS=','
    for cidr in $NGINX_REAL_IP_FROM; do
        cidr="$(trim "$cidr")"
        if [ -n "$cidr" ]; then
            printf '    set_real_ip_from %s;\n' "$cidr"
        fi
    done
    IFS="$old_ifs"
    cat <<'EOF'
    real_ip_header X-Forwarded-For;
    real_ip_recursive on;
EOF
}

write_geo_allow_block() {
    # 白名单 geo
    if ! is_true "$NGINX_IP_WHITELIST_ON"; then
        cat <<'EOF'
    # 无 IP 白名单文件：不按白名单限制
    map $remote_addr $ip_whitelisted {
        default 1;
    }
EOF
    else
        cat <<EOF
    # IP 白名单 + 本机/私网（集群探针）
    geo \$remote_addr \$ip_whitelisted {
        default 0;
        127.0.0.1/32 1;
        ::1/128 1;
        10.0.0.0/8 1;
        172.16.0.0/12 1;
        192.168.0.0/16 1;
        include ${NGINX_LOG_DIR}/ip-whitelist.geo;
    }
EOF
    fi

    # 黑名单 geo
    if ! is_true "$NGINX_IP_BLACKLIST_ON"; then
        cat <<'EOF'
    # 无 IP 黑名单文件
    map $remote_addr $ip_blacklisted {
        default 0;
    }
EOF
    else
        cat <<EOF
    # IP 黑名单：命中则拒绝
    geo \$remote_addr \$ip_blacklisted {
        default 0;
        include ${NGINX_LOG_DIR}/ip-blacklist.geo;
    }
EOF
    fi
}

# 先黑名单后白名单
write_ip_access_guard() {
    if is_true "$NGINX_IP_BLACKLIST_ON"; then
        cat <<'EOF'
            if ($ip_blacklisted = 1) {
                return 403;
            }
EOF
    fi
    if is_true "$NGINX_IP_WHITELIST_ON"; then
        cat <<'EOF'
            if ($ip_whitelisted = 0) {
                return 403;
            }
EOF
    fi
}

write_proxy_headers() {
    cat <<'EOF'
            proxy_http_version 1.1;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection $connection_upgrade;
EOF
}

write_proxy_locations() {
    cat <<EOF
        root /usr/share/nginx/html;
        index index.html;
        client_max_body_size 32m;

        # 访问日志含真实客户端 IP（经 real_ip 后 \$remote_addr）
        access_log ${NGINX_LOG_DIR}/access.log pixiu_access;
        # 仅记录真实客户端 IP
        access_log ${NGINX_LOG_DIR}/client-ip.log pixiu_client_ip;

        location = /pixiu/users/login {
$(write_ip_access_guard)
            limit_req zone=login_limit burst=${NGINX_LOGIN_BURST} nodelay;
            limit_conn perip_conn ${NGINX_LOGIN_CONN};
$(write_proxy_headers)
            proxy_pass http://127.0.0.1:8091;
        }

        location /api/ {
$(write_ip_access_guard)
            limit_req zone=api_limit burst=${NGINX_API_BURST} nodelay;
            limit_conn perip_conn ${NGINX_API_CONN};
$(write_proxy_headers)
            proxy_pass http://127.0.0.1:8091;
        }

        location /pixiu/ {
$(write_ip_access_guard)
            limit_req zone=api_limit burst=${NGINX_API_BURST} nodelay;
            limit_conn perip_conn ${NGINX_API_CONN};
$(write_proxy_headers)
            proxy_pass http://127.0.0.1:8091;
        }

        # 探针不走 IP 黑白名单
        location = /healthz {
            access_log off;
            proxy_pass http://127.0.0.1:8091;
            proxy_set_header Host \$host;
            proxy_set_header X-Real-IP \$remote_addr;
            proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto \$scheme;
        }

        location /api-ref/ {
$(write_ip_access_guard)
            proxy_pass http://127.0.0.1:8091;
            proxy_set_header Host \$host;
            proxy_set_header X-Real-IP \$remote_addr;
            proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto \$scheme;
        }

        location / {
$(write_ip_access_guard)
            try_files \$uri \$uri/ /index.html;
        }
EOF
}

load_config() {
    if [ ! -f "$PIXIU_CONFIG_PATH" ]; then
        log "pixiu config file not found: $PIXIU_CONFIG_PATH"
        exit 1
    fi

    NGINX_ENABLE_SSL="$(read_section_value tls enable "$PIXIU_CONFIG_PATH")"
    NGINX_SSL_CERT_PATH="$(read_section_value tls cert_file "$PIXIU_CONFIG_PATH")"
    NGINX_SSL_KEY_PATH="$(read_section_value tls key_file "$PIXIU_CONFIG_PATH")"

    if [ -z "${NGINX_ENABLE_SSL:-}" ]; then
        NGINX_ENABLE_SSL="false"
    fi
}

validate_config() {
    if is_true "$NGINX_ENABLE_SSL"; then
        if [ -z "${NGINX_SSL_CERT_PATH:-}" ] || [ -z "${NGINX_SSL_KEY_PATH:-}" ]; then
            log "tls.enable is true, but tls.cert_file or tls.key_file is missing"
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
    conf_out="${1:-/etc/nginx/nginx.conf}"
    cat > "$conf_out" <<EOF
worker_processes auto;
error_log ${NGINX_LOG_DIR}/error.log warn;
pid /var/run/nginx.pid;

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

    # \$remote_addr 在 real_ip 生效后即为真实客户端 IP
    log_format pixiu_access '\$remote_addr - [\$time_local] "\$request" '
                            '\$status \$body_bytes_sent rt=\$request_time '
                            'ua="\$http_user_agent" xff="\$http_x_forwarded_for"';
    log_format pixiu_client_ip '\$remote_addr';

    limit_req_zone \$binary_remote_addr zone=login_limit:10m rate=${NGINX_LOGIN_RATE};
    limit_req_zone \$binary_remote_addr zone=api_limit:10m rate=${NGINX_API_RATE};
    limit_conn_zone \$binary_remote_addr zone=perip_conn:10m;
    limit_req_status 429;
    limit_conn_status 429;

$(write_real_ip_directives)
$(write_geo_allow_block)

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
        cat >> "$conf_out" <<EOF

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

    cat >> "$conf_out" <<EOF
}
EOF
}

# 白/黑名单文件指纹（大小+校验和）；文件不存在为 absent
ip_list_fingerprint() {
    fp_one() {
        f="$1"
        if [ -f "$f" ] && [ -s "$f" ]; then
            printf '%s:%s' "$(wc -c <"$f" | tr -d ' ')" "$(cksum "$f" | awk '{print $1}')"
        else
            printf 'absent'
        fi
    }
    printf '%s|%s' "$(fp_one "$NGINX_IP_WHITELIST_FILE")" "$(fp_one "$NGINX_IP_BLACKLIST_FILE")"
}

# 重新加载白/黑名单并热更新 nginx（不中断业务进程）
reload_ip_lists() {
    prepare_ip_whitelist
    prepare_ip_blacklist

    tmp_conf="/etc/nginx/nginx.conf.hot"
    generate_nginx_config "$tmp_conf"
    if ! nginx -t -c "$tmp_conf" >/dev/null 2>&1; then
        log "IP 名单热更新失败：nginx -t 未通过，保留原配置"
        rm -f "$tmp_conf"
        return 1
    fi
    mv "$tmp_conf" /etc/nginx/nginx.conf
    if ! nginx -s reload; then
        log "IP 名单热更新失败：nginx -s reload 失败"
        return 1
    fi
    log "IP 白/黑名单已热更新 whitelist=${NGINX_IP_WHITELIST_ON} blacklist=${NGINX_IP_BLACKLIST_ON}"
    return 0
}

# 后台轮询白/黑名单文件变化
watch_ip_lists() {
    interval="$NGINX_IP_LIST_WATCH_INTERVAL"
    case "$interval" in
        ''|*[!0-9]*)
            interval=5
            ;;
    esac
    if [ "$interval" -lt 1 ]; then
        interval=5
    fi

    last="$(ip_list_fingerprint)"
    log "IP 白/黑名单热更新已开启，检测间隔 ${interval}s"
    while true; do
        sleep "$interval"
        cur="$(ip_list_fingerprint)"
        if [ "$cur" = "$last" ]; then
            continue
        fi
        log "检测到 IP 白/黑名单文件变化，开始热更新"
        if reload_ip_lists; then
            last="$(ip_list_fingerprint)"
        fi
    done
}

start_services() {
    /app --configfile "$PIXIU_CONFIG_PATH" &
    pixiu_pid=$!

    nginx -g "daemon off;" &
    nginx_pid=$!

    watch_ip_lists &
    watcher_pid=$!

    cleanup() {
        trap - INT TERM EXIT
        kill -TERM "$pixiu_pid" "$nginx_pid" "$watcher_pid" 2>/dev/null || true
        wait "$pixiu_pid" 2>/dev/null || true
        wait "$nginx_pid" 2>/dev/null || true
        wait "$watcher_pid" 2>/dev/null || true
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
prepare_log_dir
prepare_ip_whitelist
prepare_ip_blacklist
generate_nginx_config
nginx -t -c /etc/nginx/nginx.conf

whitelist_flag="off"
blacklist_flag="off"
if is_true "$NGINX_IP_WHITELIST_ON"; then
    whitelist_flag="on"
fi
if is_true "$NGINX_IP_BLACKLIST_ON"; then
    blacklist_flag="on"
fi

if is_true "$NGINX_ENABLE_SSL"; then
    log "starting services http=${NGINX_HTTP_PORT} https=${NGINX_HTTPS_PORT} login_rate=${NGINX_LOGIN_RATE} ip_whitelist=${whitelist_flag} ip_blacklist=${blacklist_flag} log_dir=${NGINX_LOG_DIR}"
else
    log "starting services http=${NGINX_HTTP_PORT} https=disabled login_rate=${NGINX_LOGIN_RATE} ip_whitelist=${whitelist_flag} ip_blacklist=${blacklist_flag} log_dir=${NGINX_LOG_DIR}"
fi

start_services
