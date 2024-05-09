# 前置准备
```bash
docker 已经安装
代码 https://github.com/caoyingjunz/pixiu
```
# 数据库
```bash
选择1：直接提供可用数据库

选择2：快速启动数据库
docker run -d --net host --restart=always --privileged=true --name mariadb -e MYSQL_ROOT_PASSWORD="Pixiu868686" mysql:5.7

创建 pixiu 数据库
CREATE DATABASE pixiu;
```
# 启动 pixiu 服务端
```bash
创建配置文件夹
mkdir -p /etc/pixiu/

后端配置(host 根据实际情况调整)
vim /etc/pixiu/config.yaml 写入后端如下配置
default:
  # 运行模式，可选 debug 和 release
  mode: debug
  listen: 8090
  auto_migrate: true

数据库地址信息
mysql:
  host: pixiu
  user: root
  password: Pixiu868686
  port: 3306
  name: pixiu

前端配置(ip 根据实际情况调整)
vim /etc/pixiu/config.json
{
    "url": "http://192.168.16.156"
}
```
# 启动 pixiu
```bash
docker run -d --net host --restart=always --privileged=true -v /etc/pixiu/:/configs  --name pixiu-aio jacky06/pixiu-aio
登录效果
浏览器登陆: http://192.168.16.156
```
