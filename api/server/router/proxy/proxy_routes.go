/*
Copyright 2021 The Pixiu Authors.

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

package proxy

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
)

// buildProxyAPIEntries 与 pixiu-ui 集群详情使用的 /pixiu/proxy 路径对齐，仅写入 API 权限库。
// 实际 HTTP 仍由单一透传路由 proxyHandler 处理。
func buildProxyAPIEntries() []apiregistry.RouteEntry {
	var entries []apiregistry.RouteEntry
	c := "/:clusterName"

	add := func(methods []string, rel, desc, subGroup string) {
		for _, m := range methods {
			entries = append(entries, apiregistry.RouteEntry{
				Method: m, RelativePath: rel, Description: desc, SubGroup: subGroup,
			})
		}
	}
	crud := func(rel, label, subGroup string) {
		add([]string{"GET"}, rel, "查询 "+label, subGroup)
		add([]string{"POST"}, rel, "创建 "+label, subGroup)
		item := rel + "/:name"
		add([]string{"GET"}, item, "查看 "+label, subGroup)
		add([]string{"PUT", "PATCH"}, item, "更新 "+label, subGroup)
		add([]string{"DELETE"}, item, "删除 "+label, subGroup)
	}
	crudNamespace := func(rel, label, subGroup string) {
		add([]string{"GET"}, rel, "查看 "+label+"列表", subGroup)
		add([]string{"POST"}, rel, "创建 "+label, subGroup)
		item := rel + "/:namespace"
		add([]string{"GET"}, item, "查看 "+label, subGroup)
		add([]string{"PUT", "PATCH"}, item, "更新 "+label, subGroup)
		add([]string{"DELETE"}, item, "删除 "+label, subGroup)
	}
	crudRead := func(rel, label, subGroup string) {
		add([]string{"GET"}, rel, "查看 "+label+" 列表", subGroup)
		add([]string{"GET"}, rel+"/:name", "查看 "+label+" 详情", subGroup)
	}

	// --- 概览 / 基本信息 ---
	crudRead(c+"/api/v1/nodes", "节点", "Node")
	add([]string{"GET"}, c+"/api/v1/namespaces/kube-system/configmaps/kubeadm-config", "查看 kubeadm 网络配置", "Kubernetes")
	add([]string{"GET"}, c+"/api/v1/namespaces/kube-system/configmaps/kube-proxy", "查看 kube-proxy 配置", "Kubernetes")
	add([]string{"GET"}, c+"/apis/apps/v1/namespaces/kube-system/deployments", "查看 kube-system 工作负载", "Kubernetes")

	// --- 命名空间 ---
	crudNamespace(c+"/api/v1/namespaces", "命名空间", "Namespace")
	crud(c+"/api/v1/namespaces/:namespace/resourcequotas", "命名空间配额", "Quota")

	// --- 节点 / Pod ---
	crud(c+"/api/v1/namespaces/:namespace/pods", "Pod（命名空间）", "Pod")
	add([]string{"GET"}, c+"/api/v1/pods", "Pod列表（集群）", "Pod")
	add([]string{"GET"}, c+"/api/v1/namespaces/:namespace/pods/:name/log", "Pod 日志", "Pod")

	// --- 工作负载 apps/v1 ---
	crud(c+"/apis/apps/v1/deployments", "Deployment（集群）", "Deployment")
	crud(c+"/apis/apps/v1/namespaces/:namespace/deployments", "Deployment（命名空间）", "Deployment")
	crud(c+"/apis/apps/v1/statefulsets", "StatefulSet（集群）", "StatefulSet")
	crud(c+"/apis/apps/v1/namespaces/:namespace/statefulsets", "StatefulSet（命名空间）", "StatefulSet")
	crud(c+"/apis/apps/v1/daemonsets", "DaemonSet（集群）", "DaemonSet")
	crud(c+"/apis/apps/v1/namespaces/:namespace/daemonsets", "DaemonSet（命名空间）", "DaemonSet")
	crudRead(c+"/apis/apps/v1/namespaces/:namespace/replicasets", "ReplicaSet", "ReplicaSet")

	// --- 批处理 batch/v1 ---
	crud(c+"/apis/batch/v1/jobs", "Job（集群）", "Job")
	crud(c+"/apis/batch/v1/namespaces/:namespace/jobs", "Job（命名空间）", "Job")
	crud(c+"/apis/batch/v1/cronjobs", "CronJob（集群）", "CronJob")
	crud(c+"/apis/batch/v1/namespaces/:namespace/cronjobs", "CronJob（命名空间）", "CronJob")

	// --- 服务与路由 ---
	crud(c+"/api/v1/services", "Service（集群）", "Service")
	crud(c+"/api/v1/namespaces/:namespace/services", "Service（命名空间）", "Service")
	crud(c+"/apis/networking.k8s.io/v1/ingresses", "Ingress（集群）", "Ingress")
	crud(c+"/apis/networking.k8s.io/v1/namespaces/:namespace/ingresses", "Ingress（命名空间）", "Ingress")

	// --- 配置 ---
	crud(c+"/api/v1/configmaps", "ConfigMap（集群）", "ConfigMap")
	crud(c+"/api/v1/namespaces/:namespace/configmaps", "ConfigMap（命名空间）", "ConfigMap")
	crud(c+"/api/v1/secrets", "Secret（集群）", "Secret")
	crud(c+"/api/v1/namespaces/:namespace/secrets", "Secret（命名空间）", "Secret")

	// --- 存储 ---
	crud(c+"/api/v1/persistentvolumes", "PV", "PV")
	crud(c+"/api/v1/persistentvolumeclaims", "PVC（集群）", "PVC")
	crud(c+"/api/v1/namespaces/:namespace/persistentvolumeclaims", "PVC（命名空间）", "PVC")
	crud(c+"/apis/storage.k8s.io/v1/storageclasses", "StorageClass", "StorageClass")

	// --- 弹性伸缩 ---
	crud(c+"/apis/autoscaling/v2/horizontalpodautoscalers", "HPA（集群）", "HPA")
	crud(c+"/apis/autoscaling/v2/namespaces/:namespace/horizontalpodautoscalers", "HPA（命名空间）", "HPA")

	// --- 认证授权 RBAC ---
	crud(c+"/apis/rbac.authorization.k8s.io/v1/clusterroles", "ClusterRole", "ClusterRole")
	crud(c+"/apis/rbac.authorization.k8s.io/v1/clusterrolebindings", "ClusterRoleBinding", "ClusterRoleBinding")
	crud(c+"/apis/rbac.authorization.k8s.io/v1/namespaces/:namespace/roles", "Role", "Role")
	crud(c+"/apis/rbac.authorization.k8s.io/v1/namespaces/:namespace/rolebindings", "RoleBinding", "RoleBinding")
	crud(c+"/api/v1/namespaces/:namespace/serviceaccounts", "ServiceAccount", "ServiceAccount")

	// --- 扩展资源 ---
	crudRead(c+"/apis/apiextensions.k8s.io/v1/customresourcedefinitions", "CRD", "CRD")
	crudRead(c+"/apis/apiregistration.k8s.io/v1/apiservices", "APIService", "APIService")
	add([]string{"GET"}, c+"/apis/apiregistration.k8s.io/v1/apiservices/:name", "查看 APIService 详情", "APIService")

	// --- 事件 ---
	crud(c+"/api/v1/events", "Event（集群）", "Event")
	crud(c+"/api/v1/namespaces/:namespace/events", "Event（命名空间）", "Event")

	// --- Pixiu 指标（dashboard metrics）---
	add([]string{"GET"},
		c+"/apis/metrics.pixiu.io/v1beta1/api/v1/dashboard/nodes/:name/metrics/:metricsName/:subPath",
		"节点指标", "Metrics")
	add([]string{"GET"},
		c+"/apis/metrics.pixiu.io/v1beta1/api/v1/dashboard/namespaces/:namespace/pod-list/:name/metrics/:metricsName/:subPath",
		"Pod 指标", "Metrics")

	// --- YAML 创建 / 编辑：发现 API 资源 ---
	add([]string{"GET"}, c+"/api/:apiVersion", "发现 CoreAPI 列表", "API")
	add([]string{"GET"}, c+"/apis/:groupVersion", "发现 GroupAPI 列表", "API")

	return entries
}

func proxyAPIRegistryGroup() *apiregistry.Group {
	return &apiregistry.Group{
		Name:    "Kubernetes 资源",
		BaseURL: proxyBaseURL,
		Entries: buildProxyAPIEntries(),
	}
}
