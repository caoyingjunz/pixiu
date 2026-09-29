#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# deploy/containerd/run.sh
# 使用 nerdctl(containerd) 一键部署 pixiu（可选同时启动 mariadb）。
#
# 与 install.md 中 docker 部署参数逐条对应：
#   docker run                              nerdctl run
#   -d                                      -d
#   --restart=always                        --restart=always（nerdctl 原生支持，见 README）
#   --net host                              --net host
#   --privileged=true                       --privileged
#   -v /etc/pixiu:/etc/pixiu                -v /etc/pixiu:/etc/pixiu
#   -v /var/run/docker.sock:/var/run/...    -v /run/containerd/containerd.sock:...（关键差异）
#   （无）                                  -v /var/lib/pixiu:/var/lib/pixiu（持久化 runner 容器日志）
#   --name pixiu                            --name pixiu
#
# 用法:
#   sudo bash run.sh                仅启动 pixiu（数据库需已就绪）
#   sudo bash run.sh --with-mysql   同时启动 mariadb 与 pixiu
#   sudo bash run.sh --recreate     已存在同名容器时先删除再重建
#   sudo bash run.sh --help         查看帮助
#
# 可用环境变量覆盖默认值（默认与 install.md 一致）：
#   PIXIU_IMAGE          pixiu 镜像
#   MYSQL_IMAGE          mariadb 镜像
#   PIXIU_NAME           pixiu 容器名（默认 pixiu）
#   MYSQL_NAME           mariadb 容器名（默认 mariadb）
#   CONFIG_DIR           配置目录（默认 /etc/pixiu）
#   LOG_DIR              runner 日志宿主目录（默认 /var/lib/pixiu，应包含 config 的 runtime.log_dir）
#   CONTAINERD_SOCK      containerd socket（默认 /run/containerd/containerd.sock）
#   CONTAINERD_NS        containerd 命名空间（默认 default；禁止 k8s.io）
#   RESTART_POLICY       重启策略（默认 always，可选 no|always|on-failure:n|unless-stopped）
#   NET_MODE             网络模式（默认 host；非 host 时按 PIXIU_PORT 做端口映射）
#   PIXIU_PORT           NET_MODE 非 host 时映射的宿主机端口（默认 8080，对应容器 80）
#   MYSQL_ROOT_PASSWORD  mariadb root 密码（默认 Pixiu868686）
#   MYSQL_DATABASE       初始化数据库名（默认 pixiu）
#   NERDCTL              nerdctl 可执行文件（默认 nerdctl）
#   INSECURE_REGISTRY    置 1 时访问 HTTP/自签私仓（追加全局 --insecure-registry）
# ---------------------------------------------------------------------------
set -euo pipefail

PIXIU_IMAGE="${PIXIU_IMAGE:-crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1}"
MYSQL_IMAGE="${MYSQL_IMAGE:-ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7}"
PIXIU_NAME="${PIXIU_NAME:-pixiu}"
MYSQL_NAME="${MYSQL_NAME:-mariadb}"
CONFIG_DIR="${CONFIG_DIR:-/etc/pixiu}"
LOG_DIR="${LOG_DIR:-/var/lib/pixiu}"
CONTAINERD_SOCK="${CONTAINERD_SOCK:-/run/containerd/containerd.sock}"
CONTAINERD_NS="${CONTAINERD_NS:-default}"
RESTART_POLICY="${RESTART_POLICY:-always}"
NET_MODE="${NET_MODE:-host}"
PIXIU_PORT="${PIXIU_PORT:-8080}"
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-Pixiu868686}"
MYSQL_DATABASE="${MYSQL_DATABASE:-pixiu}"
NERDCTL="${NERDCTL:-nerdctl}"
INSECURE_REGISTRY="${INSECURE_REGISTRY:-0}"

CONFIG_FILE="${CONFIG_DIR}/config.yaml"
WITH_MYSQL=0
RECREATE=0

usage() {
  # 打印文件头注释（第 3 行起，直到下一条分隔线）
  awk 'NR>2 { if ($0 ~ /^# -{5,}$/) exit; sub(/^# ?/, ""); print }' "$0"
}

log() {
  echo "[pixiu-containerd] $*"
}

die() {
  echo "[pixiu-containerd] 错误: $*" >&2
  exit 1
}

for arg in "$@"; do
  case "$arg" in
    --with-mysql) WITH_MYSQL=1 ;;
    --recreate) RECREATE=1 ;;
    -h|--help) usage; exit 0 ;;
    *) die "未知参数: ${arg}（可用: --with-mysql --recreate --help）" ;;
  esac
done

[ "${EUID}" -eq 0 ] || die "需要 root 权限操作 containerd，请使用 sudo 运行本脚本"

command -v "${NERDCTL}" >/dev/null 2>&1 || die "未找到 ${NERDCTL}，请先安装 nerdctl 并确保在 PATH 中"

[ -S "${CONTAINERD_SOCK}" ] || die "未找到 containerd socket ${CONTAINERD_SOCK}（containerd 未运行？或路径不同，可用 CONTAINERD_SOCK 覆盖）"

[ "${CONTAINERD_NS}" = "k8s.io" ] && die "禁止使用 k8s.io 命名空间：该命名空间为 kubelet 专用，写入会污染 kubelet 视图"

[ -f "${CONFIG_FILE}" ] || die "未找到配置文件 ${CONFIG_FILE}，请先按 install.md 创建（含 mysql 连接信息与 runtime 段）"

# 全局参数：显式指定 containerd socket 与命名空间，避免落到错误的 containerd 实例/命名空间
NERDCTL_GLOBAL=(--address "${CONTAINERD_SOCK}" --namespace "${CONTAINERD_NS}")
if [ "${INSECURE_REGISTRY}" = "1" ]; then
  NERDCTL_GLOBAL+=(--insecure-registry)
fi

"${NERDCTL}" "${NERDCTL_GLOBAL[@]}" info >/dev/null 2>&1 \
  || die "无法连接 containerd（${CONTAINERD_SOCK}），请确认 containerd 正在运行且 socket 可访问"

container_exists() {
  local names
  names="$("${NERDCTL}" "${NERDCTL_GLOBAL[@]}" ps -a --format '{{.Names}}' 2>/dev/null || true)"
  printf '%s\n' "${names}" | grep -qx "$1"
}

remove_container() {
  log "移除已存在的容器 ${1}"
  "${NERDCTL}" "${NERDCTL_GLOBAL[@]}" rm -f "$1" >/dev/null
}

# --- 前置提示：服务端运行时配置 ---------------------------------------------
if grep -Eq '^[[:space:]]*cri:[[:space:]]*"?docker"?([[:space:]]|$)' "${CONFIG_FILE}"; then
  log "警告: ${CONFIG_FILE} 显式配置了 runtime.cri=docker，pixiu 拉起部署 runner 容器时走 docker。"
  log "      如需在本机使用 containerd 拉起 runner，请将该配置改为 containerd（或删除该行使用默认值）后重启 pixiu："
  log "        runtime:"
  log "          cri: containerd"
  log "          containerd:"
  log "            address: ${CONTAINERD_SOCK}"
  log "            namespace: ${CONTAINERD_NS}"
else
  log "配置检查: runtime.cri=containerd（显式配置或默认值）"
fi

# --- mariadb（可选） ---------------------------------------------------------
if [ "${WITH_MYSQL}" = "1" ]; then
  if container_exists "${MYSQL_NAME}"; then
    log "已存在容器 ${MYSQL_NAME}，跳过数据库启动"
  else
    log "启动 mariadb 容器 ${MYSQL_NAME}（镜像 ${MYSQL_IMAGE}）"
    "${NERDCTL}" "${NERDCTL_GLOBAL[@]}" run -d \
      --restart "${RESTART_POLICY}" \
      --net host \
      --privileged \
      --name "${MYSQL_NAME}" \
      -e "MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD}" \
      -e "MYSQL_DATABASE=${MYSQL_DATABASE}" \
      "${MYSQL_IMAGE}"
  fi
fi

# --- pixiu ------------------------------------------------------------------
if container_exists "${PIXIU_NAME}"; then
  if [ "${RECREATE}" = "1" ]; then
    remove_container "${PIXIU_NAME}"
  else
    die "已存在容器 ${PIXIU_NAME}，如需重建请加 --recreate（或先执行: ${NERDCTL} -n ${CONTAINERD_NS} rm -f ${PIXIU_NAME}）"
  fi
fi

PIXIU_NET_ARGS=(--net host)
if [ "${NET_MODE}" != "host" ]; then
  PIXIU_NET_ARGS=(--net "${NET_MODE}" -p "${PIXIU_PORT}:80")
  log "网络模式 ${NET_MODE}：宿主机端口 ${PIXIU_PORT} 映射到容器 80"
fi

log "启动 pixiu 容器 ${PIXIU_NAME}（镜像 ${PIXIU_IMAGE}，namespace ${CONTAINERD_NS}）"
"${NERDCTL}" "${NERDCTL_GLOBAL[@]}" run -d \
  --restart "${RESTART_POLICY}" \
  "${PIXIU_NET_ARGS[@]}" \
  --privileged \
  --name "${PIXIU_NAME}" \
  -v "${CONFIG_DIR}:${CONFIG_DIR}" \
  -v "${CONTAINERD_SOCK}:${CONTAINERD_SOCK}" \
  -v "${LOG_DIR}:${LOG_DIR}" \
  "${PIXIU_IMAGE}"

log "启动完成。查看状态与日志："
log "  ${NERDCTL} -n ${CONTAINERD_NS} ps -a"
log "  ${NERDCTL} -n ${CONTAINERD_NS} logs -f ${PIXIU_NAME}"
log "浏览器访问: http://<宿主机IP>:$([ "${NET_MODE}" = "host" ] && echo 80 || echo "${PIXIU_PORT}")（默认账号 admin/Pixiu123456!）"
