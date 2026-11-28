# 前置准备
```bash
确保 docker 和 docker-compose 已经安装
```

# 获取部署驱动镜像（可选）

如果没有部署k8s需求，或者可联网可跳过，pixiu 部署时会自行拉取镜像

```shell
docker pull ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v2.0.2
docker pull ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v3.0.4
```

# 启动 pixiu

将`docker-compose.yaml`和`config.yaml` 放到 `/etc/pixiu`目录下

```bash
docker-compose up -d
```

验证
```bash
# docker-compose ps
NAME                IMAGE                                                   COMMAND                  SERVICE             CREATED             STATUS              PORTS
mysql               ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7             "docker-entrypoint.s…"   mysql               17 minutes ago      Up 17 minutes       3306/tcp, 33060/tcp
pixiu               crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com/pixiu-public/pixiu:v2.0.2-beta.3   "/docker-entrypoint.…"   pixiu               17 minutes ago      Up 17 minutes       0.0.0.0:8080->80/tcp, :::8080->80/tcp
```

## 登陆 pixiu
```
# 根据配置文件中指定的账密输入；如果未指定默认用户名密码是 admin/Pixiu123456!
浏览器登陆: http://<ip>:8080
```

## 开启自动升级

`docker-compose.yaml` 中的 `pixiu-upgrade` 服务负责执行升级，默认会随 `docker compose up -d` 启动。开启前确认：

1. `config.yaml` 位于宿主机 `/etc/pixiu/config.yaml`，且 pixiu 与 pixiu-upgrade 都挂载 `/etc/pixiu:/etc/pixiu`（仓库自带 compose 已配置）。配置目录不要改成其他路径——升级器按内置默认参数重建 pixiu 容器，固定使用 `-v /etc/pixiu:/etc/pixiu`。
2. `config.yaml` 中设置 `runtime.cri: docker` 和 `upgrade.enabled: true`。
3. 修改配置后执行 `docker compose down && docker compose up -d` 重建，使两个容器挂载同一份配置。

升级后 pixiu 容器由 docker 按 `--net host --restart=always --privileged=true` 重建；若原部署依赖自定义端口映射或环境变量，请在 `config.yaml` 的 `upgrade.run_args_docker` 中写全参数。详见 [自动升级文档](../../docs/upgrade/auto-upgrade.md)。
