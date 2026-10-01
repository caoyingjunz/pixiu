# Containerd 在线安装

> containerd 复用 Docker 官方 apt 源安装（官方维护的 containerd.io 包），nerdctl 用官方发布包安装；容器操作需 root。

```bash
# 1) 添加 Docker 官方 apt 源（containerd.io 由 Docker 官方维护发布）
sudo apt-get update
sudo apt-get install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update

# 2) 安装 containerd 并设置开机自启（国内网络可把两处 download.docker.com 都换成 mirrors.aliyun.com/docker-ce）
sudo apt-get install -y containerd.io
sudo systemctl enable --now containerd

# 3) 安装 nerdctl v2.4.0（官方发布包；arm64 节点把文件名中的 amd64 换成 arm64）
curl -fsSL -o /tmp/nerdctl.tar.gz https://github.com/containerd/nerdctl/releases/download/v2.4.0/nerdctl-2.4.0-linux-amd64.tar.gz
sudo tar -xzf /tmp/nerdctl.tar.gz -C /usr/local/bin nerdctl
rm -f /tmp/nerdctl.tar.gz
```

> 如需 nerdctl 桥接网络（非 host），还需将 CNI 插件安装到 `/opt/cni/bin`。
