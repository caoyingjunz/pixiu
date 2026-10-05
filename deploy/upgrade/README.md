# 手动升级

根据当初的安装方式选择对应步骤。升级只替换 pixiu 容器，mysql 和 `/etc/pixiu` 配置保持不动。

将下文中的镜像标签换成目标版本（当前示例为 `v2.0.2-beta.1`）。

> **存量升级注意：运行时缺省值变更**
>
> 本版本起 `runtime.cri` 缺省为 containerd，且 pixiu 启动时会连接宿主 containerd，连不上即**启动报错退出**（报错信息会提示检查 socket 路径或改配 `cri: docker`）；docker 运行时为懒连接，首次执行部署才报错。
> **docker 部署的存量环境**（docker-compose 与手动 `docker run`）升级前，请先在 `/etc/pixiu/config.yaml` 增加 `runtime.cri: docker`，否则 pixiu 会因连不上 containerd 启动失败。
> 宿主本就使用 containerd 的部署无需改动。

## 基于 docker-compose 安装

进入部署目录，先改 `docker-compose.yaml` 里 pixiu 的 image 标签，再拉镜像并重建容器：

```bash
cd /etc/pixiu

# 修改 pixiu 镜像版本
vim docker-compose.yaml
```

拉取镜像

```bash
# pixiu
docker pull crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

重建 pixiu

```bash
docker-compose up -d pixiu
```

验证

```bash
docker-compose ps
```

## 基于手动安装

手动安装是 `docker run` 启动的，需要先拉新镜像，再删掉旧容器后按原参数重新启动：

拉取镜像

```bash
# pixiu
docker pull crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

替换容器

```bash
docker stop pixiu
docker rm pixiu

# 参数与 README.md 保持一致，仅替换镜像版本
docker run -d --net host --restart=always --privileged=true \
  -v /etc/pixiu:/etc/pixiu \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --name pixiu \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

## 基于 containerd 安装（nerdctl）

若宿主当初是用 nerdctl/containerd 手工部署的，升级步骤同理，只是把 `docker` 换成 `nerdctl`，socket 挂载换成 containerd 状态目录（`-v /run/containerd:/run/containerd` 与 `-v /var/lib/containerd:/var/lib/containerd`，整目录共享；runner 日志已随 `/etc/pixiu` 持久化，无需独立日志卷）：

```bash
# 1) 拉取新版本镜像（tag 换成目标版本）
sudo nerdctl -n default pull crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1

# 2) 删除旧容器
sudo nerdctl -n default rm -f pixiu

# 3) 按下方参数重建（containerd 状态路径需以整目录共享；仅替换镜像 tag）
sudo nerdctl -n default run -d --restart=always --net host --privileged \
  -v /etc/pixiu:/etc/pixiu \
  -v /run/containerd:/run/containerd \
  -v /var/lib/containerd:/var/lib/containerd \
  --name pixiu \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

说明：`/etc/pixiu` 配置目录与数据库数据不受影响；沿用旧配置即可，无需为本步改动运行时配置。runner 日志新位置为 `/etc/pixiu/runner-logs`；历史日志如需保留，可一次性拷贝 `cp -a /var/lib/pixiu/runner-logs /etc/pixiu/runner-logs`（不拷贝则旧日志不再被读取）。
