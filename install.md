# 前置准备
```bash
确保 docker 已经安装
```
# 数据库
```bash
# 选择1：直接提供可用数据库

# 选择2：快速启动数据库（生产环境自行部署或者使用高可用数据库）
docker run -d --net host --restart=always --privileged=true --name mariadb -e MYSQL_ROOT_PASSWORD="Pixiu868686" -e MYSQL_DATABASE="pixiu" ccr.ccs.tencentyun.com/pixiucloud/mysql:5.7
```

# 获取部署驱动镜像（可选，如果没有部署k8s需求可跳过）
```shell
docker pull ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v2.0.2
docker pull ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v3.0.2
```

# 启动 pixiu 服务端
## 配置 pixiu
```bash
# 创建配置文件夹
mkdir -p /etc/pixiu/

# 后端配置(host 根据实际情况调整)
vim /etc/pixiu/config.yaml 写入后端如下配置
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

worker:
  engines:
    - image: ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v2.0.2
      os_supported:
        - centos7
        - debian10
        - ubuntu18.04
    - image: ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v3.0.2
      os_supported:
        - debian11
        - ubuntu20.04
        - ubuntu22.04
        - rocky8.5
        - rocky9.2
        - rocky9.3
        - openEuler22.03
```

## 启动 pixiu
```bash
# 根据实际需要修改宿主机端口，默认使用宿主机端口，可替换 --net host 为期望端口映射 -p <hostPort>:80
docker run -d --net host --restart=always --privileged=true -v /etc/pixiu:/etc/pixiu -v /var/run/docker.sock:/var/run/docker.sock --name pixiu ccr.ccs.tencentyun.com/pixiucloud/pixiu:v2.0.1-beta.2
```

## 登陆 pixiu
```
# 根据配置文件中指定的账密输入；如果未指定默认用户名密码是 admin/Pixiu123456!
浏览器登陆: http://<ip>:80
```
