# Pixiu Deploy Agent（单向装集群 HTTPS 作业通道）

边缘节点主动出站连接 Pixiu，拉取部署任务与计划数据，在本地渲染配置并执行 `kubez-ansible` 容器。

## 控制面

1. 创建 Agent（保存返回的 `token`）：

```bash
POST /pixiu/agents
{"name":"edge-1","description":"单向部署节点"}
```

2. 创建 Plan 时指定：

```json
{
  "exec_mode": "agent",
  "deploy_agent_id": 1,
  ...
}
```

3. 启动 Plan：`POST /pixiu/plans/:id/start`

控制面 **不渲染** 部署配置，只下发 Job 并等待 Agent 回报结果。

## Agent 节点

```bash
export PIXIU_SERVER=https://pixiu.example.com
export PIXIU_DEPLOY_TOKEN=<token>
deploy-agent
```

或使用 `deploy/agent/pixiu-deploy-agent.yaml`（需挂载 docker.sock）。

## Agent 作业 API（Token: X-Pixiu-Deploy-Token）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/pixiu/agents/heartbeat` | 心跳 |
| GET | `/pixiu/agents/claim` | 领取任务 |
| GET | `/pixiu/agents/jobs/:id/material` | 拉取计划数据（Agent 本地渲染） |
| POST | `/pixiu/agents/jobs/:id/logs` | 追加日志 |
| POST | `/pixiu/agents/jobs/:id/result` | 回传结果 |

## 作业类型

| kind | 说明 |
|------|------|
| `pull_image` | 拉取 runner 镜像 |
| `render_config` | 拉取 material 并在 Agent 本地渲染 hosts/multinode/globals |
| `run_container` | 挂载渲染目录执行 kubez-ansible |
| `fetch_kubeconfig` | SSH 拉取 admin.conf 并回传 |
