# 手动升级

根据当初的安装方式选择对应步骤。升级只替换 pixiu 容器，mysql 和 `/etc/pixiu` 配置保持不动。

将下文中的镜像标签换成目标版本（当前示例为 `v2.0.2-beta.1`）。

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

# 参数与 install.md 保持一致，仅替换镜像版本
docker run -d --net host --restart=always --privileged=true \
  -v /etc/pixiu:/etc/pixiu \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --name pixiu \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

## 基于 containerd 安装（nerdctl）

若宿主当初是用 `deploy/containerd/run.sh`（nerdctl/containerd）部署的，升级步骤同理，只是把 `docker` 换成 `nerdctl`、`docker.sock` 换成 `containerd.sock`，并保留 runner 日志卷 `/var/lib/pixiu`：

```bash
# 1) 拉取新版本镜像（tag 换成目标版本）
sudo nerdctl -n default pull crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1

# 2) 删除旧容器
sudo nerdctl -n default rm -f pixiu

# 3) 按原参数重建（参数与 deploy/containerd/README.md 第 5 节一致，仅替换镜像 tag）
sudo nerdctl -n default run -d --restart=always --net host --privileged \
  -v /etc/pixiu:/etc/pixiu \
  -v /run/containerd/containerd.sock:/run/containerd/containerd.sock \
  -v /var/lib/pixiu:/var/lib/pixiu \
  --name pixiu \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

也可直接用脚本重建（脚本默认镜像 tag 见 `deploy/containerd/run.sh` 头部，或用 `PIXIU_IMAGE` 覆盖）：

```bash
sudo PIXIU_IMAGE=crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1 \
     bash deploy/containerd/run.sh --recreate
```

说明：`/etc/pixiu`（含 `runtime.cri` 等配置）与数据库数据不受影响；沿用旧配置即可，无需为本步改动运行时配置。
