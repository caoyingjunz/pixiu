# 集群 Node 扩容（add-node）

## 目标

在集群详情 → 节点管理页，把主机池中的一台或多台 node 节点加入**已存在**的 K8s 集群。扩容复用部署计划体系：每个集群对应一个 **扩容计划**（`plans.type=1`，添加节点时自动创建），扩容节点挂在扩容计划下，也支持由其他工具部署、导入 pixiu 的集群。

## 适用条件（能力探测）

`GET /pixiu/clusters/:clusterId/expansion-config` 会探测并返回：

- 集群可连接；
- `kube-system/kubeadm-config` 存在（kubez add-node 本质是 `kubeadm join`）；
- 现有节点容器运行时为纯 containerd；
- Kubernetes 版本、imageRepository、controlPlaneEndpoint（来自 kubeadm-config）；
- 若集群由 pixiu 部署（`clusters.plan_id > 0`），额外返回 plan 的默认 os_image / network_interface。

不满足条件时 `supported=false`，前端禁用「添加节点」并展示 `reason`。

## 使用流程

1. 节点管理页点击「添加节点」：
   - 首次添加时检查该集群的扩容计划（`plans.type=1`）是否已存在，不存在则自动创建（`expand-cluster-<clusterId>`）；
   - **从主机库选择**：仅显示未绑定主机；仅允许 node 角色主机，名称或 IP 已存在于集群中的主机不允许添加；绑定后节点 `plan_id` 写入扩容计划 id；
   - **手动录入**：填写主机名/IP/SSH 凭据，直接创建并挂到扩容计划（`plan_id` = 扩容计划 id）；名称或 IP 已存在于集群中时拒绝添加。
2. 添加后的节点状态为 `未添加`（`nodes.status=0`），不会立即执行任何操作。
3. 勾选一个或多个未加入节点，点击「扩容」：
   - 选择操作系统镜像（用于解析 runner 镜像，bootstrap 时也用于安装对应版本组件）；
   - 可配置网卡、镜像仓库、主机名修改；
   - kubeconfig 权限不足或 apiserver 地址不可达时，可在高级选项中覆盖 api_server / token / ca_cert_hash。
4. 执行状态：`未添加 → 添加中 → 已加入（K8s Node Ready 后显示运行中）`；失败为 `添加失败`，可重试。

## 执行链路

```
创建 expansion（cluster_expansions，plan_id 关联扩容计划）
  → 节点置 添加中
  → worker（执行扩容计划）：
      1. 通过集群 API 获取 K8s 必须的 token 和配置：
         解析 join 参数：apiserver / CA hash / bootstrap token / live version
      2. 渲染 <workdir>/expansions/<id>/{hosts,multinode,globals.yml,ssh/}
      3. 容器 pixiu-expand-<id>-bootstrap  COMMAND=bootstrap-servers（必经步骤）
      4. 容器 pixiu-expand-<id>-add-node  COMMAND=add-node（与 kubez-ansible 对齐：join 参数加入已有集群，非 deploy-node）
      5. 按 K8s 实时节点列表校正节点状态
      6. 清理含 token/SSH 私钥的渲染目录
```

join 参数来源：

| 参数 | 来源 |
|---|---|
| apiserver | 请求覆盖 → kubeadm-config controlPlaneEndpoint → kubeconfig server |
| CA hash | 请求覆盖 → kubeconfig CA 证书公钥 SHA256（纯 hex） |
| bootstrap token | 请求覆盖 → Kubernetes API 创建 `bootstrap-token-*` Secret（TTL 2h） |
| kube_release | kubeadm-config kubernetesVersion → 集群实际版本 |
| image_repository | 请求覆盖 → kubeadm-config imageRepository |

## 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/pixiu/clusters/:clusterId/expansion-config` | 能力探测与默认参数 |
| GET | `/pixiu/clusters/:clusterId/nodes` | 扩容节点列表（不含凭据） |
| POST | `/pixiu/clusters/:clusterId/nodes` | 添加节点（绑定已有主机 / 手动创建） |
| DELETE | `/pixiu/clusters/:clusterId/nodes/:nodeId` | 移除未加入节点（解绑回主机池） |
| POST | `/pixiu/clusters/:clusterId/expansions` | 创建并启动扩容 |
| GET | `/pixiu/clusters/:clusterId/expansions` | 扩容历史 |
| GET | `/pixiu/clusters/:clusterId/expansions/:expansionId` | 扩容详情 |
| GET | `/pixiu/clusters/:clusterId/expansions/:expansionId/logs?step=bootstrap\|add-node` | 扩容日志（SSE） |

## 数据模型

- `plans` 新增 `type`（0=普通部署计划，1=集群扩容计划，默认 0）与 `cluster_id`（扩容计划关联的目标集群）。
- `nodes` 新增 `cluster_id`、`status`、`status_message`；扩容节点同时写入扩容计划的 `plan_id` 与 `cluster_id`（普通计划节点仅 `plan_id`）。
- 新增 `cluster_expansions`：每次扩容一条记录（plan_id、目标节点、参数、状态、步骤、消息、时间）。
- 计划向导「关联节点」已过滤 `cluster_id != 0`，避免扩容节点被部署计划再次绑定；扩容计划不通过计划页 Start（节点管理页发起扩容执行），存在绑定时不可删除。

## 限制

- 仅支持 **worker（role=node）**，不支持 master/etcd 扩容；
- 仅支持 **containerd** 运行时；
- 仅支持 **local** 执行模式；agent 模式（单向网络边缘）暂不支持，后续需扩展 job 模型；
- bootstrap 需要新节点可访问系统包仓库；离线环境需配置自定义 repo；
- 扩容不改集群状态；扩容计划（type=1）仅承载扩容节点与执行记录，不参与普通部署流程；
- 已加入节点不允许在扩容页移除，请使用集群节点移除流程。

## 验证

- 单元测试：`go test ./pkg/controller/clusterexpand/... ./pkg/planrender/...`
- 前端：`vitest run src/constants/node-expand-status.spec.ts`；`vite build`
- 端到端：测试集群（kubeadm + containerd）添加 1/2 个 node，确认 task 日志、节点 Ready、状态流转；错误 SSH 密码验证失败态与重试；扩容期间并发部署/扩容被拒绝。
