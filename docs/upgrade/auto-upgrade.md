# 自动升级

版本自发现功能默认开启：pixiu 主服务会定期探测新发布的版本。自动升级默认关闭：显式开启后才在页面提示升级、允许触发升级接口，并由独立的 pixiu-upgrade 进程负责升级 pixiu 容器。支持手动安装和 docker-compose 安装的 pixiu 环境，不支持 kubernetes 安装模式。

## 配置

`upgrade` 段包含两个独立开关：

```yaml
upgrade:
  # 版本自发现开关，默认开启；设为 false 后主服务不再探测版本源
  version_discovery: true
  # 自动升级开关，默认关闭；设为 true 后页面才显示升级提示
  enabled: false
```

两个开关相互独立：`enabled: false` 不影响版本自发现，`version_discovery: false` 时也不会执行升级。

## 网络依赖

- pixiu 服务：需能访问外网的版本源（默认 `https://hub.pixiuio.com`），用于版本自发现。
- pixiu-upgrade：需能访问 MySQL（与主服务同一库，沿用 `mysql` 段配置）和镜像仓库，用于拉取目标镜像和版本升级。

## 部署

pixiu-upgrade 必须运行在 pixiu 容器之外，与 pixiu 在同一节点，且能访问 `/etc/pixiu`（读取同一份 config.yaml）、MySQL 与容器运行时 socket。

```bash
# docker
docker run -d --name pixiu-upgrade --restart=always --net host \
  -v /etc/pixiu:/etc/pixiu -v /var/run/docker.sock:/var/run/docker.sock \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu-upgrade:latest

# containerd
nerdctl run -d --name pixiu-upgrade --restart=always --net host --privileged=true \
  -v /etc/pixiu:/etc/pixiu -v /run/containerd:/run/containerd -v /var/lib/containerd:/var/lib/containerd \
  -v /var/lib/nerdctl:/var/lib/nerdctl \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu-upgrade:latest
```

升级后的 pixiu 容器由 nerdctl/docker 创建，日志分别用 `nerdctl -n default logs pixiu` / `docker logs pixiu` 查看。

### docker-compose 部署

`deploy/docker-compose/docker-compose.yaml` 中已包含 `pixiu-upgrade` 服务，随 `docker compose up -d` 一并启动，与 pixiu 共用宿主机的 `/etc/pixiu` 配置目录和 docker.sock。使用该方式需满足：

- `config.yaml` 放在宿主机 `/etc/pixiu/config.yaml`，pixiu 与 pixiu-upgrade 两个容器都挂载 `/etc/pixiu:/etc/pixiu`（仓库自带 compose 文件已是此配置）。
- `config.yaml` 中 `runtime.cri: docker`、`upgrade.enabled: true`。
- 升级器重建 pixiu 容器时使用内置的 docker 默认参数 `--net host --restart=always --privileged=true -v /etc/pixiu:/etc/pixiu -v /var/run/docker.sock:/var/run/docker.sock`，与 compose 中 pixiu 的端口映射等方式可能不同；若部署依赖自定义网络、端口映射或环境变量，请在 `config.yaml` 的 `upgrade.run_args_docker` 中显式写全，避免升级后行为变化。

> 配置目录务必保持为 `/etc/pixiu`。升级器不会读取 compose 文件，而是按上述默认参数重建容器；若把配置挂到自定义路径（如 `/data/pixiu`），新容器将读不到 `config.yaml`。

## 开启 / 关闭

默认状态：版本自发现开启、自动升级关闭。

- **开启自动升级**：在 `/etc/pixiu/config.yaml` 设置 `upgrade.enabled: true`，并部署 pixiu-upgrade 进程（该进程同样读取 `upgrade.enabled`，未开启时启动即退出）。
- **关闭版本自发现**（不再探测版本源）：设置 `upgrade.version_discovery: false`。
- **关闭自动升级**：设置 `upgrade.enabled: false`（接口返回 `enabled: false`，前端不显示升级提示，触发升级返回 409），并停止/移除 pixiu-upgrade 容器。

```yaml
upgrade:
  version_discovery: true   # 版本自发现，默认开启
  enabled: false            # 自动升级，默认关闭
```

升级长期停留在「升级中」时，参见 [FAQ-009 点击升级后一直显示「升级中」（未部署 pixiu-upgrade）](../faq/upgrade-stuck-pending.md)。
