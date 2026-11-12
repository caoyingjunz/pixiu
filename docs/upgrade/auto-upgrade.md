# 自动升级

pixiu 会检测新版本，独立的 pixiu-upgrade 负责拉取镜像并重建 pixiu 容器。

## 依赖

- 从 v2.0.2-beta.2 默认开启自动更新功能
- 能访问外网，用于获取版本源（不外发任何数据）

## 部署

pixiu-upgrade 容器必须运行在 pixiu 所在节点上。

```bash
# docker
docker run -d --name pixiu-upgrade --restart=always --net host \
  -v /etc/pixiu:/etc/pixiu -v /var/run/docker.sock:/var/run/docker.sock \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu-upgrade:latest

# containerd
nerdctl run -d --name pixiu-upgrade --restart=always --net host --privileged=true \
  -v /etc/pixiu:/etc/pixiu -v /run/containerd:/run/containerd -v /var/lib/containerd:/var/lib/containerd \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu-upgrade:latest
```

升级后的 pixiu 容器日志写在 `/etc/pixiu/runner-logs/<容器名>.log`（`nerdctl logs` 不适用），可用 `tail -f` 或 pixiu 页面查看。镜像 tag 与 `docs/upgrade/README.md` 中的版本保持一致。

## 关闭自动升级

在 `/etc/pixiu/config.yaml` 设置：

```yaml
upgrade:
  enabled: false
```

不再探测版本（接口返回 `enabled: false`，触发升级返回 409）；同时停止并移除 pixiu-upgrade 容器（不部署该进程时只做版本检测，不会执行升级）。
