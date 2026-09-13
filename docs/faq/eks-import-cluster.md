# FAQ-002 无法通过「导入集群」管理 AWS EKS

- **分类**：容器服务 / 集群接入
- **涉及组件**：导入集群（kubeconfig）/ Cluster Agent（隧道）/ EKS IAM 认证

## 现象

通过「导入集群」接入 AWS EKS 时常见失败形态：

1. 粘贴 `aws eks update-kubeconfig` 生成的 kubeconfig 后，**测试连接 / 创建集群失败**（连接 Kubernetes API 失败）
2. 短暂连通后，约十余分钟后集群状态异常、代理与同步失效
3. 仅私网 API endpoint 的 EKS，直连模式一直超时不可达
4. 连通成功，但创建失败回滚（无法创建 `pixiu-system` 或注入 `pixiu-view`）

## 根因

Pixiu「导入集群」按 **静态 kubeconfig + 控制面发起认证** 设计：kubeconfig 落库后由服务端 `client-go` 反复复用。

EKS 默认 kubeconfig 使用 **exec 凭据插件**：

```yaml
user:
  exec:
    apiVersion: client.authentication.k8s.io/v1beta1
    command: aws
    args: ["eks", "get-token", "--cluster-name", "...", "--region", "..."]
```

与现有模型冲突：

| 点 | 说明 |
|---|---|
| 运行环境 | Pixiu 服务端镜像通常 **无 AWS CLI / IAM 角色**，无法执行 `aws eks get-token` |
| Token 寿命 | 即便本机先写入短期 token，EKS token 约 **15 分钟**过期，无法长期管理 |
| 隧道模式 | 仅解决控制面到 API 的 **网络拨号**；认证仍在控制面，**不解决** exec 问题 |
| 权限不足 | Ping（`/version`）成功后，若无建 Namespace / ClusterRole 权限，导入仍会回滚 |

结论：不是显式禁用 EKS，而是 **不支持以官方 IAM exec kubeconfig 作为长期接入凭据**。自建集群常见的 client-cert / 长期 ServiceAccount token 与当前模型匹配。

## 验证方式

1. 查看待导入 kubeconfig 的 `users[].user` 是否含 `exec`（尤其 `command: aws`）
2. 在 Pixiu 所在网络探测 EKS API（公网 / 私网）：

```bash
curl -vk --connect-timeout 5 https://<eks-api-endpoint>/version
```

3. 确认当前身份映射到 K8s 后是否具备创建 Namespace、读写 ClusterRole 的权限（接近 cluster-admin）

## 解决方案

### 方案 A：ServiceAccount Token kubeconfig（推荐，与 Kuboard 同类）

在 EKS 上创建管理用 SA，签发长期 token，再拼成 **无 exec** 的 kubeconfig 导入 Pixiu（公网用直连；私网用隧道且 `server` 填 Agent 可达的私网 API 地址）。

示例（可按环境改名）：

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: pixiu-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pixiu-admin
  namespace: pixiu-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: pixiu-admin-crb
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: pixiu-admin
  namespace: pixiu-system
---
apiVersion: v1
kind: Secret
type: kubernetes.io/service-account-token
metadata:
  annotations:
    kubernetes.io/service-account.name: pixiu-admin
  name: pixiu-admin-token
  namespace: pixiu-system
EOF

TOKEN=$(kubectl -n pixiu-system get secret pixiu-admin-token -o jsonpath='{.data.token}' | base64 -d)
# 从现有 kubeconfig 取 server / CA，或从 AWS 控制台获取后填入下方
```

导入用 kubeconfig 形态（**不要**再带 `exec`）：

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://<eks-api-endpoint>
    certificate-authority-data: <base64-ca>
  name: eks
users:
- name: pixiu-admin
  user:
    token: <上一步 TOKEN>
contexts:
- context:
    cluster: eks
    user: pixiu-admin
  name: eks
current-context: eks
```

说明：Pixiu 当前需上传 **完整 kubeconfig**，认证方式相同。

### 方案 B：私网 EKS + 隧道

1. 用本机 AWS 身份先将 Cluster Agent 部署进集群  
2. 导入时选择 **隧道**，kubeconfig 仍使用方案 A 的 **SA token**（`server` 对 Agent 可达）  
3. 勿指望「隧道 + 官方 exec kubeconfig」——网络通了认证仍会失败  

### 方案 C：产品级原生 EKS（后续能力）

服务端集成 AWS SDK，按 cluster ARN / region / IAM 角色自动刷新 token。当前版本 **未实现**；在落地前请使用方案 A/B。

## 备注 / 风险

- SA 绑定 `cluster-admin` 权限极大，生产环境建议收紧为平台所需最小权限，并做好 token 轮换与泄露管控  
- 新版 Kubernetes 默认不再自动为 SA 挂载 token，需显式创建 `kubernetes.io/service-account-token` Secret（见方案 A）  
- 导入账号至少能创建/使用 `pixiu-system` 相关资源；权限不足会出现「连通后创建失败」  
- 勿将本机 `~/.kube/config` 里带 `aws eks get-token` 的片段直接当作长期凭据导入
