# 自动升级

自动升级由两个进程协作完成：

- **pixiu 主服务**：内置版本检查循环，定期查询版本源并与当前运行镜像比对，把结果写入数据库；不参与升级执行与完成回写。
- **pixiu-upgrade**：独立进程，轮询数据库中的升级请求，执行镜像拉取与容器重建；容器重建后稳定运行时由它原子回写「升级完成」并把 `is_current` 切到新版本。

升级状态以版本记录（`server_versions`，每个 version+commit 一行，历史全部保留）持久化，使用 `status` 字段表达完整状态机；业务层按 `id` 降序取最新一条。

## 状态机

| status | 名称 | 含义 | 写入方 |
|---|---|---|---|
| 0 | 未升级 | 检查器发现数据库中不存在的新版本，等待用户确认 | pixiu 主服务（检查器 `Create`） |
| 1 | 待升级 | 用户确认升级，等待 upgrade 进程认领 | 主服务 API `POST /pixiu/upgrade`（`MarkPending`） |
| 2 | 升级中 | upgrade 进程原子认领成功并写入 `upgrade_id` | pixiu-upgrade（`Claim`，条件 1→2） |
| 3 | 升级完成 | pixiu-upgrade 确认新容器稳定运行后回写完成，并在同一事务内把 `is_current` 切到新版本 | pixiu-upgrade（`MarkCompleted`，条件 2→3） |
| 4 | 升级失败 | 拉取/重建失败，或新容器在启动观察窗口内退出，`error_message` 记录原因 | pixiu-upgrade（`MarkFailed`，2/3→4）或启动自检（`Reset`，2→4） |

状态迁移全部通过带条件的原子 `UPDATE` 完成，只有 `RowsAffected == 1` 才算成功：

```
0 ──用户确认──> 1 ──upgrade 进程认领──> 2 ──容器稳定后 upgrade 进程回写──> 3
↑                                  │
└──────────发现更新版本─────────────┴──失败/崩溃恢复──> 4 ──用户重试──> 1
```

其他约束：

- 每个 version+commit 对应一条 server_versions 记录，历史全部保留；状态接口返回最新一条（id 最大）。
- 检查器只新增版本记录，不修改已有记录的状态。
- 检查器发现新的 version+commit 时新增一条记录，历史记录（含状态 4）保持不变。
- 同一版本升级失败后，该条记录保持状态 4，直到用户重试。
- 新发现的 version+commit 直接入库并置 status=0；不再读取运行容器镜像。

## pixiu 主服务配置

在 `/etc/pixiu/config.yaml` 中增加 `upgrade` 段（未配置 `enabled` 时默认开启版本检测）：

```yaml
upgrade:
  # 是否启用自动升级的版本检测（默认开启，显式 false 关闭；升级执行由独立 upgrade 进程负责）
  enabled: true
  # 版本源基础地址，实际请求 = version_url + version_route；
  # JSON 响应需包含 code/result，result 内提供 version、image、commit、release_time（可选）
  # HTTP 200 但业务 code!=200、响应非法 JSON 或缺少 result 时直接报错；commit 缺失同样报错
  # release_time 支持 RFC3339、"2006-01-02 15:04:05"、"2006-01-02" 三种格式
  version_url: https://hub.pixiuio.com
  # 版本号检查的具体路由（version_url 只填基础地址）
  version_route: /api/v2/pixiu/versions/latest
  # 版本源未返回镜像地址时的镜像仓库前缀
  image_repository: crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu
  # 版本检测间隔，默认 10s，不低于 1 秒
  interval: 10s
  # 待升级的 pixiu 容器名
  container_name: pixiu
  # upgrade 进程轮询数据库中升级请求的间隔，不低于 1 秒
  poll_interval: 5s

# 容器运行时（升级时重建 pixiu 容器使用）
runtime:
  # docker | containerd，默认 containerd
  cri: containerd
  # 宿主运行时 socket 裸路径，留空时 containerd 使用 /run/containerd/containerd.sock
  socket: /run/containerd/containerd.sock
```

拨测时会在 version_route 上附加 `?current_version=<is_current 记录 version_tag>&current_commit=<编译版本 commit>`。

主服务未配置 `upgrade.enabled` 时默认开启版本检测；`pixiu-upgrade` 独立进程仍要求显式配置 `enabled: true`。

主服务还需要能访问版本源；版本检测依赖 CRI 查询当前容器镜像（懒加载，CRI 不可用时只记录 `error_message`，不影响服务运行）。

## pixiu-upgrade 配置（与主服务共用 config.yaml）

upgrade 进程与 pixiu 主服务共用配置文件 `/etc/pixiu/config.yaml`（`-config` 可覆盖路径），读取其中的 `upgrade`/`runtime`/`mysql` 三段；无需单独的 upgrade 配置文件。以下为 upgrade 进程关注的字段示例（可合并进同一份 config.yaml）：

```yaml
upgrade:
  enabled: true
  # 升级执行者标识，默认主机名；用于任务认领与状态回写校验，多实例部署时应保证唯一
  upgrade_id: pixiu-upgrade-01
  container_name: pixiu
  poll_interval: 5s

runtime:
  cri: containerd
  socket: /run/containerd/containerd.sock

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
  - containerd：挂载 `/run/containerd`、`/var/lib/containerd`（直接使用 containerd 客户端重建，无需安装 nerdctl）；同时挂载 `/var/lib/nerdctl`，升级重建后会同步维护 nerdctl 状态与日志，保证 `nerdctl logs` 可用（未挂载时升级仍可执行，但升级后的容器无法用 `nerdctl logs` 查看）
  - docker：挂载 `/var/run/docker.sock`
- 能访问 MySQL（与 pixiu 主服务相同的库）
- 能访问 `/etc/pixiu`（读取配置；重建容器时沿用原有挂载参数）
- 能访问版本源与镜像仓库（拉取新镜像）
- 升级状态表 `server_versions` 由 pixiu 主服务启动时统一初始化（依赖 `default.auto_migrate: true`，或由运维手工建表）；pixiu-upgrade 不再建表，只做更新与查询，请确保主服务至少成功启动过一次。
- pixiu-upgrade 启动时会校验 `server_versions` 必需列，缺失时直接报错退出，提示先启动主服务建表。

## 镜像构建与运行

- 镜像地址：`crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu-upgrade:latest`（dev，amd64）与 `...:v2.0.2-beta.1`（多架构版本）。
- GitHub Actions `build-upgrade-dev-image.yml` 在 master/开发分支推送且 upgrade 相关路径变化时，仅构建 linux/amd64 并推送 `latest`，用于日常开发快速验证。
- GitHub Actions `build-upgrade-image.yml` 在 master 分支推送且 `cmd/upgrade/**`、`pkg/upgrade/**`、`pkg/controller/upgrade/**`、`pkg/db/**`、`pkg/util/runtime/**`、`cmd/app/config/**`、`docker/Dockerfile-upgrade`、`go.mod`、`go.sum` 等路径变化时，构建 linux/amd64 与 linux/arm64 多架构镜像，仅推送版本 tag `v2.0.2-beta.1`（不再覆盖 `latest`）。
- 镜像内置 `upgrade` 二进制并默认读取 `/etc/pixiu/config.yaml`，可用 `-config` 覆盖；重建容器通过 docker/containerd 客户端完成，不依赖 nerdctl。

Docker 运行示例：

```bash
docker run -d --name pixiu-upgrade --restart=always --net host \
  -v /etc/pixiu:/etc/pixiu \
  -v /var/run/docker.sock:/var/run/docker.sock \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu-upgrade:latest
```

containerd 运行示例：

```bash
nerdctl run -d --name pixiu-upgrade --restart=always --net host --privileged=true \
  -v /etc/pixiu:/etc/pixiu \
  -v /run/containerd:/run/containerd \
  -v /var/lib/containerd:/var/lib/containerd \
  -v /var/lib/nerdctl:/var/lib/nerdctl \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu-upgrade:latest
```

覆盖配置路径时，在镜像名后追加 `-config /path/to/config.yaml` 即可。

## 升级窗口与失败处理

- 用户确认升级后，API 返回成功即表示状态已迁移到 1；真正的容器重建在 upgrade 进程的下一次轮询（默认 5 秒）开始。
- 升级窗口内 pixiu 服务会短暂不可用（容器重建），建议在业务低峰执行。
- 拉取镜像或重建容器失败时，状态置为 4 并记录 `error_message`，不会自动回滚；需人工处理后在页面重试（4 → 1）。
- 重建后会观察容器 30 秒：期间容器退出/崩溃即判定启动失败，状态置为 4。
- 成功重建并在 30 秒观察窗口内保持运行后，由 pixiu-upgrade 原子回写状态 3，并在同一事务内把 `is_current` 切到新版本记录（旧记录 `is_current` 置 0）。回写失败只记录日志，不影响已运行的新容器。

## 崩溃恢复

- upgrade 进程启动时执行 `Reset`：把自己上次遗留的「升级中」（状态 2）记录置为失败（状态 4），并写入 `error_message` 说明进程重启。这样记录不会永久卡在 2，用户可以重新发起升级。
- 由于 `upgrade_id` 参与条件更新，其他实例不会误清理不属于自己的「升级中」记录。

## 接口

- `GET /pixiu/upgrade/latest-version`：返回最新一条升级记录：`enabled`、`version_tag`、`is_current`、`commit_id`、`image`、`status`、`upgrading`（兼容字段，等价于 `status` 为 1 或 2）、`error_message`、`release_time`。`enabled` 表示升级功能是否开启；`release_time` 为该版本的发布时间（可选，无法解析时缺省）。
- `POST /pixiu/upgrade`：管理员触发升级（0/4 → 1）。无可用升级或目标镜像为空返回 409「当前已是最新版本」；状态 1/2 返回 409「升级已在进行中」；并发下状态被改变返回 409「升级状态已变化，请刷新后重试」。
- `GET /pixiu/version`：返回 `{"version": "..."}`，即当前 `is_current` 记录的 `version_tag`；无 `is_current` 记录时 `version` 为空字符串。所有登录用户可读。

`GET /latest-version` 与 `POST /pixiu/upgrade` 均仅管理员可访问；`upgrade.enabled=false` 时返回 enabled=false，触发升级返回 409。

## is_current 语义

`is_current` 标记当前已部署且容器稳定运行的版本：由 pixiu-upgrade 在容器稳定运行后回写升级完成时，于同一事务内切换（新记录置 1、其他记录置 0）；升级进行中或启动观察窗口内不会切换。
