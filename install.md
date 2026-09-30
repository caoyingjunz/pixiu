# 前置准备
```bash
确保 docker 已经安装
注意: pixiu 和 kubernetes 集群复用节点的时候, 在ubuntu系统上通过 apt 的方式直接安装 docker. 安装方式推荐如下:
```
[docker 快速安装](./deploy/offline/docker.md)

> 不安装 docker、直接使用宿主 containerd 部署 pixiu，见文末[使用 containerd 部署](#使用-containerd-部署可选)。

# 数据库
```bash
# 选择1：直接提供可用数据库，初始化 pixiu 数据库（CREATE DATABASE pixiu;）

# 选择2：快速启动数据库，并初始化 pixiu 数据库（生产环境自行部署或者使用高可用数据库）
docker run -d --net host --restart=always --privileged=true --name mariadb -e MYSQL_ROOT_PASSWORD="Pixiu868686" -e MYSQL_DATABASE="pixiu" ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7
```

# 获取部署驱动镜像（可选，如果没有部署k8s需求，或者可联网可跳过，pixiu 部署时会自行同步 runner）
```shell
docker pull ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v2.0.2
docker pull ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v3.0.4
```

# 启动 pixiu 服务端
## 配置 pixiu
```bash
# 创建配置文件夹
mkdir -p /etc/pixiu/
# 后端配置(host 根据实际情况调整)
vim /etc/pixiu/config.yaml 写入后端如下配置

### 配置文件内容
default:
  # 自动创建指定模型的数据库表结构，不会更新已存在的数据库表
  auto_migrate: true

  # 超级管理初始化用户名和密码；不指定的情况下，默认为 admin/Pixiu123456!
  admin_user: admin
  admin_password: Pixiu123456!

# 数据库地址信息, 根据实际情况配置
mysql:
  host: pixiu # 数据库的ip
  user: root
  password: Pixiu868686
  port: 3306
  name: pixiu
```

### 容器运行时配置（可选）
pixiu 会在部署节点上拉起 runner 容器（kubez-ansible），其使用的容器运行时由以下配置决定；**默认 containerd，不配置即使用 containerd**：

```yaml
runtime:
  # 宿主容器运行时类型：docker / containerd，默认 containerd
  # 本文是 docker 部署方式，故显式写 docker；宿主改用 containerd 时删掉本行（缺省即 containerd）
  cri: docker
  #docker:                                # 仅当 cri: docker 时生效
  #  host: ""                              # 留空沿用 docker 默认/环境变量
  #containerd:                            # cri: containerd 时的可选项，整段不写即用下列默认值
  #  address: /run/containerd/containerd.sock
  #  namespace: default                    # 默认 default；禁止写 k8s.io（会污染 kubelet 视图）
```

说明：宿主用 docker 部署 pixiu 时须显式写 `cri: docker`（缺省值已改为 containerd）；宿主用 containerd 时保持 `cri: containerd`（默认值，可整段不写），此时的 socket 路径、命名空间与日志目录挂载要求见 [deploy/containerd/README.md](./deploy/containerd/README.md)。

## 启动 pixiu
```bash
# 根据实际需要修改宿主机端口，默认使用宿主机端口，可替换 --net host 为期望端口映射 -p <hostPort>:80
docker run -d --net host --restart=always --privileged=true -v /etc/pixiu:/etc/pixiu -v /var/run/docker.sock:/var/run/docker.sock --name pixiu crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

## 登陆 pixiu
```
# 根据配置文件中指定的账密输入；如果未指定默认用户名密码是 admin/Pixiu123456!
浏览器登陆: http://<ip>:<port>
```

# 使用 containerd 部署（可选）

宿主已运行 containerd（未安装 docker，或希望 pixiu 与 k8s 共用 containerd）时，用 nerdctl 替代 docker，参数与上方 docker 章节一一对应，**唯一差异是把 `/var/run/docker.sock` 换成 `/run/containerd/containerd.sock`**。

前置条件详见 [deploy/containerd/README.md](./deploy/containerd/README.md)：containerd 运行中、nerdctl（v2.x）已安装、操作需 root。

```bash
# 数据库（可选，与 docker 版参数一致）
nerdctl -n default run -d --restart=always --net host --privileged=true --name mariadb -e MYSQL_ROOT_PASSWORD="Pixiu868686" -e MYSQL_DATABASE="pixiu" ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7

# 启动 pixiu（-v /var/lib/pixiu 用于持久化 runner 容器日志，避免容器重建后丢失）
nerdctl -n default run -d --restart=always --net host --privileged=true -v /etc/pixiu:/etc/pixiu -v /run/containerd/containerd.sock:/run/containerd/containerd.sock -v /var/lib/pixiu:/var/lib/pixiu --name pixiu crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

也可直接使用一键脚本（含前置检查、幂等处理、`--with-mysql` 可选起库）：

```bash
sudo bash deploy/containerd/run.sh --with-mysql
```

注意：

- `-n default` 是 nerdctl 的 containerd 命名空间，必须与 `/etc/pixiu/config.yaml` 中 `runtime.containerd.namespace` 一致，且**不要写 k8s.io**。
- 配置文件中的 `runtime.cri` 保持默认 `containerd`（见上方「容器运行时配置」，缺省即 containerd），pixiu 才会用 containerd 拉起部署 runner 容器。
- `-v /var/lib/pixiu:/var/lib/pixiu` 用于持久化 runner 容器日志（固定写入 `/var/lib/pixiu/runner-logs`），请保持该挂载。
- 验证与卸载命令见 [deploy/containerd/README.md](./deploy/containerd/README.md)。
