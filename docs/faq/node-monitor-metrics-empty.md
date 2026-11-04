# FAQ-007 节点监控 CPU、内存指标无数据（节点标签不匹配）

- **分类**：监控与数据源
- **涉及组件**：节点详情 / Node 监控 / Prometheus / cAdvisor / kube-state-metrics

## 现象

集群已关联 Prometheus 数据源，实时查询可以查到部分指标，但节点详情的 CPU、内存监控图显示「暂无数据」。

也可能出现节点详情有数据，而集群监控的「Node 监控」总览中 CPU、内存使用率为空。两个页面的查询条件和依赖不完全相同，需要分别排查。

> 如果集群未关联可用的 Prometheus 数据源，请先参阅 [FAQ-003](./monitor-no-data.md)。

## 根因

节点详情页使用 cAdvisor 容器指标，并按 Kubernetes 节点名称匹配 `node` 标签。部分 Prometheus 只在 `instance` 中保存节点名称，导致指标存在但查询无法匹配。

「Node 监控」总览还依赖 `kube_node_status_allocatable`；若该指标缺失或两侧 `node` 值不一致，使用率也可能为空。

## 验证方式

1. 在 **监控告警 → 实时查询** 中，选择目标集群节点页面使用的 Prometheus 数据源。
2. 查询容器指标标签，并与 Kubernetes 节点名称比较：

   ```promql
   count by (job, node, instance) (
     container_memory_working_set_bytes{container!="",image!=""}
   )
   ```

   如果 `instance` 是节点名，但没有匹配的 `node` 标签，可能存在标签不匹配。

3. 将 `<节点名>` 替换为实际名称，查询节点内存：

   ```promql
   sum(container_memory_working_set_bytes{node="<节点名>",container!="",image!=""})
   ```

   如果按 `node` 无结果，而改为 `instance="<节点名>"` 后有结果，说明该指标存在节点标签不匹配。CPU 可同样检查 `container_cpu_usage_seconds_total`。

4. 如果原始指标也无数据或查询报错，检查 cAdvisor 抓取目标和数据源连通性。
5. 如果仅 Node 监控总览使用率为空，查询可分配资源，并检查其 `node` 是否与容器指标一致：

   ```promql
   sum by (node) (kube_node_status_allocatable{resource="memory"})
   ```

   CPU 对应 `resource="cpu"`。若指标不存在，检查 kube-state-metrics 的采集。

## 解决方案

### 为节点 cAdvisor 采集目标添加 node 标签

适用于 cAdvisor 任务通过 Kubernetes **`role: node`** 服务发现，且已确认 `__meta_kubernetes_node_name` 是目标集群的节点名称。

1. 备份 Prometheus 当前配置，找到采集 cAdvisor 的任务，例如 `kubernetes-nodes-cadvisor`。
2. 在该任务已有的 `relabel_configs` 列表中追加以下规则，保留原有配置，不要重复创建 `relabel_configs` 字段：

   ```yaml
   - source_labels: [__meta_kubernetes_node_name]
     target_label: node
   ```

3. 校验完整配置，并按该部署的配置重载机制使其生效。ConfigMap 保存成功不代表 Prometheus 一定已经重新加载配置。
4. 等待至少一个采集周期，重新执行「验证方式」中的查询，确认 cAdvisor 指标同时具有正确的 `node` 和 `instance`，再刷新节点详情页。

## 备注 / 风险

- 不同集群、数据源和页面需分别验证，不能将所有无数据归因于标签。
- 上述规则仅适用于 `role: node`；其他发现方式不可直接照搬。
- 标签变更可能影响告警、仪表盘和记录规则；只对新采集的数据生效，不会补充历史数据。
