# FAQ-007 节点监控同一节点出现两行（节点名与主机名不一致）

- **分类**：监控与数据源
- **涉及组件**：节点监控大盘（Node 监控）/ Prometheus / node-exporter

## 现象

「监控 → 节点监控」表格中，同一台机器出现两行，指标被拆开：

| 行 | 节点名形态 | 有值列 | 空（-）列 |
|---|---|---|---|
| 1 | 点分 IP，如 `10.172.30.75` | 状态、Pod 数、总 CPU、CPU 使用率、总内存、内存使用率 | 剩余磁盘、发送、接收、连接数 |
| 2 | 短横线 IP，如 `10-172-30-75` | 剩余磁盘、发送、接收、连接数 | 其余全部 |

点第二行进入节点详情抽屉时，CPU/内存趋势为空；点第一行则磁盘/网络相关面板为空。

## 根因

同一物理节点在 Prometheus 中存在两套节点标识：

1. **Kubernetes 节点名**（点分 IP）：来自 kube-state-metrics 与 cAdvisor 的 `node` 标签，形如 `10.172.30.75`
2. **操作系统主机名**（短横线 IP）：来自 node-exporter 的 `node_uname_info.nodename`，形如 `10-172-30-75`

Node 监控的磁盘/网络/连接数等面板，主查询通过 `sum by (nodename) (...) * on(instance) group_left(nodename) node_uname_info` 关联主机名，再用 `label_replace` 把它写成 `node` 标签；而 CPU/内存/状态/Pod 数面板直接使用 KSM/cAdvisor 的 `node` 标签。Node 监控表格把所有面板的 `node`/`nodename` 取并集作为行，且不做名称归一化，于是同一台机器被拆成两行，每行只命中一半面板。

此外，`sum by (nodename)` 聚合丢掉了 `instance` 标签，前端基于 `instance` 的兜底匹配也无法把两行合并。

**根本原因**：节点操作系统主机名与 Kubelet 注册的 Node 名不一致。常见于部署时 inventory 主机名使用 IP 短横线形式（如 `10-172-30-75`），而 kubelet 注册节点名使用点分 IP（`--node-name` / `--hostname-override`）；部分云镜像也会默认把主机名设置为 IP 的点换短横线形式。

## 验证方式

1. 确认两个名字不同：

```bash
kubectl get nodes -o wide                 # Node 名：10.172.30.75
ssh 10.172.30.75 hostname                 # 主机名：10-172-30-75
```

2. 在 Prometheus 中确认两套标签集合不一致：

```promql
count by (nodename) (node_uname_info)
count by (node) (kube_node_status_allocatable{resource="cpu"})
```

3. 确认 node-exporter 抓取配置未把 Pod 所在节点名写入 `node` 标签：

```bash
kubectl -n <monitoring 命名空间> get servicemonitor -l app.kubernetes.io/name=prometheus-node-exporter -o yaml
```

若 `spec.endpoints[].relabelings` 为空或没有 `__meta_kubernetes_pod_node_name → node`，即命中本问题。

## 解决方案（根治：统一 node-exporter 的 node 标签）

目标：让 node-exporter 指标自带与 KSM 一致的 `node` 标签（Kubernetes 节点名），面板查询不再依赖主机名。

### 步骤 1：为 node-exporter 增加 node 标签 relabel

使用 kube-prometheus-stack 时，在其 values 中追加（Helm 升级即可生效）：

```yaml
prometheus-node-exporter:
  prometheus:
    monitor:
      enabled: true
      relabelings:
        - sourceLabels: [__meta_kubernetes_pod_node_name]
          regex: ^(.*)$
          targetLabel: node
          replacement: $1
          action: replace
```

> 若使用独立 prometheus-node-exporter chart，则把上述内容放在 `prometheus.monitor.relabelings` 下。
> 若为手写 scrape_config（非 ServiceMonitor），在 node-exporter 的 job 下增加：
>
> ```yaml
> relabel_configs:
>   - source_labels: [__meta_kubernetes_pod_node_name]
>     target_label: node
>     action: replace
> ```

升级后确认指标已带点分 IP 的 `node` 标签：

```promql
count by (node) (node_uname_info)
```

应输出 `10.172.30.75` 等 K8s 节点名（不再是 `10-172-30-75`）。

### 步骤 2：面板查询改用 node 标签

将 Node 监控中磁盘/网络/连接数/Load/在线时间等面板的主查询由「`sum by (nodename) ... * on(instance) group_left(nodename) node_uname_info` + `label_replace`」改为直接按 `node` 聚合，例如：

```promql
sum by (node) (node_filesystem_avail_bytes{fstype!~"tmpfs|overlay|squashfs|nsfs",mountpoint="/"})
sum by (node) (rate(node_network_transmit_bytes_total{device!="lo"}[5m]))
sum by (node) (node_netstat_Tcp_CurrEstab)
```

Node 监控表格的行集合以 KSM 的 `node` 标签为准，不再混入 `nodename`，两行即合并为一行。

> 若集群不具备节点元数据（`__meta_kubernetes_pod_node_name` 不存在，例如外部静态抓取），可退而使用 `instance` 作为节点标识：`label_replace(sum by (instance)(...), "node", "$1", "instance", "^([^:]+):.*")`，前提是 node-exporter 的 `instance` 就是节点 IP（kube-prometheus-stack 默认 hostNetwork 满足）。

### 替代方案（不建议）

直接修改节点操作系统主机名使其与 Node 名一致。会影响证书、/etc/hosts、已有监控与自动化，风险高，仅建议作为新建集群的规范约束。

## 备注 / 风险

- 该问题不是采集断点，也不是前端把 `.` 替换成 `-`；是两套标签口径未统一
- 修改 relabel 后需等待一次抓取周期（默认 30s）再刷新监控页
- 修改后同步检查外部 Grafana 看板与告警规则：使用 `nodename` 的应一并改为 `node`
- 该修复需集群侧（Helm values）与平台侧（监控面板查询口径）同时生效；仅改一侧仍可能残留半行空值
- 新建集群建议在部署规范中统一「节点主机名 = Kubernetes 节点名」
