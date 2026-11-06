# 自动升级

自动升级由两个进程协作完成：

- **pixiu 主服务**：内置版本检查循环，定期查询版本源并与当前运行镜像比对，把结果写入数据库；新版 pixiu 启动后还会确认运行版本并回写「升级完成」。
- **pixiu-upgrade**：独立进程，轮询数据库中的升级请求，执行镜像拉取、容器重建、健康检查与失败回滚。

升级状态以单行记录（`upgrade_records`，`id` 固定为 1）持久化，使用 `status` 字段表达完整状态机。

## 状态机

| status | 名称 | 含义 | 写入方 |
|---|---|---|---|
| 0 | 未升级 | 检查器发现 `latest > current`，等待用户确认 | pixiu 主服务（检查器 `SaveCheck`） |
| 1 | 待升级 | 用户确认升级，等待 upgrade 进程认领 | 主服务 API `POST /pixiu/upgrade`（`MarkPending`） |
| 2 | 升级中 | upgrade 进程原子认领成功并写入 `upgrade_id` | pixiu-upgrade（`Claim`，条件 1→2） |
| 3 | 升级完成 | 新版 pixiu 确认当前运行版本与目标版本一致 | 新版 pixiu 启动钩子（`MarkCompleted`，条件 2→3） |
| 4 | 升级失败 | 拉取/重建/健康检查失败（含回滚），`last_error` 记录原因 | pixiu-upgrade（`MarkFailed`，2/3→4）或启动自检（`ResetStale`，2→4） |

状态迁移全部通过带条件的原子 `UPDATE` 完成，只有 `RowsAffected == 1` 才算成功：

```
0 ──用户确认──> 1 ──upgrade 进程认领──> 2 ──新版 pixiu 回写──> 3
↑                                  │
└──────────发现更新版本─────────────┴──失败/崩溃恢复──> 4 ──用户重试──> 1
```

其他约束：

- 检查器在状态 1/2 期间只刷新 `last_checked_at`，不会覆盖状态与目标版本。
- 状态 4 → 0 表示检查器发现了更新的版本（`latest` 变化），此时清空 `upgrade_id` 与 `last_error`。
- 同一版本升级失败后，检查器保持状态 4，直到用户重试或出现更新的版本。
- 状态 3 → 4 是允许的：新版 pixiu 可能先回写 3，upgrade 进程随后的健康检查仍可能失败，此时按失败处理。

## pixiu 主服务配置

在 `/etc/pixiu/config.yaml` 中增加 `upgrade` 段（`enabled: true` 才会启动检查器）：

```yaml
upgrade:
  # 是否启用自动升级的版本检测（升级执行由独立 upgrade 进程负责）
  enabled: true
  # 版本号检查地址，支持 JSON（version/latest_version/tag + image）或纯文本
  version_url: https://console.cloud.pixiuio.com
  # 版本源未返回镜像地址时的镜像仓库前缀
  image_repository: crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu
  # 版本检测间隔，不低于 1 分钟
  interval: 6h
  # 待升级的 pixiu 容器名
  container_name: pixiu
  # upgrade 进程轮询数据库中升级请求的间隔，不低于 1 秒
  poll_interval: 5s
  # 重建后的健康检查地址，留空视为跳过健康检查
  health_check_url: http://127.0.0.1:8091/healthz
  # 单次健康检查超时
  health_check_timeout: 60s
  # 健康检查重试次数
  health_check_retries: 10

# 容器运行时（升级时重建 pixiu 容器使用）
runtime:
  # docker | containerd，默认 containerd
  cri: containerd
  # 宿主运行时 socket 裸路径，留空时 containerd 使用 /run/containerd/containerd.sock
  socket: /run/containerd/containerd.sock
  # containerd 下重建容器使用的 nerdctl 路径
  nerdctl: nerdctl
  # containerd 命名空间
  namespace: default
```

主服务还需要能访问版本源；版本检测依赖 CRI 查询当前容器镜像（懒加载，CRI 不可用时只记录 `last_error`，不影响服务运行）。

## pixiu-upgrade 配置（与主服务共用 config.yaml）

upgrade 进程与 pixiu 主服务共用配置文件 `/etc/pixiu/config.yaml`（`-config` 可覆盖路径），读取其中的 `upgrade`/`runtime`/`mysql` 三段；无需单独的 upgrade 配置文件。以下为 upgrade 进程关注的字段示例（可合并进同一份 config.yaml）：

```yaml
upgrade:
  enabled: true
  # 升级执行者标识，默认主机名；用于任务认领与状态回写校验，多实例部署时应保证唯一
  upgrade_id: pixiu-upgrade-01
  container_name: pixiu
  poll_interval: 5s
  health_check_url: http://127.0.0.1:8091/healthz
  health_check_timeout: 60s
  health_check_retries: 10

runtime:
  cri: containerd
  socket: /run/containerd/containerd.sock
  nerdctl: nerdctl
  namespace: default

mysql:
  host: 127.0.0.1
  port: 3306
  user: root
  password: Pixiu868686
  name: pixiu
```

`upgrade.enabled=false` 时 upgrade 进程直接退出。

## 部署要求

pixiu-upgrade **必须运行在 pixiu 容器之外**。升级过程会停止并删除名为 `container_name` 的 pixiu 容器再重新创建，upgrade 进程若运行在其中会连同自身一起被删除，导致升级中断。

运行 upgrade 进程的主机/容器需要：

- 挂载宿主 CRI socket 或目录，能创建/删除 pixiu 容器：
  - containerd：挂载 `/run/containerd` 与 `/var/lib/containerd`，并保证 `nerdctl` 可用
  - docker：挂载 `/var/run/docker.sock`
- 能访问 MySQL（与 pixiu 主服务相同的库）
- 能访问 `/etc/pixiu`（读取配置；重建容器时沿用原有挂载参数）
- 能访问版本源与镜像仓库（拉取新镜像）
- 与 pixiu 相同的 `health_check_url` 网络可达性

## 镜像构建与运行

- 镜像地址：`crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu-upgrade:latest`，版本 tag 为 `v2.0.2-beta.1`。
- GitHub Actions `build-upgrade-image.yml` 在 master 分支推送且 `cmd/upgrade/**`、`pkg/upgrade/**`、`pkg/controller/upgrade/**`、`pkg/db/**`、`pkg/util/runtime/**`、`cmd/app/config/**`、`docker/Dockerfile-upgrade`、`go.mod`、`go.sum` 等路径变化时，构建 linux/amd64 与 linux/arm64 镜像。
- 镜像内置 `upgrade` 二进制并默认读取 `/etc/pixiu/config.yaml`，可用 `-config` 覆盖；镜像未内置 `nerdctl`，containerd 运行时需挂载宿主 nerdctl（或基于本镜像自行派生）。

Docker 运行示例：

```bash
docker run -d --name pixiu-upgrade --restart=always --net host \
  -v /etc/pixiu:/etc/pixiu \
  -v /var/run/docker.sock:/var/run/docker.sock \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu-upgrade:latest
```

containerd 运行示例（nerdctl 路径按宿主实际调整）：

```bash
nerdctl run -d --name pixiu-upgrade --restart=always --net host --privileged=true \
  -v /etc/pixiu:/etc/pixiu \
  -v /run/containerd:/run/containerd \
  -v /var/lib/containerd:/var/lib/containerd \
  -v /usr/local/bin/nerdctl:/usr/local/bin/nerdctl \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu-upgrade:latest
```

覆盖配置路径时，在镜像名后追加 `-config /path/to/config.yaml` 即可。

## 升级窗口与回滚

- 用户确认升级后，API 返回成功即表示状态已迁移到 1；真正的容器重建在 upgrade 进程的下一次轮询（默认 5 秒）开始。
- 升级窗口内 pixiu 服务会短暂不可用（容器重建 + 健康检查），建议在业务低峰执行。
- 任一环节失败（拉取镜像、重建容器、健康检查）都会尝试回滚到升级前的镜像，并把状态置为 4、记录 `last_error`；用户可在修复后重试（4 → 1）。
- 成功重建后不会立即写状态 3：新版 pixiu 启动后会校验「当前运行镜像 tag / 镜像地址 / 编译版本」与目标版本是否一致，一致才回写 3。回写最多重试 10 次、间隔 3 秒。

## 崩溃恢复

- upgrade 进程启动时执行 `ResetStale`：把自己上次遗留的「升级中」（状态 2）记录置为失败（状态 4），并写入 `last_error` 说明进程重启。这样记录不会永久卡在 2，用户可以重新发起升级。
- 由于 `upgrade_id` 参与条件更新，其他实例不会误清理不属于自己的「升级中」记录。
- pixiu 主服务启动时如果发现自己是「升级中」的新容器，会在 60 秒内反复确认运行版本并回写状态 3；确认失败只记录告警，不影响服务启动。

## 接口

- `GET /pixiu/upgrade/status`：返回 `current_version`、`latest_version`、`image`、`upgrade_available`、`status`、`upgrading`（兼容字段，等价于 `status` 为 1 或 2）、`last_error`、`last_checked_at`。
- `POST /pixiu/upgrade`：管理员触发升级（0/4 → 1）。无可用升级或目标镜像为空返回 409「当前已是最新版本」；状态 1/2 返回 409「升级已在进行中」；并发下状态被改变返回 409「升级状态已变化，请刷新后重试」。
