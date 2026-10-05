# Containerd 在线安装

### 安装 Containerd
```bash
# 1) 添加 Docker apt 源（containerd.io 由 Docker 官方维护发布）
export DOCKER_REPO=https://mirrors.aliyun.com/docker-ce
sudo apt-get update
sudo apt-get install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL "$DOCKER_REPO/linux/ubuntu/gpg" -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] $DOCKER_REPO/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 2) 安装
sudo apt-get update
sudo apt-get install -y containerd.io
sudo systemctl enable --now containerd
sudo systemctl start containerd
```

### 安装 nerdctl
```bash
curl -fsSL -o /tmp/nerdctl.tar.gz https://github.com/containerd/nerdctl/releases/download/v2.4.0/nerdctl-2.4.0-linux-amd64.tar.gz
sudo tar -xzf /tmp/nerdctl.tar.gz -C /usr/local/bin nerdctl
rm -f /tmp/nerdctl.tar.gz
```
