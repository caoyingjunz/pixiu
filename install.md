# 前置准备
```bash
docker 已经安装
代码 https://github.com/caoyingjunz/pixiu
```
# 数据库
```bash
选择1：直接提供可用数据库

选择2：快速启动数据库
docker run -d --net host --restart=always --privileged=true --name mariadb -e MYSQL_ROOT_PASSWORD="Pixiu868686" harbor.cloud.pixiuio.com/pixiuio/mysql:5.7

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
docker run -d --net host --restart=always --privileged=true -v /etc/pixiu:/etc/pixiu -v /var/run/docker.sock:/var/run/docker.sock --name pixiu-aio harbor.cloud.pixiuio.com/pixiuio/pixiu-aio
登录效果
浏览器登陆: http://192.168.16.156
```
# 页面展示
首页
<img width="1647" alt="image" src="https://github.com/youdian-xiaoshuai/pixiu/assets/64686398/9fc5e005-95cd-49ee-a13c-13f22949fd74">

容器服务
![image](https://github.com/youdian-xiaoshuai/pixiu/assets/64686398/9e450085-2297-4453-80e3-40b0775796a8)

集群展示
![image](https://github.com/youdian-xiaoshuai/pixiu/assets/64686398/9d3fa88a-6da1-4d86-bfb2-0f9a50ab77d5)

命名空间
![image](https://github.com/youdian-xiaoshuai/pixiu/assets/64686398/2af3946b-4f66-4859-bee4-68589d889ef5)



