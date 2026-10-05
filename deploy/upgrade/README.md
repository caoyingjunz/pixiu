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

## 基于 systemd unit（ctr）

宿主使用 containerd、pixiu 与 mariadb 由 systemd unit 托管（unit 以 `ctr` 前台运行容器）时，升级 = 拉取新版本镜像 → 更新 unit 文件 → 重载并重启。

> **迁移提醒（旧手工命令式部署首次切到 unit 必读）**
>
> - **数据库数据**：旧 mariadb 容器未挂载数据卷（数据在容器内），unit 使用宿主目录 `/var/lib/pixiu-mariadb`（新目录），直接切换将得到**空库**。旧库有业务数据时，请在执行下方升级步骤**前**完成本节末尾「迁移：旧库数据导出与导入」中的**导出**，升级后再**导入**。
> - **旧 pixiu 容器**：无需手工处理，unit 的 `ExecStartPre` 会在启动时自动清理同名旧容器/任务并重建（如确需手工删除：`sudo ctr -n default task delete -f pixiu && sudo ctr -n default containers rm pixiu`；若原部署工具仍在，用其等效的 `rm -f` 亦可）。

### 1）拉取新版本镜像

```bash
# tag 换成目标版本
sudo ctr -n default images pull crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1
```

### 2）更新 unit 文件

```bash
# pixiu
sudo tee /etc/systemd/system/pixiu.service > /dev/null <<'EOF'
[Unit]
Description=pixiu server (containerd runtime)
After=containerd.service network-online.target
Requires=containerd.service
Wants=network-online.target

[Service]
Type=simple
ExecStartPre=-/usr/bin/ctr -n default task kill -s SIGKILL pixiu
ExecStartPre=-/usr/bin/ctr -n default task delete -f pixiu
ExecStartPre=-/usr/bin/ctr -n default containers rm pixiu
ExecStart=/usr/bin/ctr -n default run \
  --privileged \
  --net-host \
  --mount type=bind,src=/etc/pixiu,dst=/etc/pixiu,options=rbind:rw \
  --mount type=bind,src=/run/containerd,dst=/run/containerd,options=rbind:rw \
  --mount type=bind,src=/var/lib/containerd,dst=/var/lib/containerd,options=rbind:rw \
  crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.1 \
  pixiu
ExecStop=-/usr/bin/ctr -n default task kill -s SIGTERM pixiu
Restart=always
RestartSec=5
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
EOF

# 数据库
sudo mkdir -p /var/lib/pixiu-mariadb
sudo tee /etc/systemd/system/pixiu-mariadb.service > /dev/null <<'EOF'
[Unit]
Description=pixiu mariadb (containerd runtime)
After=containerd.service network-online.target
Requires=containerd.service
Wants=network-online.target

[Service]
Type=simple
ExecStartPre=-/usr/bin/ctr -n default task kill -s SIGKILL mariadb
ExecStartPre=-/usr/bin/ctr -n default task delete -f mariadb
ExecStartPre=-/usr/bin/ctr -n default containers rm mariadb
ExecStart=/usr/bin/ctr -n default run \
  --privileged \
  --net-host \
  --mount type=bind,src=/var/lib/pixiu-mariadb,dst=/var/lib/mysql,options=rbind:rw \
  --env MYSQL_ROOT_PASSWORD=Pixiu868686 \
  --env MYSQL_DATABASE=pixiu \
  ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7 \
  mariadb
ExecStop=-/usr/bin/ctr -n default task kill -s SIGTERM mariadb
Restart=always
RestartSec=5
TimeoutStopSec=60

[Install]
WantedBy=multi-user.target
EOF
```

### 3）重载并重启

```bash
sudo systemctl daemon-reload
sudo systemctl enable pixiu pixiu-mariadb
sudo systemctl restart pixiu pixiu-mariadb
```

### 迁移：旧库数据导出与导入

```bash
# 切换前：旧 mariadb 容器仍在运行时导出
sudo ctr -n default task exec --exec-id dump mariadb mysqldump -uroot -pPixiu868686 pixiu > /root/pixiu-db.sql
```

```bash
# 切换后：unit 拉起的 mariadb 就绪后导入
sudo ctr -n default task exec --exec-id imp mariadb mysql -uroot -pPixiu868686 pixiu < /root/pixiu-db.sql
```

说明：`/etc/pixiu` 配置目录不受影响，沿用旧配置即可（unit 到 unit 的升级，数据库数据也不受影响——数据在宿主目录 `/var/lib/pixiu-mariadb`；从旧手工部署迁移则按上方迁移提醒导出/导入）；runner 日志新位置为 `/etc/pixiu/runner-logs`；历史日志如需保留，可一次性拷贝 `cp -a /var/lib/pixiu/runner-logs /etc/pixiu/runner-logs`（不拷贝则旧日志不再被读取）。
