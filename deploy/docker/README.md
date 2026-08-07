# 前置准备
```bash
确保 docker 已经安装
```



# 获取部署驱动镜像（可选）

如果没有部署k8s需求，或者可联网可跳过，pixiu 部署时会自行拉取镜像

```shell
docker pull ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v2.0.2
docker pull ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v3.0.3

```



# 启动 pixiu

将`docker-compose.yaml`和`config.yaml` 放到 `/etc/pixiu`目录下

```bash
docker-compose up -d
```



## 登陆 pixiu
```
# 根据配置文件中指定的账密输入；如果未指定默认用户名密码是 admin/Pixiu123456!
浏览器登陆: http://<ip>:8080
```

