# 使用 containerd 部署 pixiu

本目录提供不依赖 Docker 的部署方式：宿主使用 **containerd + nerdctl** 运行 pixiu（可选同时运行 mariadb）。
配套脚本 [run.sh](./run.sh)；Docker 方式仍见 [install.md](../../install.md)，两者互不影响。

## 1. 与 Docker 部署的差异

| 对比项 | Docker 部署 | containerd 部署 |
|---|---|---|
| 客户端 | `docker` | `nerdctl`（v2.x） |
| 宿主守护进程 | `dockerd` | `containerd`（1.7 / 2.x 均可） |
| 挂载的宿主 socket | `/var/run/docker.sock` | `/run/containerd/containerd.sock` |
| 容器重启策略 | `--restart=always`（dockerd 解释） | `--restart=always`（取值相同，由 containerd restart monitor 解释，见第 2 节） |
| 镜像命名空间 | 无该概念 | 有，默认 `default`；`k8s.io` 为 kubelet 专用 |
| 离线导入 | `docker load -i x.tar` | `nerdctl -n default load -i x.tar` |
| 数据卷挂载 | `/etc/pixiu` | `/etc/pixiu` + `/var/lib/pixiu`（持久化 runner 容器日志，见第 4 节） |

`--net host`、`--privileged`、`-v /etc/pixiu:/etc/pixiu`、`--name pixiu` 四项与 Docker 版**逐条一致**，容器名、配置目录、默认账号（admin/Pixiu123456!）均不变；containerd 版**额外多挂一个 `/var/lib/pixiu`** 用于持久化 runner 容器日志（原因见第 4 节末尾）。

## 2. nerdctl 的重启策略语义（结论）

**nerdctl 支持 `--restart`，不需要 systemd 兜底即可等价 Docker 的 `--restart=always`。** 依据如下：

- `nerdctl run` 提供 `--restart=(no|always|on-failure[:max-retries]|unless-stopped)`，默认 `no`。
  出处：nerdctl 官方 `docs/command-reference.md`（`nerdctl run` 小节）；源码 `cmd/nerdctl/container/container_run.go` 中该 flag 的注册与取值说明。
- 实现方式：策略写入容器上的 containerd 标签（`containerd.io/restart.policy`、`containerd.io/restart.status` 等），由 **containerd 内置的 restart monitor 插件**周期性 reconcile（默认间隔 10s，可配 `plugins.restart.interval`），跨全部 containerd 命名空间扫描。
  出处：containerd 1.7 `runtime/restart/monitor/monitor.go`；插件在官方二进制中默认编译进来（1.7 `cmd/containerd/builtins` 引入 `runtime/restart/monitor`，2.x 引入 `plugins/restart`）。
- 因此重启策略**不依赖 nerdctl 进程存活**：`nerdctl run -d` 返回后策略依然生效；containerd 重启、宿主重启后，期望状态为 running 的容器会被重新拉起。仅当容器被显式 `nerdctl stop` 时会被标记 `explicitly-stopped` 而不再自动拉起（与 Docker `unless-stopped` 的行为区分）。
- 例外：rootless 模式下宿主重启后自动拉起还需 `loginctl enable-linger <用户>`。

说明：本机（开发环境）未安装 nerdctl/containerd，上述结论来自 nerdctl 与 containerd 官方源码/文档，未在本机实测；实机部署时请用第 5 节的验证命令确认。

## 3. 前置条件

1. `containerd` 已安装并运行，socket 默认 `/run/containerd/containerd.sock`。
2. `nerdctl` 已安装（v2.x）且在 `PATH` 中；容器操作需要 root（或使用 sudo）。
3. 若使用桥接网络（`NET_MODE` 非 host），需 `/opt/cni/bin` 下存在 CNI 插件；默认 host 网络无需 CNI。
4. 已按 [install.md](../../install.md) 创建 `/etc/pixiu/config.yaml`（含数据库连接信息）。
5. 宿主 `/var/lib/pixiu` 目录可用（runner 容器日志固定落盘 `/var/lib/pixiu/runner-logs`，见第 4 节）。
6. 需要 pixiu 在本机创建部署 runner 容器时，宿主还需具备 containerd 运行容器所需的 snapshotter 与网络配置（kubez-ansible 相关镜像已就绪）。

## 4. 配置 pixiu：runtime 段

`/etc/pixiu/config.yaml` 顶层新增（**默认 containerd，不配置即使用 containerd**，下面为显式写法）：

```yaml
runtime:
  # 宿主容器运行时类型：docker / containerd，默认 containerd
  cri: containerd
  docker:
    host: ""                              # 留空沿用 docker 默认/环境变量
  containerd:
    address: /run/containerd/containerd.sock
    namespace: default                    # 默认 default；禁止写 k8s.io（会污染 kubelet 视图）
```

要点：

- `cri` 只影响 **pixiu 拉起部署 runner 容器**时使用哪种运行时；pixiu 自身跑在 docker 还是 containerd 上与此无关。
- `containerd.address` 必须与 `run.sh` 的 `CONTAINERD_SOCK` 指向同一路径，并且该路径要挂载进 pixiu 容器（见第 5 节）。
- `containerd.namespace` 不要用 `k8s.io`。pixiu 的 runner 镜像必须存在于该命名空间下（containerd 的镜像存储按命名空间隔离）。
- containerd 没有原生日志流，pixiu 会把 runner 容器的 stdout/stderr 写到固定目录 `/var/lib/pixiu/runner-logs`（不可配置），因此**必须把 `/var/lib/pixiu` 挂载进 pixiu 容器**，否则日志只写在容器可写层、容器重建即丢。`run.sh` 已默认挂载该路径。

## 5. 部署

一键部署（脚本自动做前置检查、幂等处理）：

```bash
# 仅启动 pixiu（数据库需已就绪）
sudo bash deploy/containerd/run.sh

# 同时启动 mariadb 与 pixiu
sudo bash deploy/containerd/run.sh --with-mysql

# 已存在同名容器时重建
sudo bash deploy/containerd/run.sh --recreate
```

脚本与 `install.md` 中 Docker 命令的等价关系（拆开手动执行时等价）：

```bash
# 1) 数据库（可选）
sudo nerdctl -n default run -d --restart=always --net host --privileged \
  --name mariadb -e MYSQL_ROOT_PASSWORD="Pixiu868686" -e MYSQL_DATABASE="pixiu" \
  ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7

# 2) pixiu（与 docker 版相比：socket 换成 containerd，并多挂 /var/lib/pixiu 持久化 runner 日志）
sudo nerdctl -n default run -d --restart=always --net host --privileged \
  -v /etc/pixiu:/etc/pixiu \
  -v /run/containerd/containerd.sock:/run/containerd/containerd.sock \
  -v /var/lib/pixiu:/var/lib/pixiu \
  --name pixiu \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

镜像与运行参数可用环境变量覆盖（默认值即上面两条命令中的真实地址）：

```bash
sudo PIXIU_IMAGE=<私仓>/pixiu/pixiu:v2.0.2-beta.1 \
     MYSQL_IMAGE=<私仓>/pixiu/mysql:5.7 \
     INSECURE_REGISTRY=1 \
     bash deploy/containerd/run.sh --with-mysql
```

非 host 网络（等价 docker 版 `-p 8080:80`）：

```bash
sudo NET_MODE=bridge PIXIU_PORT=8080 bash deploy/containerd/run.sh
```

验证：

```bash
sudo nerdctl -n default ps -a
sudo nerdctl -n default logs -f pixiu
curl -fsS http://127.0.0.1/healthz        # host 网络时
sudo nerdctl -n default inspect pixiu | grep -i restart   # 查看重启策略是否写入
ls -l /var/lib/pixiu/runner-logs          # 宿主机上可见 runner 容器日志（执行过部署后）
```

浏览器访问 `http://<宿主IP>:80`（host 网络）或 `http://<宿主IP>:8080`（桥接），默认账号 `admin/Pixiu123456!`。

## 6. 用 systemd 管理（可选方案）

若希望统一由 systemd 管理（依赖 `containerd.service`、集中日志、`systemctl status` 查看状态），可改用下面的 unit。**该方案与 `--restart=always` 二选一，不要同时使用**（同时使用会出现 systemd 与 containerd restart monitor 同时抢拉起的情况）。

模板内容（保存为 `/etc/systemd/system/pixiu.service`，`ExecStart` 中的 `nerdctl` 必须写绝对路径，用 `command -v nerdctl` 获取实际路径）：

```ini
[Unit]
Description=Pixiu server (nerdctl / containerd)
After=containerd.service network-online.target
Wants=network-online.target
Requires=containerd.service

[Service]
Type=simple
# 清理上次残留的同名容器；前缀 "-" 表示失败不阻断启动
ExecStartPre=-/usr/local/bin/nerdctl --namespace default rm -f pixiu
ExecStart=/usr/local/bin/nerdctl --namespace default run --rm --name pixiu \
  --net host --privileged \
  -v /etc/pixiu:/etc/pixiu \
  -v /run/containerd/containerd.sock:/run/containerd/containerd.sock \
  -v /var/lib/pixiu:/var/lib/pixiu \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
ExecStop=/usr/local/bin/nerdctl --namespace default stop pixiu
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

安装与启用：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now pixiu
systemctl status pixiu
journalctl -u pixiu -f
```

说明：`--rm` 使容器退出后即被删除，由 systemd `Restart=always` 重新创建，因此该模板下**不要**再传 `--restart=always`。

## 7. 卸载

```bash
sudo nerdctl -n default rm -f pixiu
sudo nerdctl -n default rm -f mariadb      # 使用过 --with-mysql 时
sudo nerdctl -n default rmi <pixiu镜像>    # 可选：删除镜像
```

配置目录 `/etc/pixiu` 与数据库数据不受影响；数据库若由本机容器提供，删容器即丢数据，请先备份。若不再需要 runner 日志，可自行清理宿主 `/var/lib/pixiu/runner-logs`（本脚本不自动删除）。

## 8. 与 pixiu 服务端的联动（重要）

pixiu 服务端在「创建部署」时，需要在本机拉起 runner 容器（kubez-ansible）。此时使用哪种运行时由 `/etc/pixiu/config.yaml` 的 `runtime.cri` 决定：

- `runtime.cri` 缺省即 `containerd`，所以**不写 runtime 段（或只写 `cri: containerd`）时，pixiu 就会用 containerd 拉起 runner**。
- 仅当**显式**配置 `cri: docker` 时，pixiu 才会去调用宿主 docker（宿主没有 docker 则 runner 创建失败）。
- 用 containerd 时，pixiu 通过 `containerd.address` 连 socket、在 `containerd.namespace` 中创建 runner 容器；**该 socket 必须被挂载进 pixiu 容器**（run.sh 已包含），否则容器内 `/run/containerd/containerd.sock` 不存在，创建失败。
- runner 镜像（如 kubez-ansible）需要预先存在于 `containerd.namespace` 指定的命名空间下；若宿主同时运行 Kubernetes，kubelet 使用的 `k8s.io` 命名空间中的镜像**不会**被 pixiu 直接复用，需另外导入（见 [deploy/offline/README.md](../offline/README.md)）。
- runner 容器日志固定写入 `/var/lib/pixiu/runner-logs`，该目录需由 `/var/lib/pixiu` 卷持久化（run.sh 已包含），否则 pixiu 重启/容器重建后日志丢失。

## 9. 常见问题

| 现象 | 原因与处理 |
|---|---|
| `错误: 未找到 containerd socket /run/containerd/containerd.sock` | containerd 未运行，或 socket 路径不同；用 `systemctl status containerd` 确认，必要时用 `CONTAINERD_SOCK` 覆盖 |
| `permission denied ... containerd.sock` | 需要 root（sudo）；非 root 用户需具备 socket 读权限 |
| `未找到 nerdctl` | 安装 nerdctl v2.x 并放入 `PATH`（发布包解压后放到 `/usr/local/bin`） |
| 镜像明明导入过却 `not found` | 镜像在别的 containerd 命名空间；用 `nerdctl -n default images` 确认，导入时指定同一 `-n` |
| 拉取私仓镜像报 TLS/HTTPS 错误 | 私仓为 HTTP 或自签证书时，加全局 `--insecure-registry`（脚本设 `INSECURE_REGISTRY=1`） |
| 桥接网络启动失败（CNI 相关报错） | 宿主缺少 `/opt/cni/bin` 插件；或直接使用默认 host 网络 |
| 容器退出后没有自动拉起 | 确认启动时带了 `--restart=always`（`nerdctl -n default inspect <name> \| grep -i restart`）；被 `nerdctl stop` 显式停止的容器不会自动拉起 |
| 创建部署时报无 runtime | `config.yaml` 显式写了 `cri: docker`，或 containerd socket 未挂载进 pixiu 容器 |
| pixiu 重启/容器重建后查不到 runner 日志 | 未挂载 `/var/lib/pixiu` 卷（日志只写在容器可写层）；按第 4 节确认 `/var/lib/pixiu` 已挂载（日志固定写在其下 runner-logs 目录） |

## 10. 参考

- nerdctl 命令参考（`run` 的 `--restart/--net/--privileged`、`save`/`load`、全局 `--namespace/--address/--insecure-registry`）：nerdctl 仓库 `docs/command-reference.md`
- containerd 重启监控插件：containerd 1.7 `runtime/restart/monitor/monitor.go`、2.x `plugins/restart`
- 离线部署（含 containerd 环境差异、`nerdctl save/load` 导入导出命令）：[deploy/offline/README.md](../offline/README.md)
