# Containerd 在线安装

> containerd 复用 Docker 官方 apt 源安装（官方维护的 containerd.io 包，国内默认走镜像源），nerdctl 用官方发布包安装；容器操作需 root。

```bash
# 1) 添加 Docker apt 源（containerd.io 由 Docker 官方维护发布）
#    默认使用阿里云镜像源；海外网络可改为官方源 https://download.docker.com
#    备选镜像：https://mirrors.cloud.tencent.com/docker-ce 、https://mirrors.ustc.edu.cn/docker-ce
#    （变量与后续命令需在同一终端会话内执行）
export DOCKER_REPO=https://mirrors.aliyun.com/docker-ce
sudo apt-get update
sudo apt-get install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL "$DOCKER_REPO/linux/ubuntu/gpg" -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] $DOCKER_REPO/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update

# 2) 安装 containerd 并设置开机自启
#    （此前按旧版文档执行失败过的机器：重跑第 1 步整段即可修复，会重下 key 并覆写 docker.list）
sudo apt-get install -y containerd.io
sudo systemctl enable --now containerd
sudo systemctl start containerd

# 3) 安装 nerdctl v2.4.0（官方发布包；arm64 节点把文件名中的 amd64 换成 arm64）
curl -fsSL -o /tmp/nerdctl.tar.gz https://github.com/containerd/nerdctl/releases/download/v2.4.0/nerdctl-2.4.0-linux-amd64.tar.gz
sudo tar -xzf /tmp/nerdctl.tar.gz -C /usr/local/bin nerdctl
rm -f /tmp/nerdctl.tar.gz
```

> 如需 nerdctl 桥接网络（非 host），还需将 CNI 插件安装到 `/opt/cni/bin`。
