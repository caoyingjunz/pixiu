# 安装手册

### 启动 pixiu 服务端
```bash
# 创建配置文件夹
mkdir -p /etc/pixiu/

# 后端配置
vim /etc/pixiu/config.yaml
default:
  # 运行模式，可选 debug 和 release
  mode: debug
  listen: 8090
  auto_migrate: true

# 前端配置(ip 根据实际情况调整)
vim /etc/pixiu/config.json
{
    "url": "http://192.168.16.156"
}
```

### 启动 pixiu
```bash
docker run -d --net host --restart=always --privileged=true -v /etc/pixiu/:/configs  --name pixiu-aio jacky06/pixiu-aio

# 登录效果
浏览器登陆: http://192.168.16.156
```
