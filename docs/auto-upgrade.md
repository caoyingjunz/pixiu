# pixiu 自动升级（本地部署模式）

自动升级让 pixiu 在本地部署（容器方式运行在本机 docker/containerd 上）的场景下自主完成版本升级：
守护任务定时检查外部版本号，发现新版本后拉取镜像并重建 pixiu 服务容器。

> 仅支持本地部署模式。k8s/helm 等部署形态下守护任务发现不了本地 pixiu 容器，会自动跳过。

## 工作原理

```text
┌────────────────────────────── pixiu 容器（旧版本） ──────────────────────────────┐
│  pixiu-upgrader job（cron 周期）                                                │
│    1. GET upgrade.version_url 获取最新版本号                                    │
│    2. 与当前版本比较（状态文件优先，其次解析当前镜像 tag）                          │
│    3. 有新版本：拉取 <image>:<新版本>                                             │
│    4. 把重建规格写入 /etc/pixiu/upgrade/state.json（挂载卷内，升级不丢失）          │
│    5. 以新镜像拉起一次性「升级执行器」容器                                         │
└────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌────────────── 升级执行器容器（新镜像，独立任务） ──────────────────────────────────┐
│  pixiu-server upgrade-execute                                                   │
│    1. 等待 5s（让调用方收尾）                                                     │
│    2. 删除旧 pixiu 容器                                                          │
│    3. 按原容器规格（env/挂载/网络/特权）+ 新镜像重建服务容器                          │
│    4. 失败时用旧镜像回滚；成功后更新状态文件并退出                                   │
└─────────────────────────────────────────────────────────────────────────────────┘
```

为什么需要独立的升级执行器：pixiu 自身容器被删除时，容器内的进程（包括发起重建的守护任务）
也会随之终止，「先停自己、谁来起新」是自我重建的死结；执行器是宿主 CRI 上的独立容器任务，
不随旧容器生命周期结束。

## 配置

在 `/etc/pixiu/config.yaml` 中追加：

```yaml
upgrade:
  enabled: true
  # 检查周期（cron 表达式），默认每 30 分钟
  schedule: "*/30 * * * *"
  # 外部版本号地址：HTTP(S) GET 返回最新版本号，支持纯文本或 JSON
  # 纯文本示例：v2.0.2-beta.1
  # JSON 示例：{"version": "v2.0.2-beta.1"}
  version_url: "https://your-domain/pixiu/latest-version"
  # 镜像仓库地址（不能带 tag）：升级时以版本号作为 tag 拉取与重建
  image: "crpi-xxx.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu"
  # pixiu 服务容器名，默认 pixiu
  container_name: "pixiu"
  # pixiu 二进制在镜像内的路径，默认 /app（用于拉起升级执行器）
  # executable: "/app"
```

### 外部版本号地址

一个返回最新版本号的 HTTP(S) GET 接口即可，例如由 CI 在镜像发布后更新的静态文件：

```bash
# 镜像发布流水线中追加
echo "v2.0.3" > /var/www/html/pixiu/latest-version
```

## 使用前提

- pixiu 以容器方式运行在本机 docker 或 containerd 上（`runtime.cri` 配置正确，socket 已挂载）
- `/etc/pixiu` 以宿主目录挂载进容器（升级状态文件存放在 `/etc/pixiu/upgrade/state.json`）
- 当前容器规格中的挂载（含 CRI socket）会被检出并原样还原到新容器

## 版本比较规则

- 支持 `v` 前缀可选的多段数字版本号（如 `v1.2.3`、`1.2`）与预发布后缀（如 `-beta.1`，低于同版本正式版）
- 外部版本**不高于**当前版本时跳过升级
- 首次运行且无法从镜像 tag 解析版本（如 `latest`）时，采纳外部版本号为基线并记录，
  从下一轮检查开始生效，避免对未知现状盲目重建

## 状态文件

`/etc/pixiu/upgrade/state.json`：

```json
{
  "version": "v2.0.3",
  "image": "crpi-xxx/pixiu-public/pixiu:v2.0.3",
  "container_name": "pixiu",
  "updated_at": "2026-11-06T18:00:00+08:00",
  "pending": {
    "target_version": "v2.0.3",
    "target_image": "crpi-xxx/pixiu-public/pixiu:v2.0.3",
    "old_image": "crpi-xxx/pixiu-public/pixiu:v2.0.2-beta.1",
    "spec": {},
    "runtime_opts": {},
    "error": ""
  }
}
```

- `pending` 非空表示一次升级进行中；成功重建后被清空，`version`/`image` 更新为新版本
- `pending.error` 记录执行器侧最近一次失败原因
- 执行器重建失败时会先尝试用旧镜像回滚；若回滚也失败，需要人工介入

## 升级窗口与影响

- 重建即中断：从旧容器被删除到新容器就绪之间有短暂不可用（通常数秒），请选择合适窗口
- 升级期间宿主上的其他容器（如数据库）不受影响
- 升级不会修改 `/etc/pixiu/config.yaml`；跨版本配置兼容性需自行确认
