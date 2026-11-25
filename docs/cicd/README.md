# CI/CD 流水线（控制面 MVP）

基于 `docs/cicd` 设计方案的 P1 实现：平台侧控制面 + 前端，执行引擎为集群内的 Tekton，构建使用 Buildah，产物推送到外部镜像仓库。本阶段覆盖「手动源码构建闭环」，不含 Webhook / 定时触发 / 部署联动（P2+）。

## 启用前置条件

1. 目标集群已安装 Tekton Pipelines（`tekton.dev/v1` 的 `pipelineruns` / `taskruns` CRD）。
2. 集群可构建镜像：构建 Pod 能 `buildah bud/push`（P0 已验证 Buildah rootless 可用；否则按设计降级 BuildKit rootless）。
3. 离线环境已将 Tekton 控制器、git-clone、buildah 镜像按 digest 随包分发。

## 配置

`config.yaml`：

```yaml
cicd:
  enabled: true                 # 总开关，false 时 worker 不启动、菜单隐藏
  workers: 2
  poll_interval: 3s
  log_retention_days: 14
  max_running_per_pipeline: 1
  max_pending_runs: 100
  log_dir: /tmp/pixiu/cicd-logs
  build:
    tool: buildah
    isolation: chroot
    storage_driver: vfs
    insecure_registry: false
```

前端 feature flag：`src/constants/feature-flags.ts` 的 `CICD_UI_VISIBLE`，关闭时隐藏一级菜单（与后端开关配合做灰度/回滚）。

## 使用流程

1. 在「CI/CD → 流水线」创建流水线：选目标集群与命名空间，填 Git 地址/ref、Dockerfile、上下文、构建参数、镜像仓库与 tag。
2. 凭据按 K8s Secret 准备，打 label `pixiu.io/cicd-credential=git|registry`，控制在命名空间内；平台 DB 不落明文、接口只返回元数据。
3. 点击「触发」创建运行记录；worker 每 `poll_interval` 领取 pending，原子 claim 后创建 Tekton PipelineRun（内联 `pipelineSpec`：clone → buildah → push，输出 `IMAGE_DIGEST` result）。
4. 运行详情页点「日志」以 SSE（`fetch` + `ReadableStream`，携带 Authorization）实时查看；运行结束回写状态、digest、耗时，可取消 / 重跑。

## API

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/pixiu/cicd/clusters/:clusterId/engine-status` | Tekton 安装状态 / 版本 |
| GET/POST | `/pixiu/cicd/pipelines` | 流水线列表 / 创建 |
| GET/PUT/DELETE | `/pixiu/cicd/pipelines/:id` | 详情 / 更新 / 删除 |
| POST | `/pixiu/cicd/pipelines/:id/runs` | 手动触发 |
| GET | `/pixiu/cicd/runs` | 运行列表 |
| GET | `/pixiu/cicd/runs/:id` | 运行详情 |
| GET | `/pixiu/cicd/runs/:id/logs` | SSE 日志 |
| POST | `/pixiu/cicd/runs/:id/cancel` | 取消 |
| POST | `/pixiu/cicd/runs/:id/retry` | 重跑 |
| GET/POST/DELETE | `/pixiu/cicd/clusters/:c/namespaces/:n/credentials[...]` | Secret 元数据管理 |

所有路由经 `apiregistry` 注册，`METHOD:path` 自动持久化为按钮权限；菜单码 `cicd` / `cicd.pipelines` 在 `pkg/rbac/menu/catalog.go` 与前端 `src/constants/menus.ts` 同步。

## 状态机与对账

`pending → running → succeeded / failed / timeout / cancelled`。以集群 PipelineRun 为事实源，平台表只做索引与审计：

- worker claim 前二次校验单流水线并发；确定性命名 `px-{pipelineId}-{runId}` 支持「创建成功但回写失败」对账。
- 启动时回查 running 记录，按集群真实状态收口；超时的僵尸记录置 failed。
- PipelineRun 设置 `ttlSecondsAfterFinished`，CR 自动回收，平台 DB 保留历史。

## 回滚

`cicd.enabled=false` + `CICD_UI_VISIBLE=false`：菜单隐藏、worker 不启动、接口不可用。两表为纯增量，历史 run 只读，无需回滚 DDL；整个 `pkg/controller/cicd` / `api/server/router/cicd` 模块可整体移除。
