#!/bin/bash
# 更新 pixiu-aio 容器镜像并重建容器。
#
# 默认使用 docker；宿主使用 containerd 时用 RUNTIME 切换到 nerdctl：
#   bash hack/update-image.sh                      # docker（默认）
#   sudo RUNTIME=nerdctl bash hack/update-image.sh # containerd / nerdctl
#
# 可用环境变量：
#   RUNTIME           docker | nerdctl（默认 docker）
#   IMAGE             镜像地址（默认 ccr.ccs.tencentyun.com/pixiucloud/pixiu-aio:latest）
#   CONTAINERD_SOCK   containerd socket（默认 /run/containerd/containerd.sock）
#   CONTAINERD_NS     containerd 命名空间（默认 default；禁止 k8s.io）
RUNTIME="${RUNTIME:-docker}"
IMAGE="${IMAGE:-ccr.ccs.tencentyun.com/pixiucloud/pixiu-aio:latest}"
CONTAINERD_SOCK="${CONTAINERD_SOCK:-/run/containerd/containerd.sock}"
CONTAINERD_NS="${CONTAINERD_NS:-default}"

function update() {
    if [ "${RUNTIME}" = "nerdctl" ]; then
        # containerd：挂载 containerd socket，其余参数与 docker 版对齐；
        # 额外挂载 /var/lib/pixiu 持久化 runner 容器日志（containerd 无原生日志流，重建容器不丢日志）
        nerdctl -n "${CONTAINERD_NS}" pull "${IMAGE}"
        nerdctl -n "${CONTAINERD_NS}" rm -f pixiu-aio
        sleep 3
        nerdctl -n "${CONTAINERD_NS}" run -d --net host --restart=always --privileged \
            -v /etc/pixiu:/etc/pixiu \
            -v "${CONTAINERD_SOCK}:${CONTAINERD_SOCK}" \
            -v /var/lib/pixiu:/var/lib/pixiu \
            --name pixiu-aio "${IMAGE}"
    else
        docker pull "${IMAGE}"
        docker rm -f pixiu-aio
        sleep 3
        docker run -d --net host --restart=always --privileged=true \
            -v /etc/pixiu:/etc/pixiu \
            -v /var/run/docker.sock:/var/run/docker.sock \
            --name pixiu-aio "${IMAGE}"
    fi
}

update
