# 自动升级

版本自发现功能默认开启：pixiu 主服务会定期探测新发布的版本。自动升级默认关闭：显式开启后才在页面提示升级、允许触发升级接口，并由独立的 pixiu-upgrade 进程负责升级 pixiu 容器。目前只支持手动安装的 pixiu 环境，不支持 docker-compose 和 kubernetes 的安装模式。

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
