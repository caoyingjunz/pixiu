# 自动升级

开启后，pixiu 会检测新版本，独立的 pixiu-upgrade 负责升级 pixiu 容器，目前只支持手动安装的 pixiu 环境，不支持 docker-compose 和 kubernetes 的安装模式。

## 网络依赖

- pixiu 服务：需要能连通外网的 `https://hub.pixiuio.com` 地址，用于自动获取版本信息
- pixiu-upgrade：需要和 pixiu 部署在同一台节点。

## 部署

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

## 关闭自动升级

在 `/etc/pixiu/config.yaml` 设置：

```yaml
upgrade:
  enabled: false
```

不再探测版本（接口返回 `enabled: false`，触发升级返回 409）；同时停止并移除 pixiu-upgrade 容器（不部署该进程时只做版本检测，不会执行升级）。
