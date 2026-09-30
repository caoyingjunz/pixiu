# Pixiu Deploy Agent

边缘节点主动出站连接 Pixiu，拉取部署任务与计划数据，在本地渲染配置并通过安装驱动容器完成集群部署。

## 工作原理

```
┌──────────────┐  HTTPS (出站)  ┌──────────────┐
│  Pixiu 控制面  │ ◄──────────── │  Deploy Agent │
│              │  拉取任务/回传   │  （边缘节点）   │
└──────────────┘                └──────┬───────┘
                                       │ socket
                                       ▼
                               ┌──────────────┐
                               │ 部署容器运行时  │
                               └──────────────┘
```

- 有任务时拉取执行，完成后回传结果
- 适用于单向网络环境（Agent 可访问控制面，反之不可）

## 前置条件

- Pixiu 控制面已部署，Agent 可访问控制面地址
- 已在控制面创建 Agent 并获取 Token
- Agent 节点已安装并运行 containerd（默认）或 docker：`runtime.cri` **缺省为 containerd**，只有本机只有 docker 时才需在 `agent.yaml` 显式写 `cri: docker`
- Agent 以 root 运行：containerd 需通过 `/run/containerd/containerd.sock` 通信（默认 socket），非 root 需具备该 socket 读写权限

## 安装步骤
### 0. 容器运行时准备

- 用 containerd：参见 [deploy/containerd/README.md](../containerd/README.md)（安装 containerd 与 nerdctl、权限说明；Agent 只用到 containerd，不依赖 nerdctl）。
- 用 docker：[Docker极速安装](../offline/docker.md)。

### 1. 获取 Token

在 Pixiu 管理页面「代理管理」中新增一个 **部署代理**，复制生成的 Token。
![img.png](img.png)

### 2. 下载二进制文件

```bash
# 下载最新版本
curl -Lo /usr/local/bin/pixiu-deploy-agent \
  https://pixiu-1302939330.cos.ap-guangzhou.myqcloud.com/deploy-agent/pixiu-deploy-agent
chmod +x /usr/local/bin/pixiu-deploy-agent
```

### 3. 生成配置文件

```bash
mkdir -p /etc/pixiu

cat > /etc/pixiu/agent.yaml <<EOF
default:
  server: "https://pixiu.example.com"
  token: "<your-agent-token>"

  # 宿主容器运行时配置（Agent 在本机拉起部署 runner 容器）
  # 注意：这是宿主运行时，与「被部署集群的 CRI」不是一回事
  # 缺省 containerd，不配置即使用 containerd；仅当本机只有 docker 时才改为 cri: docker
  runtime:
    # 运行时类型：docker / containerd，默认 containerd
    cri: containerd
    #docker:                # 仅当 cri: docker 时生效
    #  # 留空沿用 docker 环境变量/默认 socket
    #  host: ""
    #containerd:            # cri: containerd 时的可选项，整段不写即用下列默认值
    #  # containerd gRPC socket 地址
    #  address: /run/containerd/containerd.sock
    #  # 命名空间，默认 default；禁止使用 k8s.io（kubelet 工作区，会被拒绝）
    #  namespace: default
EOF

# 运行时配置只取自本文件（或下面的 PIXIU_RUNTIME_CRI 环境变量），
# 由 Agent 自己决定用哪种宿主运行时，控制面不会下发 runtime 配置。
# 配置文件优先于环境变量，留空时回退到环境变量；
# 运行时类型也可用 PIXIU_RUNTIME_CRI 指定；docker.host、containerd.address/namespace
# 等子项仅走配置文件，环境变量不生效
```

> **存量节点升级注意**：历史 `agent.yaml` 若无 runtime 段，新版默认按 containerd 启动；docker 节点请先在 `agent.yaml` 写 `cri: docker`，否则 agent 会因连不上 containerd 启动失败。

### 4. 注册 systemd 服务

```bash
# After/Wants 按宿主实际使用的运行时选：默认 containerd 用 containerd.service；
# 若在 agent.yaml 里配了 cri: docker，则把下面两行的 containerd.service 换成 docker.service
cat > /etc/systemd/system/pixiu-deploy-agent.service <<EOF
[Unit]
Description=Pixiu Deploy Agent
After=network.target containerd.service
Wants=containerd.service

[Service]
Type=simple
ExecStart=/usr/local/bin/pixiu-deploy-agent -config=/etc/pixiu/agent.yaml
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable pixiu-deploy-agent
systemctl start pixiu-deploy-agent
```

### 5. 验证状态

```bash
systemctl status pixiu-deploy-agent
journalctl -u pixiu-deploy-agent -f
```

正常日志应显示 `pixiu-deploy-agent v0.1.0 starting, server=https://pixiu.example.com`，且心跳日志定期出现。
