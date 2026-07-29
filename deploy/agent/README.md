# Pixiu Deploy Agent（单向装集群 HTTPS 作业通道）

边缘节点主动出站连接 Pixiu，拉取部署任务并在本地执行 `kubez-ansible` 容器。

## 控制面

1. 创建 Agent（保存返回的 `token`）：

```bash
POST /pixiu/deploy-agents
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
| POST | `/pixiu/deploy-agents/heartbeat` | 心跳 |
| GET | `/pixiu/deploy-agents/tasks/claim` | 领取任务 |
| GET | `/pixiu/deploy-agents/tasks/:id/bundle` | 下载配置包 |
| POST | `/pixiu/deploy-agents/tasks/:id/logs` | 追加日志 |
| POST | `/pixiu/deploy-agents/tasks/:id/result` | 回传结果 |
