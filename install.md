# 前置准备
```bash
确保 docker 已经安装
```
# 数据库
```bash
# 选择1：直接提供可用数据库

# 选择2：快速启动数据库
docker run -d --net host --restart=always --privileged=true --name mariadb -e MYSQL_ROOT_PASSWORD="Pixiu868686" harbor.cloud.pixiuio.com/pixiuio/mysql:5.7

# 创建 pixiu 数据库
CREATE DATABASE pixiu;
```

# 获取部署驱动镜像
```shell
docker pull harbor.cloud.pixiuio.com/pixiuio/kubez-ansible:v2.0.1
docker pull harbor.cloud.pixiuio.com/pixiuio/kubez-ansible:v3.0.1
```

# 启动 pixiu 服务端
```bash
# 创建配置文件夹
mkdir -p /etc/pixiu/

# 后端配置(host 根据实际情况调整)
vim /etc/pixiu/config.yaml 写入后端如下配置
default:
  # 运行模式，可选 debug 和 release
  mode: debug
  # 服务监听端口
  listen: 8090
  # 自动创建指定模型的数据库表结构，不会更新已存在的数据库表
  auto_migrate: true

# 数据库地址信息, 根据实际情况配置
mysql:
  host: pixiu # 数据库的ip
  user: root
  password: Pixiu868686
  port: 3306
  name: pixiu

worker:
  engines:
    - image: harbor.cloud.pixiuio.com/pixiuio/kubez-ansible:v2.0.1
      os_supported:
        - centos7
        - debian10
        - ubuntu18.04
    - image: harbor.cloud.pixiuio.com/pixiuio/kubez-ansible:v3.0.1
      os_supported:
        - debian11
        - ubuntu20.04
        - ubuntu22.04
        - rocky8.5
        - rocky9.2
        - rocky9.3
        - openEuler22.03

# 前端配置(ip 根据实际情况调整，如果是虚拟机，则配置成虚拟机的公网IP，安全组放通80和8090端口)
vim /etc/pixiu/config.json
{
    "url": "http://192.168.16.156",
    "watchUrl": "http://192.168.16.156:8090"
}
```
# 启动 pixiu
```bash
docker run -d --net host --restart=always --privileged=true -v /etc/pixiu:/etc/pixiu -v /var/run/docker.sock:/var/run/docker.sock --name pixiu-aio harbor.cloud.pixiuio.com/pixiuio/pixiu-aio
登录效果
浏览器登陆: http://192.168.16.156
```



