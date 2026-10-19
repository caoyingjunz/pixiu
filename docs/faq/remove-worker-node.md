# FAQ-005 移除 k8s 集群中的 Worker 节点

- **分类**：容器服务 / 节点管理
- **涉及组件**：kubeadm / kubelet / kubectl / 容器运行时 / Pixiu 节点管理

> 范围说明：本文只覆盖 **Worker 节点**的移除。控制面节点（涉及 etcd member、API Server / Controller Manager / Scheduler 静态 Pod、VIP / LB）不在本文范围，需按控制面专用流程处理。

## 现象

适用于集群缩容、主机下线/报废、重装系统、节点迁移等场景。常见误区与现象：

1. **在 Pixiu 集群详情 → 节点管理 → 删除节点**：K8s 里看不到节点了，但主机上的 kubelet 和容器/服务仍在运行；这不是完整清理，kubelet 凭据有效时节点还可能重新注册。
2. **只执行 `kubectl delete node`**：与在 Pixiu 节点管理页删除等价，只删除 Node 对象；节点上的 kubelet 仍在运行、凭据仍有效，节点可能重新注册回集群。
3. **未清理旧节点就重新 join 同名节点**：出现证书、端口、CNI 残留冲突（如 10250 端口占用、旧集群 CA 配置残留、CNI 网桥/接口冲突）。
4. 节点已下线或无法登录，但没有先在控制面摘除，后续误以为「已经删除」。

## 根因

- 仅删除 Node 对象不会停止 kubelet，也不会清理 kubelet 证书、CNI 配置、iptables/IPVS 规则；kubelet 凭据有效时可重新注册节点。
- **K8s 里看不到节点 ≠ 节点已清理**：完整移除仍需停止 kubelet 并做主机侧清理。完整移除 = **业务迁移（cordon/drain）+ 控制面摘除（删除 Node 对象）+ 主机侧清理（停止 kubelet + 清理证书/CNI/网络规则）**，三步缺一不可。

## 验证方式

操作前在控制面执行以下检查，并确认预期结果：

```bash
kubectl get nodes -o wide
```

预期：确认目标节点在列表中，且 **节点名是 Kubernetes 节点名（通常为主机名，不是 IP）**；后续命令统一使用该名称，不要误用 INTERNAL-IP。

```bash
kubectl describe node <NODE> | grep -E 'Roles|Taints'
```

预期：Worker 节点的 `Roles` 为 `<none>`（或仅显示主机名相关标签），**不包含** `control-plane` / `master`；`Taints` 中不应出现 `node-role.kubernetes.io/control-plane:NoSchedule`。若含控制面角色，立即停止，按控制面流程处理。

```bash
kubectl get pods -A --field-selector spec.nodeName=<NODE> -o wide
```

预期：该命令会列出该节点上的全部 Pod；其中 DaemonSet Pod 与静态 Pod（API 中显示为镜像 Pod）不会在 drain 时被驱逐，其余业务 Pod 才是需要迁移/确认可中断的工作负载。记录清单后再执行 drain。

```bash
kubectl get pv -o wide
```

预期：确认是否存在绑定到该节点的本地 PV/本地存储——查看 `NODE AFFINITY` 列（版本支持时），或从 `kubectl get pv -o yaml` 的 `spec.nodeAffinity` 确认。本地 PV 数据不会随 Pod 迁移，需先迁移或备份。

## 解决方案

节点侧命令默认以 root 执行；非 root 用户请在节点侧命令前加 `sudo`（尤其 `systemctl` / `rm` / `iptables` / `ipvsadm`）。

### 1. 前置检查

- 确认目标是 Worker 节点；若 `Roles` 中含 `node-role.kubernetes.io/control-plane`，停止操作，按控制面专用流程处理。
- 确认剩余节点容量足够承接被驱逐的 Pod；检查 PDB（PodDisruptionBudget）：

  ```bash
  kubectl get pdb -A
  ```

- 检查本地 PV / hostPath / emptyDir 等数据依赖，先迁移或备份；本地存储数据不会自动迁移。
- 如节点同时是外部负载均衡 / Ingress VIP 的后端，先从 LB 摘除，再执行节点移除。
- 节点已 NotReady / 无法登录时：可先在控制面执行 `kubectl delete node <NODE>`，主机恢复后再补做主机侧清理，或直接重装系统。

### 2. 封锁节点（cordon）

```bash
kubectl cordon <NODE>
kubectl get node <NODE>   # 预期 STATUS 含 SchedulingDisabled
```

说明：cordon 只标记节点不可调度，已有 Pod 不受影响，真正的迁移由下一步 drain 完成。

### 3. 驱逐 Pod（drain）

```bash
kubectl drain <NODE> --ignore-daemonsets --delete-emptydir-data
```

说明：

- DaemonSet Pod 不会被驱逐，`--ignore-daemonsets` 为必需参数（否则 drain 直接报错退出）。
- `--delete-emptydir-data` 会删除 emptyDir 数据，**确认无数据依赖再执行**。
- 仅当存在不受控制器管理的 Pod（裸 Pod）时才考虑 `--force`，且需明确其删除后不会自动重建。
- PDB 可能阻止驱逐，先检查 `kubectl get pdb -A`；**不要长期使用 `--disable-eviction` 绕过**，那会使 PDB 保护失效。

### 4. 停止 kubelet 并从集群摘除节点

在待移除节点上执行：

```bash
systemctl stop kubelet
```

然后在控制面删除 Node 对象，两种方式等价：

- Pixiu 集群详情 → 节点管理 → 删除节点
- 命令行：

```bash
kubectl delete node <NODE>
```

说明：

- 两种方式都只删除 K8s Node 对象，不会停止主机侧服务，也不会清理证书/CNI/iptables。
- **先停 kubelet 再删除**，可避免 kubelet 凭据仍有效时节点重新注册；若无法先登录节点，则删除后尽快做主机侧清理。
- **如果已经先在 Pixiu 节点管理页删除了节点**：Node 对象已消失，无法再对该节点执行 drain。应尽快登录主机停止 kubelet 并做手工清理，同时检查原节点上的业务 Pod 是否已被控制器重建或已丢失；若 kubelet 重新注册导致节点再次出现，先停 kubelet，再删除一次。仍可在主机上执行 `kubeadm reset -f`，再按第 5 节清理残留。
- 推荐顺序：cordon → drain → 主机停止 kubelet → 删除 Node（Pixiu 页面或 kubectl）→ 节点执行 `kubeadm reset -f` → 主机侧手工清理残留。

### 5. 节点侧 reset 与手工清理

若第 4 步尚未停止 kubelet，先在节点上执行：

```bash
systemctl stop kubelet
```

在 kubeadm 部署的集群中，执行节点重置：

```bash
kubeadm reset -f
```

说明：

- `kubeadm reset` **不会停止 kubelet**；若前面尚未停止，请先执行 `systemctl stop kubelet`。
- 多 CRI 环境按需加 `--cri-socket`（如 `unix:///run/containerd/containerd.sock`）。
- 非 kubeadm 部署的集群跳过此步，按对应安装工具的节点重置方式处理。
- `kubeadm reset` 不会完整清理 CNI 配置、iptables/IPVS 规则，下面继续手工清理。

删除 Kubernetes 相关目录与 kubeconfig：

```bash
rm -rf /etc/kubernetes /var/lib/kubelet /etc/cni/net.d /var/lib/cni
rm -rf /root/.kube "$HOME/.kube"
```

清理 iptables 规则：

```bash
iptables -F
iptables -t nat -F
iptables -t mangle -F
iptables -X
```

使用 IPVS 模式时清理 IPVS 与虚拟接口：

```bash
ipvsadm --clear
ip link del kube-ipvs0 2>/dev/null || true
```

清理 CNI 残留接口（先确认，再删除）：

```bash
ip link show | grep -E 'cni0|flannel|calico|cilium|vxlan'
# 确认属于本集群 CNI 后逐个删除，例如：
ip link delete cni0
```

说明：

- Worker 节点无 etcd 数据，**不要**删除 `/var/lib/etcd`；etcd member 清理属于控制面流程。
- 刷 iptables 会影响宿主机上其他服务的规则，**仅在专用于 K8s 且确认下线的节点上执行**。
- 接口名随 CNI 不同而不同，只删除确认属于本集群的接口。
- 若节点整机重装/彻底下线，可选 `systemctl disable kubelet`，并按需卸载 kubelet/容器运行时、清理 `/var/lib/containerd`（后者会删除本机所有镜像与容器，非必要勿做）。

### 6. 重启节点

节点侧清理完成后，重启主机，确认 kubelet、容器运行时和网络规则不会在启动时恢复：

```bash
reboot
```

说明：

- 若已执行 `systemctl disable kubelet`，重启后 kubelet 不会自启；若未 disable，重启后 kubelet 可能以失败状态启动，需确认节点未重新注册回集群。
- 重启后等待 SSH 恢复，再执行第 7 节最终验证。

### 7. 最终验证

完整移除后，控制面：

```bash
kubectl get nodes
kubectl get pods -A --field-selector spec.nodeName=<NODE>
```

预期：节点已不存在，且无该节点上的 Pod。

节点侧：

```bash
systemctl is-active kubelet
ls /etc/kubernetes
ls /etc/cni/net.d
iptables -S | grep KUBE-
ipvsadm -Ln
```

预期：

- kubelet 非 active（已 disable 时为 inactive；`systemctl is-active` 在非 active 时退出码非 0，属正常）；
- `/etc/kubernetes`、`/etc/cni/net.d` 不存在；
- 无 `KUBE-` 链（`grep` 无匹配时退出码为 1，属预期，不是命令失败）；
- IPVS 中无 K8s Service（未使用 IPVS 模式时命令不存在或输出为空，属正常）。

对照检查（只在 Pixiu 节点管理页删除了节点、尚未做主机侧清理时）：

```bash
kubectl get nodes                     # 控制面：看不到 <NODE>
systemctl is-active kubelet           # 节点侧：仍为 active
crictl ps                             # containerd 运行时：仍能看到容器
nerdctl ps                            # nerdctl 运行时：仍能看到容器
docker ps                             # docker 运行时：仍能看到容器
```

预期：`kubectl get nodes` 中已无该节点，但 kubelet 仍为 active、容器仍在运行，说明只完成了控制面删除；请继续执行第 4、5 节的主机侧清理。若 kubelet 凭据有效，节点还可能重新注册回集群。

## 备注 / 风险

- drain 会驱逐业务 Pod，务必确认剩余节点容量与 PDB，避免业务中断。
- `--delete-emptydir-data` 会丢数据；本地 PV 数据不会自动迁移，需提前备份。
- `--force` 仅用于不受控制器管理的 Pod，慎用；被删除的裸 Pod 不会自动重建。
- 控制面节点不能按本文操作，etcd 成员/仲裁风险高，必须走控制面专用流程。
- 不要在仍有其他业务的宿主机上刷 iptables；`iptables -F` 会清空全部 filter 规则。
- kubeadm reset 不会停止 kubelet，也不会卸载 kubelet、容器运行时等软件包；彻底下线需另行卸载（如 kubelet、containerd/docker，如不再使用 kubeadm 也可一并卸载）。
- 同名节点重新加入前，必须确认旧 Node 对象已删除、旧证书/CNI 残留已清理；否则会出现证书、端口、CNI 冲突。
- 节点不可达时先在控制面 `kubectl delete node`，主机恢复后补清理或重装。
- 若节点使用 nftables 模式的 kube-proxy，`iptables` 命令可能无法覆盖全部规则，视发行版/运行时确认后再清理。
- 回滚说明：cordon 后若决定中止，可 `kubectl uncordon <NODE>` 恢复调度；一旦 drain 完成或删除 Node 对象，业务 Pod 已被驱逐，需等节点重新加入集群后才能恢复调度；主机侧手工清理后，必须重新 join 才能恢复。
- 先删除节点后 drain 会导致无法再驱逐该节点上的 Pod；如已删除，尽快停 kubelet 并检查业务 Pod 是否已被重建或丢失。
- Pixiu 集群详情 → 节点管理 → 删除节点 ≠ 主机清理。
