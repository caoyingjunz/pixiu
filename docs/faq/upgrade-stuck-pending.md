# FAQ-009 点击升级后一直显示「升级中」（未部署 pixiu-upgrade）

- **分类**：自动升级
- **涉及组件**：系统升级（pixiu 主服务 + pixiu-upgrade 升级进程）

## 现象

在页面点击「重启更新」后，弹窗/侧边栏按钮一直显示「升级中…」，pixiu 容器没有重建，版本不变。

接口 `GET /pixiu/upgrade/latest-version` 返回 `status` 为 1（待升级）或 2（升级中），且 `upgrading=true`。

## 根因

- 点击升级只是把记录从 0/4 改为 1（`POST /pixiu/upgrade` → `MarkPending`），**真正执行升级的是独立的 pixiu-upgrade 进程**：它轮询认领记录（1→2，`Claim`），然后拉取镜像、重建 pixiu 容器。
- **自动升级默认关闭**：配置未显式 `upgrade.enabled: true` 时，pixiu-upgrade 进程启动即退出，不会认领升级记录。部署前请确认该开关已打开。
- 没部署/没启动 pixiu-upgrade，或配置 `upgrade.enabled: false`（含未配置时的默认关闭），或连不上 MySQL / 容器运行时，记录就会停在 1，永远不推进。页面把 status 1 和 2 都展示为「升级中」。
- 若进程认领后（已到 2）崩溃/退出，记录会停在 2；只在该进程下次启动时由 `Reset` 回写为 4（失败）。

## 验证方式

```bash
# 1. 确认升级进程是否存在
nerdctl ps -a | grep pixiu-upgrade      # containerd
docker ps | grep pixiu-upgrade          # docker

# 2. 查看进程日志：是否 enabled=false 退出，或连库/运行时失败
nerdctl logs pixiu-upgrade

# 3. 查看数据库中的升级记录
SELECT id, version_tag, status, upgrade_id FROM server_versions ORDER BY id DESC LIMIT 5;
```

## 解决方案

1. 按 [自动升级部署说明](../upgrade/auto-upgrade.md) 部署 pixiu-upgrade（挂载 `/etc/pixiu`、容器运行时 socket，能访问 MySQL 与镜像仓库）。启动后会自动认领 status=1 的记录继续升级；若记录停在 2，进程启动时会先 `Reset` 回 4，再重新点升级即可。
2. 暂不升级时，手工复位最新一条记录：

   ```sql
   UPDATE server_versions SET status = 0, upgrade_id = '' WHERE id = <记录 id>;
   ```

   只按最新 `id` 处理，不要批量更新全部历史记录。

## 备注 / 风险

- 只启动 pixiu-upgrade 容器但配置 `upgrade.enabled: false`，等同于未部署（进程会直接退出）。
- 升级期间请保持该进程运行，否则记录可能停在待升级/升级中。
- `server_versions` 表由 pixiu 主服务启动时创建，主服务至少需成功启动过一次。
