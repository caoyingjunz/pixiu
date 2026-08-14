/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dashboard

import (
	"fmt"
	"strconv"
	"strings"
)

type panelSpec struct {
	PanelDefinition
	rangeQuery bool
	query      func(Filters) string
}

func Catalog() Definition {
	return Definition{
		Sections: []Section{
			{ID: "overview", Title: "监控概览", Icon: "ri:dashboard-line", Children: []string{"cluster", "namespace"}},
			{ID: "core", Title: "核心组件监控", Icon: "ri:cpu-line", Children: []string{"kubelet", "control-plane"}},
			{ID: "node", Title: "节点监控", Icon: "ri:server-line", Children: []string{"node-resource", "node-pod"}},
			{ID: "application", Title: "应用监控", Icon: "ri:apps-line", Children: []string{"workload", "pod"}},
			{ID: "network", Title: "网络监控", Icon: "ri:global-line"},
			{ID: "storage", Title: "存储监控", Icon: "ri:hard-drive-2-line"},
			{ID: "gpu", Title: "GPU 监控", Icon: "ri:dashboard-3-line"},
		},
		Panels: definitions(panelCatalog()),
	}
}

func definitions(specs []panelSpec) []PanelDefinition {
	result := make([]PanelDefinition, 0, len(specs))
	for _, spec := range specs {
		result = append(result, spec.PanelDefinition)
	}
	return result
}

func panel(section, id, title, kind, unit string, span int, metrics []string, rangeQuery bool, builder func(Filters) string) panelSpec {
	return panelSpec{PanelDefinition: PanelDefinition{ID: id, Section: section, Title: title, Kind: kind, Unit: unit, Span: span, RequiredMetrics: metrics}, rangeQuery: rangeQuery, query: builder}
}

func describe(spec panelSpec, description string) panelSpec {
	spec.Description = description
	return spec
}

func unavailablePanel(section, id, title, kind, unit string, span int, metrics ...string) panelSpec {
	return panel(section, id, title, kind, unit, span, metrics, false, nil)
}

func panelCatalog() []panelSpec {
	return []panelSpec{
		panel("cluster", "cluster.nodes", "节点数", "stat", "short", 3, []string{"kube_node_info"}, false, fixed("count(kube_node_info)")),
		panel("cluster", "cluster.ready_nodes", "Ready 节点", "stat", "short", 3, []string{"kube_node_status_condition"}, false, fixed(`count(kube_node_status_condition{condition="Ready",status="true"} == 1)`)),
		panel("cluster", "cluster.running_pods", "运行中 Pod", "stat", "short", 3, []string{"kube_pod_status_phase"}, false, fixed(`sum(kube_pod_status_phase{phase="Running"})`)),
		panel("cluster", "cluster.namespaces", "Namespace", "stat", "short", 3, []string{"kube_namespace_created"}, false, fixed("count(kube_namespace_created)")),
		panel("cluster", "cluster.cpu_usage", "CPU 使用率", "gauge", "percent", 4, []string{"container_cpu_usage_seconds_total", "kube_node_status_allocatable"}, false, clusterCPUUsage),
		panel("cluster", "cluster.cpu_requests", "CPU Request 承诺率", "gauge", "percent", 4, []string{"kube_pod_container_resource_requests", "kube_node_status_allocatable"}, false, clusterCPURequests),
		panel("cluster", "cluster.memory_usage", "内存使用率", "gauge", "percent", 4, []string{"container_memory_working_set_bytes", "kube_node_status_allocatable"}, false, clusterMemoryUsage),
		panel("namespace", "namespace.pods", "Namespace Pod 数", "bar", "short", 6, []string{"kube_pod_info"}, false, namespacePods),
		panel("namespace", "namespace.cpu", "Namespace CPU Top 10", "bar", "cores", 6, []string{"container_cpu_usage_seconds_total"}, false, namespaceCPU),
		panel("namespace", "namespace.memory", "Namespace 内存 Top 10", "bar", "bytes", 6, []string{"container_memory_working_set_bytes"}, false, namespaceMemory),
		panel("namespace", "namespace.restarts", "容器重启 Top 10", "bar", "short", 6, []string{"kube_pod_container_status_restarts_total"}, false, namespaceRestarts),

		panel("kubelet", "kubelet.running_pods", "运行中 Pod", "stat", "short", 6, []string{"kubelet_running_pods"}, false, fixed("sum(kubelet_running_pods)")),
		panel("kubelet", "kubelet.running_containers", "运行中容器", "stat", "short", 6, []string{"kubelet_running_containers"}, false, fixed("sum(kubelet_running_containers)")),
		panel("kubelet", "kubelet.operation_rate", "Runtime 操作速率", "line", "ops", 6, []string{"kubelet_runtime_operations_total"}, true, fixed("sum by (operation_type) (rate(kubelet_runtime_operations_total[5m]))")),
		describe(
			panel("kubelet", "kubelet.error_rate", "Runtime 错误速率", "line", "ops", 6, []string{"kubelet_runtime_operations_errors_total"}, true, fixed("sum by (operation_type) (rate(kubelet_runtime_operations_errors_total[5m]))")),
			"Kubelet 调用容器运行时发生错误的每秒速率，按操作类型统计。",
		),
		unavailablePanel("control-plane", "control.scheduler", "Scheduler 调度状态", "empty", "", 4, "scheduler_schedule_attempts_total"),
		unavailablePanel("control-plane", "control.controller", "Controller Manager", "empty", "", 4),
		unavailablePanel("control-plane", "control.apiserver", "API Server 请求", "empty", "", 4, "apiserver_request_total"),

		panel("node-resource", "node.ready", "节点健康状态", "status", "", 6, []string{"kube_node_status_condition"}, false, nodeReady),
		panel("node-resource", "node.pods", "节点 Pod 数", "bar", "short", 6, []string{"kube_pod_info"}, false, nodePods),
		panel("node-resource", "node.cpu", "节点 CPU 使用率", "bar", "percent", 6, []string{"container_cpu_usage_seconds_total", "kube_node_status_allocatable"}, false, nodeCPU),
		panel("node-resource", "node.memory", "节点内存使用率", "bar", "percent", 6, []string{"container_memory_working_set_bytes", "kube_node_status_allocatable"}, false, nodeMemory),
		panel("node-pod", "node.pod_cpu", "节点 Pod CPU Top 10", "bar", "cores", 6, []string{"container_cpu_usage_seconds_total"}, false, podCPU),
		panel("node-pod", "node.pod_memory", "节点 Pod 内存 Top 10", "bar", "bytes", 6, []string{"container_memory_working_set_bytes"}, false, podMemory),

		panel("workload", "workload.deployments", "Deployment 可用率", "bar", "percent", 4, []string{"kube_deployment_status_replicas_available", "kube_deployment_spec_replicas"}, false, deploymentAvailability),
		panel("workload", "workload.statefulsets", "StatefulSet Ready", "bar", "percent", 4, []string{"kube_statefulset_status_replicas_ready", "kube_statefulset_replicas"}, false, statefulsetAvailability),
		panel("workload", "workload.daemonsets", "DaemonSet Ready", "bar", "percent", 4, []string{"kube_daemonset_status_number_ready", "kube_daemonset_status_desired_number_scheduled"}, false, daemonsetAvailability),
		describe(
			panel("pod", "pod.cpu_trend", "Pod CPU 使用率", "line", "percent", 6, []string{"container_cpu_usage_seconds_total", "kube_pod_container_resource_limits"}, true, podCPUTrend),
			"当前 CPU 用量占 Pod 容器 CPU limits 的百分比；未配置 CPU limits 的 Pod 不显示。",
		),
		describe(
			panel("pod", "pod.memory_trend", "Pod 内存使用率", "line", "percent", 6, []string{"container_memory_working_set_bytes", "kube_pod_container_resource_limits"}, true, podMemoryTrend),
			"当前内存工作集占 Pod 容器内存 limits 的百分比；未配置内存 limits 的 Pod 不显示。",
		),
		panel("pod", "pod.restarts", "Pod 重启次数", "bar", "short", 6, []string{"kube_pod_container_status_restarts_total"}, false, podRestarts),
		panel("pod", "pod.phase", "Pod 状态", "status", "", 6, []string{"kube_pod_status_phase"}, false, podPhase),

		panel("network", "network.receive", "Pod 网络流入 Top 10", "bar", "Bps", 6, []string{"container_network_receive_bytes_total"}, false, networkReceive),
		panel("network", "network.transmit", "Pod 网络流出 Top 10", "bar", "Bps", 6, []string{"container_network_transmit_bytes_total"}, false, networkTransmit),
		panel("network", "network.receive_trend", "网络流入趋势", "line", "Bps", 6, []string{"container_network_receive_bytes_total"}, true, networkReceiveTrend),
		panel("network", "network.transmit_trend", "网络流出趋势", "line", "Bps", 6, []string{"container_network_transmit_bytes_total"}, true, networkTransmitTrend),

		panel("storage", "storage.pvc_phase", "PVC 状态", "status", "", 6, []string{"kube_persistentvolumeclaim_status_phase"}, false, pvcPhase),
		panel("storage", "storage.container_fs", "容器文件系统使用 Top 10", "bar", "bytes", 6, []string{"container_fs_usage_bytes"}, false, containerFS),

		unavailablePanel("gpu", "gpu.utilization", "GPU 使用率", "empty", "percent", 4, "DCGM_FI_DEV_GPU_UTIL"),
		unavailablePanel("gpu", "gpu.memory", "GPU 显存使用", "empty", "bytes", 4, "DCGM_FI_DEV_FB_USED"),
		unavailablePanel("gpu", "gpu.temperature", "GPU 温度", "empty", "celsius", 4, "DCGM_FI_DEV_GPU_TEMP"),
	}
}

func fixed(expression string) func(Filters) string { return func(Filters) string { return expression } }

func matcher(filters Filters, supported ...string) string {
	wanted := map[string]bool{}
	for _, label := range supported {
		wanted[label] = true
	}
	values := map[string]string{"namespace": filters.Namespace, "node": filters.Node, "pod": filters.Pod}
	parts := []string{}
	for _, label := range []string{"namespace", "node", "pod"} {
		if wanted[label] && strings.TrimSpace(values[label]) != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", label, strconv.Quote(strings.TrimSpace(values[label]))))
		}
	}
	return strings.Join(parts, ",")
}

func selector(metric string, filters Filters, supported []string, extra ...string) string {
	parts := []string{}
	if filtersPart := matcher(filters, supported...); filtersPart != "" {
		parts = append(parts, filtersPart)
	}
	parts = append(parts, extra...)
	if len(parts) == 0 {
		return metric
	}
	return metric + "{" + strings.Join(parts, ",") + "}"
}

func workloadJoin(expression string, filters Filters) string {
	if strings.TrimSpace(filters.WorkloadName) == "" {
		return expression
	}
	if strings.EqualFold(strings.TrimSpace(filters.WorkloadKind), "Deployment") {
		replicaSetOwners := fmt.Sprintf(
			`max by(namespace,replicaset) (kube_replicaset_owner{owner_kind="Deployment",owner_name=%s})`,
			strconv.Quote(strings.TrimSpace(filters.WorkloadName)),
		)
		podReplicaSets := `max by(namespace,pod,replicaset) (label_replace(kube_pod_owner{owner_kind="ReplicaSet"}, "replicaset", "$1", "owner_name", "(.*)"))`
		return fmt.Sprintf("(%s) * on(namespace,pod) group_left(replicaset) (%s) * on(namespace,replicaset) group_left() (%s)", expression, podReplicaSets, replicaSetOwners)
	}
	owner := []string{fmt.Sprintf("owner_name=%s", strconv.Quote(strings.TrimSpace(filters.WorkloadName)))}
	if strings.TrimSpace(filters.WorkloadKind) != "" {
		owner = append(owner, fmt.Sprintf("owner_kind=%s", strconv.Quote(strings.TrimSpace(filters.WorkloadKind))))
	}
	return fmt.Sprintf("(%s) * on(namespace,pod) group_left(owner_kind,owner_name) max by(namespace,pod,owner_kind,owner_name) (kube_pod_owner{%s})", expression, strings.Join(owner, ","))
}

func containerExpr(metric string, filters Filters, rate bool) string {
	expr := selector(metric, filters, []string{"namespace", "node", "pod"}, `container!=""`, `image!=""`)
	if rate {
		expr = "rate(" + expr + "[5m])"
	}
	return workloadJoin(expr, filters)
}

func clusterCPUUsage(f Filters) string {
	return fmt.Sprintf("100 * sum(%s) / sum(%s)", containerExpr("container_cpu_usage_seconds_total", f, true), selector("kube_node_status_allocatable", f, []string{"node"}, `resource="cpu"`))
}
func clusterCPURequests(f Filters) string {
	return fmt.Sprintf("100 * sum(%s) / sum(%s)", selector("kube_pod_container_resource_requests", f, []string{"namespace", "node", "pod"}, `resource="cpu"`), selector("kube_node_status_allocatable", f, []string{"node"}, `resource="cpu"`))
}
func clusterMemoryUsage(f Filters) string {
	return fmt.Sprintf("100 * sum(%s) / sum(%s)", containerExpr("container_memory_working_set_bytes", f, false), selector("kube_node_status_allocatable", f, []string{"node"}, `resource="memory"`))
}
func namespacePods(f Filters) string {
	return "sort_desc(sum by (namespace) (" + selector("kube_pod_info", f, []string{"namespace", "node", "pod"}) + "))"
}
func namespaceCPU(f Filters) string {
	return "topk(10, sum by (namespace) (" + containerExpr("container_cpu_usage_seconds_total", f, true) + "))"
}
func namespaceMemory(f Filters) string {
	return "topk(10, sum by (namespace) (" + containerExpr("container_memory_working_set_bytes", f, false) + "))"
}
func namespaceRestarts(f Filters) string {
	return "topk(10, sum by (namespace) (" + selector("kube_pod_container_status_restarts_total", f, []string{"namespace", "pod"}) + "))"
}
func nodeReady(f Filters) string {
	return selector("kube_node_status_condition", f, []string{"node"}, `condition="Ready"`, `status="true"`)
}
func nodePods(f Filters) string {
	return "sort_desc(sum by (node) (" + selector("kube_pod_info", f, []string{"namespace", "node", "pod"}) + "))"
}
func nodeCPU(f Filters) string {
	return fmt.Sprintf("100 * sum by (node) (%s) / sum by (node) (%s)", containerExpr("container_cpu_usage_seconds_total", f, true), selector("kube_node_status_allocatable", f, []string{"node"}, `resource="cpu"`))
}
func nodeMemory(f Filters) string {
	return fmt.Sprintf("100 * sum by (node) (%s) / sum by (node) (%s)", containerExpr("container_memory_working_set_bytes", f, false), selector("kube_node_status_allocatable", f, []string{"node"}, `resource="memory"`))
}
func podCPU(f Filters) string {
	return "topk(10, sum by (namespace,pod) (" + containerExpr("container_cpu_usage_seconds_total", f, true) + "))"
}
func podMemory(f Filters) string {
	return "topk(10, sum by (namespace,pod) (" + containerExpr("container_memory_working_set_bytes", f, false) + "))"
}
func podCPUTrend(f Filters) string {
	usage := "sum by (namespace,pod) (" + containerExpr("container_cpu_usage_seconds_total", f, true) + ")"
	limits := "sum by (namespace,pod) (" + podResourceLimits(f, "cpu", "core") + ")"
	return fmt.Sprintf("100 * %s / clamp_min(%s, 0.001)", usage, limits)
}
func podMemoryTrend(f Filters) string {
	usage := "sum by (namespace,pod) (" + containerExpr("container_memory_working_set_bytes", f, false) + ")"
	limits := "sum by (namespace,pod) (" + podResourceLimits(f, "memory", "byte") + ")"
	return fmt.Sprintf("100 * %s / clamp_min(%s, 1)", usage, limits)
}

func podResourceLimits(f Filters, resource, unit string) string {
	expression := selector(
		"kube_pod_container_resource_limits",
		f,
		[]string{"namespace", "node", "pod"},
		fmt.Sprintf("resource=%s", strconv.Quote(resource)),
		fmt.Sprintf("unit=%s", strconv.Quote(unit)),
	)
	return workloadJoin(expression, f)
}
func podRestarts(f Filters) string {
	return "topk(10, sum by (namespace,pod) (" + selector("kube_pod_container_status_restarts_total", f, []string{"namespace", "pod"}) + "))"
}
func podPhase(f Filters) string {
	return selector("kube_pod_status_phase", f, []string{"namespace", "pod"}) + " == 1"
}
func deploymentAvailability(f Filters) string {
	m := matcher(f, "namespace")
	return fmt.Sprintf("100 * kube_deployment_status_replicas_available{%s} / clamp_min(kube_deployment_spec_replicas{%s}, 1)", m, m)
}
func statefulsetAvailability(f Filters) string {
	m := matcher(f, "namespace")
	return fmt.Sprintf("100 * kube_statefulset_status_replicas_ready{%s} / clamp_min(kube_statefulset_replicas{%s}, 1)", m, m)
}
func daemonsetAvailability(f Filters) string {
	m := matcher(f, "namespace")
	return fmt.Sprintf("100 * kube_daemonset_status_number_ready{%s} / clamp_min(kube_daemonset_status_desired_number_scheduled{%s}, 1)", m, m)
}
func networkReceive(f Filters) string {
	return "topk(10, sum by (namespace,pod) (" + containerExpr("container_network_receive_bytes_total", f, true) + "))"
}
func networkTransmit(f Filters) string {
	return "topk(10, sum by (namespace,pod) (" + containerExpr("container_network_transmit_bytes_total", f, true) + "))"
}
func networkReceiveTrend(f Filters) string {
	return "sum by (namespace,pod) (" + containerExpr("container_network_receive_bytes_total", f, true) + ")"
}
func networkTransmitTrend(f Filters) string {
	return "sum by (namespace,pod) (" + containerExpr("container_network_transmit_bytes_total", f, true) + ")"
}
func pvcPhase(f Filters) string {
	return selector("kube_persistentvolumeclaim_status_phase", f, []string{"namespace"}) + " == 1"
}
func containerFS(f Filters) string {
	return "topk(10, sum by (namespace,pod) (" + containerExpr("container_fs_usage_bytes", f, false) + "))"
}
