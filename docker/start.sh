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
NGINX_IP_LIST_WATCH_INTERVAL="${NGINX_IP_LIST_WATCH_INTERVAL:-2}"

# nginx 日志按大小切割：默认单文件 100MB，保留 2 个历史；每 60s 检查一次
NGINX_LOG_MAX_SIZE="${NGINX_LOG_MAX_SIZE:-100m}"
NGINX_LOG_KEEP="${NGINX_LOG_KEEP:-2}"
NGINX_LOG_ROTATE_CHECK_INTERVAL="${NGINX_LOG_ROTATE_CHECK_INTERVAL:-60}"

# 登录爆破自动封禁：默认开启。扫 access.log 中 /pixiu/users/login 的 401/429，
# 滑动窗口内达到阈值则写入 ip-blacklist.txt（/32），由现有热更新生效。
# 关闭：NGINX_AUTO_BAN=false
NGINX_AUTO_BAN="${NGINX_AUTO_BAN:-true}"
NGINX_AUTO_BAN_WINDOW="${NGINX_AUTO_BAN_WINDOW:-300}"
NGINX_AUTO_BAN_THRESHOLD="${NGINX_AUTO_BAN_THRESHOLD:-15}"
NGINX_AUTO_BAN_TTL="${NGINX_AUTO_BAN_TTL:-86400}"
NGINX_AUTO_BAN_SCAN_INTERVAL="${NGINX_AUTO_BAN_SCAN_INTERVAL:-10}"
NGINX_AUTO_BAN_IGNORE="${NGINX_AUTO_BAN_IGNORE:-}"

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
            interval=2
            ;;
    esac
    if [ "$interval" -lt 1 ]; then
        interval=2
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

# 解析大小：支持 100、100k、100m、100g（大小写均可）
parse_size_bytes() {
    raw="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -d ' ')"
    case "$raw" in
        ''|*[!0-9kmg]*)
            printf '%s' "104857600"
            return 0
            ;;
    esac
    num="$raw"
    mul=1
    case "$raw" in
        *k)
            num="${raw%k}"
            mul=1024
            ;;
        *m)
            num="${raw%m}"
            mul=1048576
            ;;
        *g)
            num="${raw%g}"
            mul=1073741824
            ;;
    esac
    case "$num" in
        ''|*[!0-9]*)
            printf '%s' "104857600"
            return 0
            ;;
    esac
    printf '%s' "$((num * mul))"
}

# 单文件按大小轮转，保留 keep 个历史：file.1 ... file.N
rotate_one_log_file() {
    file="$1"
    max_bytes="$2"
    keep="$3"

    if [ ! -f "$file" ]; then
        return 1
    fi
    size="$(wc -c <"$file" | tr -d ' ')"
    case "$size" in
        ''|*[!0-9]*)
            return 1
            ;;
    esac
    if [ "$size" -lt "$max_bytes" ]; then
        return 1
    fi

    i="$keep"
    while [ "$i" -gt 1 ]; do
        prev=$((i - 1))
        if [ -f "${file}.${prev}" ]; then
            mv -f "${file}.${prev}" "${file}.${i}"
        fi
        i="$prev"
    done
    mv -f "$file" "${file}.1"
    : >"$file"
    if id nginx >/dev/null 2>&1; then
        chown nginx:nginx "$file" 2>/dev/null || true
    fi
    return 0
}

rotate_nginx_logs_if_needed() {
    max_bytes="$(parse_size_bytes "$NGINX_LOG_MAX_SIZE")"
    keep="$NGINX_LOG_KEEP"
    case "$keep" in
        ''|*[!0-9]*)
            keep=2
            ;;
    esac
    if [ "$keep" -lt 1 ]; then
        keep=2
    fi

    rotated=0
    for name in access.log client-ip.log error.log ip-whitelist.log ip-blacklist.log; do
        if rotate_one_log_file "${NGINX_LOG_DIR}/${name}" "$max_bytes" "$keep"; then
            rotated=1
            log "nginx 日志已切割: ${name} (max=${NGINX_LOG_MAX_SIZE}, keep=${keep})"
        fi
    done

    if [ "$rotated" -eq 1 ]; then
        # 让 nginx 重新打开日志文件句柄
        if ! nginx -s reopen >/dev/null 2>&1; then
            log "nginx -s reopen 失败，尝试 reload"
            nginx -s reload >/dev/null 2>&1 || true
        fi
    fi
}

# 后台按大小切割 nginx 日志
watch_nginx_log_rotate() {
    interval="$NGINX_LOG_ROTATE_CHECK_INTERVAL"
    case "$interval" in
        ''|*[!0-9]*)
            interval=60
            ;;
    esac
    if [ "$interval" -lt 10 ]; then
        interval=10
    fi

    log "nginx 日志切割已开启，检测间隔 ${interval}s，单文件上限 ${NGINX_LOG_MAX_SIZE}，保留 ${NGINX_LOG_KEEP} 个"
    while true; do
        sleep "$interval"
        rotate_nginx_logs_if_needed
    done
}

# ---------- 登录自动封禁 ----------

auto_ban_state_dir() {
    printf '%s' "${NGINX_LOG_DIR}/.auto-ban"
}

# 本机 / RFC1918 永不自动封
is_auto_ban_private_ip() {
    ip="$1"
    case "$ip" in
        127.*|10.*|192.168.*)
            return 0
            ;;
        172.1[6-9].*|172.2[0-9].*|172.3[0-1].*)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

# IPv4 是否落在 CIDR（支持 x.x.x.x 或 x.x.x.x/n）
ipv4_in_cidr() {
    ip="$1"
    cidr="$2"
    echo "$ip $cidr" | awk '
        function ip2int(s, a, i, n) {
            n = split(s, a, ".")
            if (n != 4) return -1
            for (i = 1; i <= 4; i++) {
                if (a[i] !~ /^[0-9]+$/ || a[i]+0 > 255) return -1
            }
            return a[1]*16777216 + a[2]*65536 + a[3]*256 + a[4]
        }
        {
            ip = $1
            cidr = $2
            bits = 32
            base = cidr
            if (index(cidr, "/") > 0) {
                split(cidr, p, "/")
                base = p[1]
                bits = p[2] + 0
            }
            if (bits < 0 || bits > 32) exit 1
            ipn = ip2int(ip)
            bn = ip2int(base)
            if (ipn < 0 || bn < 0) exit 1
            if (bits == 0) exit 0
            rem = 32 - bits
            step = 1
            while (rem > 0) { step = step * 2; rem-- }
            if (int(ipn / step) == int(bn / step)) exit 0
            exit 1
        }
    '
}

ip_in_ignore_list() {
    ip="$1"
    raw="$NGINX_AUTO_BAN_IGNORE"
    [ -n "$raw" ] || return 1
    old_ifs="$IFS"
    IFS=','
    # shellcheck disable=SC2086
    set -- $raw
    IFS="$old_ifs"
    for item in "$@"; do
        item="$(trim "$item")"
        [ -n "$item" ] || continue
        if ipv4_in_cidr "$ip" "$item"; then
            return 0
        fi
    done
    return 1
}

# 名单文件中是否已有该 IP（裸 IP 或 /32）
ip_in_list_file() {
    ip="$1"
    file="$2"
    [ -f "$file" ] || return 1
    awk -v ip="$ip" '
        /^[[:space:]]*#/ { next }
        /^[[:space:]]*$/ { next }
        {
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", $1)
            cidr = $1
            if (cidr == ip || cidr == (ip "/32")) exit 0
        }
        END { exit 1 }
    ' "$file"
}

should_skip_auto_ban_ip() {
    ip="$1"
    case "$ip" in
        ''|*[!0-9.]*)
            return 0
            ;;
    esac
    if is_auto_ban_private_ip "$ip"; then
        return 0
    fi
    if ip_in_ignore_list "$ip"; then
        return 0
    fi
    if ip_in_list_file "$ip" "$NGINX_IP_WHITELIST_FILE"; then
        return 0
    fi
    if ip_in_list_file "$ip" "$NGINX_IP_BLACKLIST_FILE"; then
        return 0
    fi
    return 1
}

# 从 access 增量中抽出登录 401/429 的客户端 IP
extract_login_attack_ips() {
    awk '
        {
            ip = $1
            if (ip !~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/) next
            n = split($0, parts, "\"")
            if (n < 3) next
            req = parts[2]
            rest = parts[3]
            sub(/^[[:space:]]+/, "", rest)
            status = rest
            sub(/[[:space:]].*$/, "", status)
            if (req !~ /\/pixiu\/users\/login/) next
            if (status != "401" && status != "429") next
            print ip
        }
    '
}

record_attack_and_maybe_ban() {
    ip="$1"
    now="$2"
    window="$3"
    threshold="$4"
    ttl="$5"
    state_dir="$(auto_ban_state_dir)"
    count_file="${state_dir}/count.${ip}"

    if should_skip_auto_ban_ip "$ip"; then
        return 0
    fi

    mkdir -p "$state_dir"
    echo "$now" >>"$count_file"

    # 滑动窗口：只保留 window 内时间戳
    awk -v now="$now" -v w="$window" '$1 + 0 >= now - w { print $1 }' "$count_file" >"${count_file}.tmp" \
        && mv -f "${count_file}.tmp" "$count_file"

    cnt="$(wc -l <"$count_file" | tr -d ' ')"
    case "$cnt" in
        ''|*[!0-9]*)
            return 0
            ;;
    esac
    if [ "$cnt" -lt "$threshold" ]; then
        return 0
    fi

    append_auto_ban_ip "$ip" "$cnt" "$window" "$ttl" "$now"
    : >"$count_file"
}

append_auto_ban_ip() {
    ip="$1"
    cnt="$2"
    window="$3"
    ttl="$4"
    now="$5"
    file="$NGINX_IP_BLACKLIST_FILE"

    # 再次确认（并发/竞态）
    if ip_in_list_file "$ip" "$file"; then
        return 0
    fi
    if should_skip_auto_ban_ip "$ip"; then
        return 0
    fi

    mkdir -p "$(dirname "$file")"
    touch "$file"

    if [ "$ttl" -eq 0 ]; then
        meta="# auto-ban permanent ip=${ip} count=${cnt} window=${window}s reason=login_401_429 ts=${now}"
    else
        until=$((now + ttl))
        meta="# auto-ban until=${until} ip=${ip} count=${cnt} window=${window}s reason=login_401_429 ts=${now}"
    fi

    lock="${file}.lock"
    (
        if command -v flock >/dev/null 2>&1; then
            flock 9
        fi
        if ip_in_list_file "$ip" "$file"; then
            exit 0
        fi
        printf '%s\n%s/32\n' "$meta" "$ip" >>"$file"
    ) 9>"$lock"

    ts="$(date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date '+%Y-%m-%dT%H:%M:%S%z')"
    echo "[${ts}] [pixiu] auto-ban ip=${ip} count=${cnt} window=${window}s ttl=${ttl}s" >>"${NGINX_LOG_DIR}/error.log"
    echo "[${ts}] auto_ban ip=${ip} count=${cnt} window=${window}s ttl=${ttl}s" >>"${NGINX_LOG_DIR}/ip-blacklist.log"
    log "auto-ban 已写入黑名单: ${ip}/32 (count=${cnt}, window=${window}s, ttl=${ttl}s)"
}

# 清理过期的 auto-ban 条目（不删人工行）
expire_auto_bans() {
    file="$NGINX_IP_BLACKLIST_FILE"
    [ -f "$file" ] || return 0
    now="$(date +%s)"
    tmp="${file}.expire.tmp"
    awk -v now="$now" '
        /^# auto-ban / {
            until = 0
            for (i = 1; i <= NF; i++) {
                if ($i ~ /^until=/) {
                    split($i, a, "=")
                    until = a[2] + 0
                }
            }
            if (until > 0 && now >= until) {
                skip_cidr = 1
                next
            }
            print
            next
        }
        skip_cidr && $1 ~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+(\/[0-9]+)?$/ {
            skip_cidr = 0
            next
        }
        {
            skip_cidr = 0
            print
        }
    ' "$file" >"$tmp"

    if cmp -s "$file" "$tmp" 2>/dev/null; then
        rm -f "$tmp"
        return 0
    fi
    mv -f "$tmp" "$file"
    ts="$(date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date '+%Y-%m-%dT%H:%M:%S%z')"
    echo "[${ts}] [pixiu] auto-ban expired entries purged" >>"${NGINX_LOG_DIR}/error.log"
    log "auto-ban 已清理过期黑名单条目"
}

scan_access_log_for_auto_ban() {
    access_log="${NGINX_LOG_DIR}/access.log"
    state_dir="$(auto_ban_state_dir)"
    offset_file="${state_dir}/access.offset"
    mkdir -p "$state_dir"

    [ -f "$access_log" ] || return 0

    size="$(wc -c <"$access_log" | tr -d ' ')"
    case "$size" in
        ''|*[!0-9]*)
            return 0
            ;;
    esac

    # 首次启动：从文件末尾开始，避免历史日志误封
    if [ ! -f "$offset_file" ]; then
        printf '%s\n' "$size" >"$offset_file"
        return 0
    fi

    offset="$(cat "$offset_file" 2>/dev/null || echo 0)"
    case "$offset" in
        ''|*[!0-9]*)
            offset=0
            ;;
    esac
    # 日志切割后文件变小，重置偏移
    if [ "$size" -lt "$offset" ]; then
        offset=0
    fi
    if [ "$size" -eq "$offset" ]; then
        return 0
    fi

    window="$NGINX_AUTO_BAN_WINDOW"
    threshold="$NGINX_AUTO_BAN_THRESHOLD"
    ttl="$NGINX_AUTO_BAN_TTL"
    case "$window" in ''|*[!0-9]*) window=300 ;; esac
    case "$threshold" in ''|*[!0-9]*) threshold=15 ;; esac
    case "$ttl" in ''|*[!0-9]*) ttl=86400 ;; esac
    if [ "$window" -lt 60 ]; then window=60; fi
    if [ "$threshold" -lt 1 ]; then threshold=15; fi

    now="$(date +%s)"
    # 只读新增字节
    dd if="$access_log" bs=1 skip="$offset" count="$((size - offset))" 2>/dev/null \
        | extract_login_attack_ips \
        | while IFS= read -r ip; do
            [ -n "$ip" ] || continue
            record_attack_and_maybe_ban "$ip" "$now" "$window" "$threshold" "$ttl"
        done

    printf '%s\n' "$size" >"$offset_file"
}

watch_auto_ban() {
    interval="$NGINX_AUTO_BAN_SCAN_INTERVAL"
    case "$interval" in
        ''|*[!0-9]*)
            interval=10
            ;;
    esac
    if [ "$interval" -lt 5 ]; then
        interval=5
    fi

    log "auto-ban 已开启，间隔 ${interval}s，窗口 ${NGINX_AUTO_BAN_WINDOW}s，阈值 ${NGINX_AUTO_BAN_THRESHOLD}，TTL ${NGINX_AUTO_BAN_TTL}s"
    while true; do
        sleep "$interval"
        expire_auto_bans || true
        scan_access_log_for_auto_ban || true
    done
}

start_services() {
    /app --configfile "$PIXIU_CONFIG_PATH" &
    pixiu_pid=$!

    nginx -g "daemon off;" &
    nginx_pid=$!

    watch_ip_lists &
    watcher_pid=$!

    watch_nginx_log_rotate &
    log_rotate_pid=$!

    auto_ban_pid=""
    if is_true "$NGINX_AUTO_BAN"; then
        watch_auto_ban &
        auto_ban_pid=$!
    fi

    cleanup() {
        trap - INT TERM EXIT
        if [ -n "$auto_ban_pid" ]; then
            kill -TERM "$pixiu_pid" "$nginx_pid" "$watcher_pid" "$log_rotate_pid" "$auto_ban_pid" 2>/dev/null || true
            wait "$auto_ban_pid" 2>/dev/null || true
        else
            kill -TERM "$pixiu_pid" "$nginx_pid" "$watcher_pid" "$log_rotate_pid" 2>/dev/null || true
        fi
        wait "$pixiu_pid" 2>/dev/null || true
        wait "$nginx_pid" 2>/dev/null || true
        wait "$watcher_pid" 2>/dev/null || true
        wait "$log_rotate_pid" 2>/dev/null || true
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
auto_ban_flag="off"
if is_true "$NGINX_IP_WHITELIST_ON"; then
    whitelist_flag="on"
fi
if is_true "$NGINX_IP_BLACKLIST_ON"; then
    blacklist_flag="on"
fi
if is_true "$NGINX_AUTO_BAN"; then
    auto_ban_flag="on"
fi

if is_true "$NGINX_ENABLE_SSL"; then
    log "starting services http=${NGINX_HTTP_PORT} https=${NGINX_HTTPS_PORT} login_rate=${NGINX_LOGIN_RATE} ip_whitelist=${whitelist_flag} ip_blacklist=${blacklist_flag} auto_ban=${auto_ban_flag} log_dir=${NGINX_LOG_DIR}"
else
    log "starting services http=${NGINX_HTTP_PORT} https=disabled login_rate=${NGINX_LOGIN_RATE} ip_whitelist=${whitelist_flag} ip_blacklist=${blacklist_flag} auto_ban=${auto_ban_flag} log_dir=${NGINX_LOG_DIR}"
fi

start_services
