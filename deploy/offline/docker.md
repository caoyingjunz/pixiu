# Docker 快速安装

### 获取安装包
- Docker 安装包 获取 [Docker 基础包](https://github.com/offline-hub/repo/releases/tag/download)

### 解压
```bash
tar xzvf docker-29.7.2.tgz
```

### 安装二进制文件
```bash
sudo cp docker/* /usr/bin/
```

### 启动Docker服务
```bash
vim /etc/systemd/system/docker.service

[Unit]
Description=Docker

[Service]
ExecStart=/usr/bin/dockerd
Restart=always

[Install]
WantedBy=multi-user.target

# 配置完加载，并设置开机启动
sudo systemctl daemon-reload
sudo systemctl enable docker
sudo systemctl start docker
```

