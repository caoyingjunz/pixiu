# 在线安装 docker（以 Ubuntu 为例）

> `docker-ce` 不在 Ubuntu 默认源里，需先添加 Docker 官方 apt 源；离线环境见 [离线安装](offline-docker.md)。

```bash
# 1) 卸载可能冲突的旧包（新机器可跳过）
sudo apt-get remove -y docker.io docker-doc docker-compose docker-compose-v2 podman-docker containerd runc

# 2) 安装依赖并添加 Docker 官方 GPG key
#    默认使用阿里云镜像源；海外网络可改为官方源 https://download.docker.com
#    备选镜像：https://mirrors.cloud.tencent.com/docker-ce 、https://mirrors.ustc.edu.cn/docker-ce
#    （变量与后续命令需在同一终端会话内执行）
export DOCKER_REPO=https://mirrors.aliyun.com/docker-ce
sudo apt-get update
sudo apt-get install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL "$DOCKER_REPO/linux/ubuntu/gpg" -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# 3) 添加 Docker 软件源
#    （此前按旧版文档执行失败过的机器：重跑第 2、3 步整段即可修复，会重下 key 并覆写 docker.list）
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] $DOCKER_REPO/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update

# 4) 安装 docker 并设置开机自启（apt 安装通常已自动启动，可核查）
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl enable --now docker
```
