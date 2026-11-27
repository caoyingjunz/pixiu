# pixiu CI/CD 控制面设计方案

| 项 | 内容 |
|---|---|
| 状态 | 方案评审中（未开发） |
| 日期 | 2026-11-25 |
| 关联 | GitHub issue #384「新增 CICD 功能的支持」（原题「支持 CICD（tekton）」） |
| 引擎 | Tekton Pipelines（开源，仅作为执行引擎） |
| 构建 | Buildah（默认），外部镜像仓库 |
| 范围 | 仅平台侧控制面 + UI，不自研调度/执行器 |

---

## 1. 范围与关键假设

**做**：平台侧控制面 + UI；执行全部交给集群内的开源引擎。

**不做**：
- 自研流水线调度器 / 构建执行器
- 内置镜像仓库（独立议题，当前前端 `image-repo` 仅为占位）
- Git 代码托管
- Jenkinsfile 解析（外部 Jenkins 仅做连接器，见 P3）
- Tekton Dashboard / Triggers / Hub（不安装，触发统一由 pixiu 接收）

已锁定的默认口径（如不同意需在开发前调整）：

| 项 | 默认口径 |
|---|---|
| 流水线归属 | 租户 + 目标集群 + 命名空间 |
| 执行对象 | 直接创建 PipelineRun（内联 pipelineSpec），不在集群预装 Task/Pipeline |
| 触发入口 | 手动 / 定时 / Webhook 统一由 pixiu 接收，再调集群 API 创建 Run |
| 凭据 | 只存 K8s Secret（按 label 发现），平台 DB 不落明文、不建凭据表 |
| 菜单 | 新增一级菜单「CI/CD」（流水线 / 运行记录） |
| 版本策略 | Tekton 锁定单一稳定版本，随离线包分发 |

---

## 2. 总体架构

```
┌────────────── pixiu 控制面（Go / gin） ──────────────┐
│  API /pixiu/cicd/*   Webhook /pixiu/hooks/cicd/*     │
│  pipeline CRUD │ run 编排 │ 引擎适配 │ 日志聚合       │
│  DB队列(claim) │ 状态同步 │ 凭据(Secret引用) │ 审计   │
└───────────────┬──────────────────────────────────────┘
                │ 现有 cluster client（含隧道/agent 集群）
                ▼
┌──────────── 目标 K8s 集群（租户命名空间） ────────────┐
│  Tekton Pipelines Controller                          │
│  PipelineRun（内联 pipelineSpec：clone → buildah → push）│
│  Pod 日志 ← 控制面采集                                 │
└───────────────────────────────────────────────────────┘
                │ 推送
                ▼
        外部镜像仓库（Harbor / ACR / SWR …）
```

要点：
- 不引入新服务，控制面逻辑跑在 pixiu 进程内（API + 后台 worker）。
- 复用现有 `pkg/client`（多集群）、`pkg/db`（GORM）、SSE、审计、菜单 RBAC。
- 目标集群需可经由 kubeconfig 或隧道访问（边缘集群走 `pkg/tunnel`）。

---

## 3. 领域模型

沿用 `pixiu.Model`（`id` / `resource_version` / `gmt_create` / `gmt_modified`），model 通过 `init(){ register(...) }` 自注册，AutoMigrate 自动建表。大文本字段用 `longtext`，保证 MySQL / PostgreSQL 兼容。

### 3.1 `cicd_pipelines`

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint PK | |
| resource_version | bigint | 乐观锁 |
| tenant_id | bigint, idx | 租户 |
| name | varchar(128) | 与 tenant 联合唯一 |
| description | text | |
| cluster_id | bigint, idx | 目标集群 |
| namespace | varchar(128) | 目标命名空间 |
| engine | varchar(16) | `tekton` |
| spec_mode | varchar(16) | `template`（默认）/ `raw`（高级 YAML） |
| template | varchar(64) | `source-build` |
| config | longtext | JSON：repo / build / image / advanced |
| pipeline_yaml | longtext | raw 模式使用 |
| timeout_seconds | int | 默认 1200 |
| enabled | bool | 默认 true |
| created_by | bigint, idx | |
| created_by_name | varchar(64) | |

索引：唯一 `(tenant_id, name)`；普通 `(cluster_id)`。

### 3.2 `cicd_runs`

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint PK | |
| pipeline_id | bigint, idx | |
| tenant_id | bigint, idx | 冗余，便于过滤 |
| cluster_id | bigint | 冗余 |
| namespace | varchar(128) | 冗余 |
| trigger_type | varchar(16) | `manual` / `webhook` / `schedule` / `api` |
| status | varchar(16), idx | `pending` / `running` / `succeeded` / `failed` / `cancelled` / `timeout` |
| engine | varchar(16) | `tekton` |
| engine_name | varchar(191) | PipelineRun 名称，`px-{pipelineId}-{runId}` |
| git_ref | varchar(191) | 本次 ref |
| commit_sha | varchar(64) | |
| image_repo | varchar(255) | |
| image_tag | varchar(128) | |
| image_digest | varchar(128) | 构建产物 digest |
| params | longtext | 本次参数快照 JSON |
| message | text | 结构化失败原因 |
| log_key | varchar(255) | 日志文件键 |
| started_at | datetime | |
| finished_at | datetime | |
| duration_ms | bigint | |
| created_by | bigint, idx | |
| created_by_name | varchar(64) | |

索引：`(pipeline_id, id)`、`(status)`、`(gmt_create)`。

### 3.3 `cicd_triggers`（P2）

`pipeline_id` / `type(webhook|schedule)` / `secret_hash` / `config` / `enabled` / `last_triggered_at`。

### 3.4 凭据（不建表）

命名空间内 K8s Secret + label `pixiu.io/cicd-credential=git|registry`。API 只返回名称 / 类型 / 创建时间，永不返回值，审计记录创建与删除。

### 3.5 状态机

```
pending ──claim──▶ running ──┬─▶ succeeded
   │                         ├─▶ failed
   │                         ├─▶ timeout
   │                         └─▶ cancelled
   └──── cancel（未执行）────▶ cancelled
```

- 启动对账：`running` 的记录回查集群 PipelineRun；不存在或已结束则按真实状态收口。
- 进程重启：以集群事实源为准，平台表只做索引与审计。

---

## 4. 引擎适配层

只定义一层薄接口，Tekton 实现；未来加 Argo 时不动业务代码。

```go
type Engine interface {
    Name() string
    Status(ctx, cluster) (installed bool, version string, err error)
    CreateRun(ctx, cluster, ns string, spec RunSpec) (engineName string, err error)
    GetRun(ctx, cluster, ns, engineName string) (RunStatus, error)
    ListPods(ctx, cluster, ns, engineName string) ([]PodRef, error)
    StreamLogs(ctx, cluster, ns string, pod PodRef, follow bool) (io.ReadCloser, error)
    Cancel(ctx, cluster, ns, engineName string) error
}
```

- Tekton 实现使用 `dynamic.Interface`（参照 `pkg/controller/backup/velero.go` 的 GVR 模式），GVR：`tekton.dev/v1 pipelineruns`。
- Pod 通过 label `tekton.dev/pipelineRun=<engineName>` 过滤。
- 状态映射：`Succeeded=True → succeeded`；`False + reason=TimedOut → timeout`；`False → failed`；`Unknown → running`；删除且平台已标记 → `cancelled`。
- 取消：删除 PipelineRun（Tekton 原生语义），平台记录取消人与时间。

**为什么用内联 pipelineSpec**：
- 避免在每个租户命名空间预装 / 升级 Task；
- 规避 Tekton 跨命名空间 `taskRef` 限制；
- 规避离线环境 remote resolver / Tekton Hub 依赖。
- 代价：集群侧看不到可复用 Task，但产品 UI 自建，不影响。

---

## 5. 构建流水线（内置模板 `source-build`）

### 5.1 参数

`git-url` / `git-revision` / `dockerfile` / `context` / `image` / `build-args` / `registry-secret` / `git-secret` / `tls-verify`。

### 5.2 步骤

1. **clone**：内联 `git-clone` taskSpec（固定版本 / 镜像 digest），凭据 Secret 只读挂载。
2. **build-push**：Buildah
   - `buildah bud --isolation=chroot --storage-driver=vfs --layers ...`
   - `buildah push --digestfile /tmp/digest`
   - 凭据经 Secret 注入，脚本 `set +x` 防日志泄露
3. 输出 result `IMAGE_DIGEST` → 回写 run。

### 5.3 构建器说明

- Kaniko 已于 2025 年进入弃用 / 归档轨道，**不选 Kaniko**。
- P1 默认 Buildah；若目标环境 Buildah rootless 不可行，降级为 BuildKit rootless（见第 13 节 P0 验证）。

### 5.4 缓存与多架构

- P1 不做跨运行镜像层缓存（依赖基础镜像 pull + 集群内 registry），行为可预测。
- P2 可选：PVC 构建缓存或 registry cache（按集群维度）。
- 多架构（P2）：`target-platforms` + QEMU，需评估耗时。

### 5.5 安全上下文（P0 必验）

`runAsNonRoot`、drop capabilities、`seccompProfile: RuntimeDefault`、`allowPrivilegeEscalation: false`。
Buildah rootless 依赖 user namespace（k8s `hostUsers: false`）与 vfs；不满足时降级为专用构建命名空间的受限特权，或改 BuildKit rootless。

---

## 6. 触发与调度

- **手动**：`POST /pipelines/:id/runs` → 写 `pending` → worker 领取执行。
- **定时（P2）**：复用现有 cron 调度模式（参照 `cron_hpa`）。
- **Webhook（P2）**：
  - 公开端点 `/pixiu/hooks/cicd/{token}`，加入 `api/server/middleware/middleware.go` 的 `allowCustomRequest`（不放 `alwaysAllowPath`）。
  - 验签：GitHub `X-Hub-Signature-256`（HMAC）、GitLab `X-Gitlab-Token`、Gitee 兼容。
  - 时间戳窗口 + 投递 ID 去重；分支 / tag 过滤；单 token 限流；仅允许已启用 pipeline。

**队列模型**：复用应用商店的 `pending + Claim` 模式（`pkg/db/chart.go` `ClaimPendingInstall`）：
- worker 每 2–3s 捞取 pending → 原子 claim → 创建 PipelineRun；
- 多副本安全；启动时对账；
- 并发控制：单 pipeline 默认 1 个运行中，全局配额可配。

---

## 7. 日志与状态

- **执行期采集**：worker 启动后并发跟随各 TaskRun Pod 日志（client-go PodLogs follow），按步骤前缀写入 `cicd-logs/{runId}.log`（复用应用商店 `logSink` 思路）。
- **SSE**：`GET /runs/:id/logs?follow=1`，先回放文件再 follow，`text/event-stream` + Flush + 心跳。
- **前端**：fetch + ReadableStream（EventSource 无法携带 Authorization），复用 `K8sLogOutput`。
- **保留**：日志按天清理；每 pipeline 保留最近 N 次；PipelineRun 设置 `ttlSecondsAfterFinished` 自动回收 CR，平台 DB 保留历史。

---

## 8. API 设计

统一 `{code, result, message}` 信封；路由通过 `apiregistry.Group{...}.Register()` 注册，会自动把 `METHOD:path` 持久化为按钮权限。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/pixiu/cicd/clusters/:clusterId/engine-status` | Tekton 是否安装 / 版本 |
| GET | `/pixiu/cicd/pipelines` | 列表（clusterId / namespace / name 过滤、分页） |
| POST | `/pixiu/cicd/pipelines` | 创建 |
| GET | `/pixiu/cicd/pipelines/:id` | 详情 |
| PUT | `/pixiu/cicd/pipelines/:id` | 更新（带 resource_version） |
| DELETE | `/pixiu/cicd/pipelines/:id` | 删除 |
| POST | `/pixiu/cicd/pipelines/:id/runs` | 手动触发 |
| GET | `/pixiu/cicd/runs` | 运行列表（pipelineId / status / 分页） |
| GET | `/pixiu/cicd/runs/:id` | 运行详情（阶段、产物、耗时） |
| GET | `/pixiu/cicd/runs/:id/logs` | SSE 日志（follow 可选） |
| POST | `/pixiu/cicd/runs/:id/cancel` | 取消 |
| POST | `/pixiu/cicd/runs/:id/retry` | 重跑 |
| GET | `/pixiu/cicd/clusters/:c/namespaces/:n/credentials` | Secret 元数据列表 |
| POST | `/pixiu/cicd/clusters/:c/namespaces/:n/credentials` | 创建 Secret |
| DELETE | `/pixiu/cicd/clusters/:c/namespaces/:n/credentials/:name` | 删除 Secret |
| GET/POST/PUT/DELETE | `/pixiu/cicd/pipelines/:id/triggers`（P2） | 触发器 |
| POST | `/pixiu/hooks/cicd/:token`（P2） | 公开 webhook（验签） |
| GET | `/pixiu/cicd/templates`（P3） | 模板列表 |

新增错误码：`ErrPipelineNotFound` / `ErrPipelineExists` / `ErrRunNotFound` / `ErrEngineNotInstalled` / `ErrRunConflict` / `ErrCredentialInvalid`。

**创建流水线请求示例**：

```json
{
  "name": "demo-api",
  "cluster_id": 1,
  "namespace": "demo",
  "template": "source-build",
  "config": {
    "repo": {
      "url": "https://git.example.com/a/b.git",
      "ref": "main",
      "credential": "git-cred"
    },
    "build": {
      "dockerfile": "Dockerfile",
      "context": ".",
      "args": ["GO_VERSION=1.25"]
    },
    "image": {
      "registry": "harbor.example.com",
      "repository": "demo/api",
      "tag": "$COMMIT_SHORT",
      "credential": "reg-cred"
    },
    "advanced": {
      "timeout_seconds": 1200,
      "tls_verify": true
    }
  }
}
```

---

## 9. 前端方案

### 9.1 路由与菜单（前后端必须同步）

1. 新建 `src/router/modules/cicd.ts`，注册到 `src/router/modules/index.ts`。
2. `src/constants/menus.ts` 增加 `Cicd: 'cicd'`、`CicdPipelines: 'cicd.pipelines'`。
3. 后端 `pkg/rbac/menu/catalog.go` 增加对应菜单码（**不加则菜单不可见**，`MenuProcessor.ts` 默认隐藏未命中项）。
4. 一级菜单「CI/CD」，子页：流水线（含运行记录 Tab）。

### 9.2 页面与复用组件

| 页面 | 复用 |
|---|---|
| `views/cicd/index.vue` 流水线列表 | `ArtTable` / 列表页模板 / `StatusText` |
| 创建 / 编辑弹窗 | `StandardFormDialog` + 手写步骤（基本信息 → 代码源 → 构建 → 镜像 → 高级）；当前无通用向导组件，参考 `views/container/cluster/deploy/index.vue` |
| 运行详情抽屉 | 全局类 `object-detail-drawer` + `confirm-info-grid` |
| 日志抽屉 | `K8sLogOutput` + fetch SSE（参照 `appstore/install-log-drawer.vue`） |
| 引擎未装引导 | 仿 `velero-status-tip.vue`，跳应用商店安装 Tekton |
| API 层 | 新增 `src/api/cicd.ts`（camelCase ↔ snake_case 转换，`pixiuAxios`） |

### 9.3 状态配色与开关

- pending 灰 / running 蓝 / succeeded 绿 / failed 红 / cancelled 灰 / timeout 橙。
- 新增 `CICD_UI_VISIBLE`（类似 `HELM_UI_VISIBLE`），支持灰度与回滚。

---

## 10. 权限、租户与审计

- **写权限**：pipeline 创建 / 删除 → root / admin 或租户内有集群操作权；运行 / 取消 → pipeline 归属人、admin、root（沿用 `util.CheckRoot / CheckAdmin / CheckResourceOwner`）。
- **读权限**：由菜单码 + 角色 API 授权共同控制；列表按 `tenant_id` 过滤，禁止跨租户。
- **集群可达性**：复用现有集群权限模型；平台以集群 kubeconfig 代理执行，因此必须在 handler 层做归属校验，不能只依赖集群 RBAC。
- **审计**：现有审计中间件已记录非 GET；补充 webhook 触发的独立审计（来源 IP、delivery ID、结果）。Webhook 不计入用户审计。

---

## 11. 配置项（`config.yaml`）

```yaml
cicd:
  enabled: true
  workers: 2
  poll_interval: 3s
  log_retention_days: 14
  max_running_per_pipeline: 1
  max_pending_runs: 100
  build:
    tool: buildah
    isolation: chroot
    storage_driver: vfs
    insecure_registry: false
  webhook:
    enabled: false
```

---

## 12. 非功能与运维

- **HA**：claim 保证单执行；确定性 CR 命名支持「创建成功但回写失败」的对账。
- **超时**：控制面 ctx + Tekton `timeouts` 双保险。
- **配额**：全局 / 单 pipeline 并发、pending 上限、日志大小上限。
- **离线**：Tekton 控制器、git-clone、buildah 镜像全部 pin digest，随离线包分发；禁用 remote resolver / Tekton Hub。
- **版本矩阵**：锁定 Tekton 稳定版本，升级前按支持的 k8s 版本回归。
- **清理**：run 记录、日志文件、已完成 CR 三条保留策略。
- **可观测**：运行数、失败率、平均耗时、队列长度（P2 可接现有监控）。

---

## 13. 风险与 P0 前置验证

| 风险 | 等级 | 应对 |
|---|---|---|
| Buildah rootless 在目标内核 / 发行版不可用 | 高 | **P0 spike 先验证**；不可用则切 BuildKit rootless 或专用构建命名空间受限特权 |
| Tekton 与 k8s 版本不兼容 | 中 | 锁定版本 + 支持矩阵 |
| 构建容器逃逸 | 高 | 非特权默认、drop caps、seccomp、独立命名空间、NetworkPolicy 可选 |
| 凭据泄露（日志 / DB） | 高 | 只存 Secret、`set +x`、日志脱敏、审计 |
| 多集群日志采集性能 | 中 | 并发上限、按需 follow、日志文件上限 |
| 离线包体积 / 完整性 | 中 | digest pin + 离线清单 |
| 私有仓库 HTTP / insecure | 中 | 默认校验 TLS，显式开关 |
| 长日志 / 卡死 | 低 | 大小上限 + 超时 |

---

## 14. 分期任务计划

### P0 —— 技术验证（先做，不写业务代码）

- 目标：确认 Buildah 在目标集群 rootless 可构建并推送。
- 验证：kind / 真机上跑 clone → buildah → push，拿到 digest；记录内核 / 发行版 / 安全上下文组合。
- 输出：可行 / 不可行结论；不可行则触发构建器降级决策。

### P1 —— 控制面 MVP（手动源码构建闭环）

| # | 任务 | 改动点 | 验证 |
|---|---|---|---|
| 1 | 表与 DAO | `pkg/db/model/cicd.go`、`pkg/db/cicd.go`、`pkg/db/factory.go` | 迁移成功、CRUD 单测 |
| 2 | 类型与控制器 | `pkg/types/cicd.go`、`pkg/controller/cicd/*`、`pkg/controller/controller.go` 注册 | 单测 |
| 3 | 路由 | `api/server/router/cicd/*`、`api/server/router/router.go` | 接口可用、按钮自动入库 |
| 4 | Tekton 适配 | dynamic client、内联 pipelineSpec、状态映射 | fake client 单测 |
| 5 | 队列 worker | claim、重试、启动对账、并发限制 | 多副本重复执行验证 |
| 6 | 日志采集与 SSE | per-run 文件、Pod follow、回放 | 端到端日志 |
| 7 | 凭据 | Secret CRUD + label 发现 | 不返回明文 |
| 8 | 前端 | 列表 / 向导 / 详情 / 日志 / 引擎引导 / 菜单 / feature flag | 浏览器回归 |
| 9 | 权限与审计 | 菜单码、按钮授权、归属校验 | 越权用例 |
| 10 | 文档 | README / 设计说明 / FAQ | 离线安装步骤 |

**P1 成功标准**：用户在 UI 上选择 Git 仓库与 Dockerfile → 触发运行 → 实时日志 → 外部仓库出现镜像 → 运行记录含 digest，可取消 / 重跑；无权用户无法跨租户访问。

### P2 —— 触发与交付

- Webhook（验签 / 去重 / 限流）、定时触发
- 部署联动（构建成功后更新 Deployment 镜像 / 触发 Helm 升级）
- 通知（复用告警渠道 / 邮件）
- 配额、构建缓存、多架构

### P3 —— 产品增强

- 模板市场、可视化编排增强
- 审批门禁
- Trivy 扫描、SBOM、cosign / Tekton Chains 签名溯源
- 外部 Jenkins / GitLab 连接器（状态展示 / 触发 / 跳转）

---

## 15. 验证矩阵

| 类型 | 内容 |
|---|---|
| 单测 | 状态映射、模板渲染、验签、claim、归属校验 |
| 集成 | fake dynamic client；kind + Tekton + 本地 registry 全链路 |
| 真机 | Ubuntu / 目标国产 OS、containerd / docker、多集群、离线环境 |
| 故障 | 引擎缺失、命名空间不存在、凭据错误、仓库不可达、超时、取消、Pod 崩溃、控制面重启 |
| 前端 | 组件测试 + 浏览器回归（列表 / 向导 / 日志 / 空态 / 权限） |

---

## 16. 回滚方案

- **开关**：`cicd.enabled=false` + `CICD_UI_VISIBLE=false` → 菜单隐藏、worker 不启动、API 拒绝。
- **数据**：两表为纯增量，无需回滚 DDL；历史 run 只读。
- **集群**：已完成 PipelineRun 按 TTL 自动清理；卸载 Tekton 不影响 pixiu。
- **代码**：单个模块目录，可整体移除。

---

## 17. 文档与沉淀（开发阶段同步）

- 设计说明：本文 + `docs/cicd/`
- 使用说明：流水线配置字段、凭据准备、webhook 配置
- 离线安装：Tekton / Buildah / 镜像清单
- FAQ：构建失败排查、rootless 限制、仓库认证
- 经验沉淀：完成后如有可复用模板，补充到 `CLAUDE.md`

---

## 18. 开放问题

开发前需确认：

1. 流水线归属按「租户 + 集群 + 命名空间」是否可以？
2. P1 是否需要「运行记录独立菜单」，还是放在流水线页的 Tab 里？
3. 构建是否允许在目标业务集群直接跑（而不是统一构建集群）？这影响隔离与网络策略。
4. 外部镜像仓库是否统一要求 HTTPS + 有效证书（默认拒绝 insecure）？

---

## 附录：可复用代码参照（当前分支 `dev/20261124-6`）

| 能力 | 位置 |
|---|---|
| 菜单目录（后端） | `pkg/rbac/menu/catalog.go` |
| 菜单码（前端） | `src/constants/menus.ts`、`src/router/modules/index.ts` |
| 路由注册 + API 自动入库 | `api/server/router/apiregistry/apiregistry.go` |
| 鉴权放行白名单 | `api/server/middleware/middleware.go`（`alwaysAllowPath` / `allowCustomRequest`） |
| 权限校验 | `pkg/controller/util/util.go`（`CheckRoot` / `CheckAdmin` / `CheckResourceOwner`） |
| DB 工厂 | `pkg/db/factory.go`（`ShareDaoFactory`） |
| 异步队列 + 抢占 | `pkg/controller/apps/worker.go`、`pkg/db/chart.go`（`ClaimPendingInstall`） |
| SSE 日志（文件回放 + follow） | `api/server/router/apps/release_routes.go`、`pkg/controller/apps/worker.go` |
| 容器日志 SSE（另一种模式） | `api/server/router/plan/task_routes.go`、`pkg/controller/plan/plan_task.go` |
| CRD 动态客户端 | `pkg/controller/backup/velero.go` |
| 多集群客户端 | `pkg/client`（`Cluster().GetClusterSetByName`） |
| 后台 worker 启动 | `cmd/app/server.go`（`runers` 列表） |
| 前端列表模板 | `src/components/core/tables/art-table` |
| 前端表单弹窗 | `src/components/business/standard-form-dialog.vue` |
| 前端日志组件 | `src/components/kubernetes/k8s-log-output.vue` |
| 引擎状态引导 | `src/views/safeguard/backup/modules/velero-status-tip.vue` |
| 多步向导参考 | `src/views/container/cluster/deploy/index.vue` |
