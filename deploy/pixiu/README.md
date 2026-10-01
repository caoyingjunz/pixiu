# Pixiu Kubernetes 安装

## 前置

- 可用的 Kubernetes 集群与 `kubectl`
- 可用的 MySQL（需先建库 `pixiu`）
- 能拉取镜像（或已提前导入）：
  - `crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1`

## 1. 准备 MySQL

任选其一。

### 方式 A：已有数据库

```sql
CREATE DATABASE pixiu;
```

### 方式 B：Docker 快速启动

```bash
docker run -d --net host --restart=always --privileged=true \
  --name mariadb \
  -e MYSQL_ROOT_PASSWORD="Pixiu868686" \
  -e MYSQL_DATABASE="pixiu" \
  ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7
```

宿主机访问库地址一般为节点 IP，端口 `3306`。

此时 ConfigMap 中可将 `mysql.host` 设为 `pixiu-mysql`。

## 2. 配置并安装 Pixiu

1. 编辑 `pixiu.yaml` 中 ConfigMap 的 `mysql.host/user/password/port/name`，指向上一步数据库
2. 部署：

```bash
kubectl apply -f deploy/pixiu/pixiu.yaml
```

## 已知限制：无法由 pixiu 在本机拉起部署 runner

本清单只在容器内挂载 `/etc/pixiu` 配置卷，**未挂载任何宿主容器运行时 socket**（`/var/run/docker.sock` 或 `/run/containerd/containerd.sock`），因此：

- 该形态下 pixiu 不支持「本地执行模式」（由 pixiu 自身在本机拉起 runner 容器执行部署）。
- 需要本地执行时，请使用 Plan 的 **agent 执行模式**：由目标节点上运行的 deploy-agent 执行部署流程。
- 确需本地执行模式时，可自行在 Deployment 中挂载宿主运行时 socket（containerd 形态为 `/run/containerd/containerd.sock`）并补足相应权限；注意挂载运行时 socket 等同于把节点控制权交给 Pod，请先评估安全影响。

## 服务端 runtime 配置

ConfigMap 中的 `runtime` 段决定 pixiu「在本机拉起部署 runner 容器」时使用哪种宿主运行时（与被部署集群的 CRI 不是一回事）：

- `runtime.cri`：`docker` / `containerd`，**默认 containerd**，不配置即使用 containerd
- `runtime.socket`：宿主运行时 socket 路径，只支持裸路径（不要写 unix:// 前缀）；留空时 containerd 使用 `/run/containerd/containerd.sock`，docker 沿用 `DOCKER_HOST`/默认 socket
- `runtime.containerd.namespace`：containerd 命名空间，默认 `default`；禁止使用 `k8s.io`（会污染 kubelet 视图）

本清单显式配置 `cri: docker`。宿主直接用 containerd 部署 pixiu 本身的完整步骤见 [deploy/containerd/README.md](../containerd/README.md)。

## 访问

```bash
kubectl -n pixiu-system get svc pixiu
```

浏览器打开：`http://<节点IP>:<NodePort>`
默认账号（与 install.md 一致，可在 ConfigMap 修改）：

- 用户名：`admin`
- 密码：`Pixiu123456!`
