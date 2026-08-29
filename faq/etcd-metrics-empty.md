# FAQ-001 监控页 etcd 指标为空

- **分类**：监控与数据源
- **涉及组件**：kube-prometheus-stack / etcd / kubeadm

## 现象

监控页 etcd 面板全部提示无数据；Prometheus 中 `up{job="kube-etcd"} = 0`。

## 根因

kubeadm 默认 etcd 静态 Pod 参数为 `--listen-metrics-urls=http://127.0.0.1:2381`（仅回环监听），而 kube-prometheus-stack 的 ServiceMonitor 经 Service 按 **Pod IP:2381** 抓取，跨节点必然 Connection refused。

## 验证方式

```bash
# 从 Prometheus Pod 探测 etcd metrics 端口，预期 Connection refused
kubectl -n monitoring exec deploy/prometheus-prometheus-kube-prometheus-prometheus -c prometheus-auth -- \
  wget -SqO- --timeout=5 http://<etcd-pod-ip>:2381/metrics
```

## 解决方案

（涉及控制面，需评估窗口）在 master 节点修改静态 Pod manifest `/etc/kubernetes/manifests/etcd.yaml`：

```yaml
--listen-metrics-urls=http://127.0.0.1:2381,http://<master-ip>:2381
```

kubelet 检测到 manifest 变化会自动重建 etcd Pod。

## 备注 / 风险

- etcd 重建期间 **apiserver 短暂不可用**（秒级），请避开业务高峰
- `2381` 仅暴露 metrics 只读数据，集群内网暴露可接受
- 同理排查：`up{job=~"kube-proxy|kube-scheduler|kube-controller-manager"}` 若为 0，大概率同为 kubeadm 默认 `--metrics-bind-address=127.0.0.1` 所致，治理方式类似
