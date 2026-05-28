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

	add := func(methods []string, rel, desc string) {
		for _, m := range methods {
			entries = append(entries, apiregistry.RouteEntry{
				Method: m, RelativePath: rel, Description: desc,
			})
		}
	}
	crud := func(rel, label string) {
		add([]string{"GET"}, rel, label+"-列表")
		add([]string{"POST"}, rel, label+"-创建")
		item := rel + "/:name"
		add([]string{"GET"}, item, label+"-详情")
		add([]string{"PUT", "PATCH"}, item, label+"-更新")
		add([]string{"DELETE"}, item, label+"-删除")
	}
	crudNamespace := func(rel, label string) {
		add([]string{"GET"}, rel, label+"-列表")
		add([]string{"POST"}, rel, label+"-创建")
		item := rel + "/:namespace"
		add([]string{"GET"}, item, label+"-详情")
		add([]string{"PUT", "PATCH"}, item, label+"-更新")
		add([]string{"DELETE"}, item, label+"-删除")
	}
	crudRead := func(rel, label string) {
		add([]string{"GET"}, rel, label+"-列表")
		add([]string{"GET"}, rel+"/:name", label+"-详情")
	}

	// --- 概览 / 基本信息 ---
	crudRead(c+"/api/v1/nodes", "节点")
	add([]string{"GET"}, c+"/api/v1/namespaces/kube-system/configmaps/kubeadm-config", "概览-kubeadm 网络配置")
	add([]string{"GET"}, c+"/api/v1/namespaces/kube-system/configmaps/kube-proxy", "概览-kube-proxy 配置")
	add([]string{"GET"}, c+"/apis/apps/v1/namespaces/kube-system/deployments", "概览-kube-system 工作负载")

	// --- 命名空间 ---
	crudNamespace(c+"/api/v1/namespaces", "命名空间")
	crud(c+"/api/v1/namespaces/:namespace/resourcequotas", "命名空间配额")

	// --- 节点 / Pod ---
	crud(c+"/api/v1/namespaces/:namespace/pods", "Pod（命名空间）")
	add([]string{"GET"}, c+"/api/v1/pods", "Pod（集群）-列表")
	add([]string{"GET"}, c+"/api/v1/namespaces/:namespace/pods/:name/log", "Pod 日志")

	// --- 工作负载 apps/v1 ---
	crud(c+"/apis/apps/v1/deployments", "Deployment（集群）")
	crud(c+"/apis/apps/v1/namespaces/:namespace/deployments", "Deployment（命名空间）")
	crud(c+"/apis/apps/v1/statefulsets", "StatefulSet（集群）")
	crud(c+"/apis/apps/v1/namespaces/:namespace/statefulsets", "StatefulSet（命名空间）")
	crud(c+"/apis/apps/v1/daemonsets", "DaemonSet（集群）")
	crud(c+"/apis/apps/v1/namespaces/:namespace/daemonsets", "DaemonSet（命名空间）")
	crudRead(c+"/apis/apps/v1/namespaces/:namespace/replicasets", "ReplicaSet")

	// --- 批处理 batch/v1 ---
	crud(c+"/apis/batch/v1/jobs", "Job（集群）")
	crud(c+"/apis/batch/v1/namespaces/:namespace/jobs", "Job（命名空间）")
	crud(c+"/apis/batch/v1/cronjobs", "CronJob（集群）")
	crud(c+"/apis/batch/v1/namespaces/:namespace/cronjobs", "CronJob（命名空间）")

	// --- 服务与路由 ---
	crud(c+"/api/v1/services", "Service（集群）")
	crud(c+"/api/v1/namespaces/:namespace/services", "Service（命名空间）")
	crud(c+"/apis/networking.k8s.io/v1/ingresses", "Ingress（集群）")
	crud(c+"/apis/networking.k8s.io/v1/namespaces/:namespace/ingresses", "Ingress（命名空间）")

	// --- 配置 ---
	crud(c+"/api/v1/configmaps", "ConfigMap（集群）")
	crud(c+"/api/v1/namespaces/:namespace/configmaps", "ConfigMap（命名空间）")
	crud(c+"/api/v1/secrets", "Secret（集群）")
	crud(c+"/api/v1/namespaces/:namespace/secrets", "Secret（命名空间）")

	// --- 存储 ---
	crud(c+"/api/v1/persistentvolumes", "持久卷 PV")
	crud(c+"/api/v1/persistentvolumeclaims", "PVC（集群）")
	crud(c+"/api/v1/namespaces/:namespace/persistentvolumeclaims", "PVC（命名空间）")
	crud(c+"/apis/storage.k8s.io/v1/storageclasses", "StorageClass")

	// --- 弹性伸缩 ---
	crud(c+"/apis/autoscaling/v2/horizontalpodautoscalers", "HPA（集群）")
	crud(c+"/apis/autoscaling/v2/namespaces/:namespace/horizontalpodautoscalers", "HPA（命名空间）")

	// --- 认证授权 RBAC ---
	crud(c+"/apis/rbac.authorization.k8s.io/v1/clusterroles", "ClusterRole")
	crud(c+"/apis/rbac.authorization.k8s.io/v1/clusterrolebindings", "ClusterRoleBinding")
	crud(c+"/apis/rbac.authorization.k8s.io/v1/namespaces/:namespace/roles", "Role")
	crud(c+"/apis/rbac.authorization.k8s.io/v1/namespaces/:namespace/rolebindings", "RoleBinding")
	crud(c+"/api/v1/namespaces/:namespace/serviceaccounts", "ServiceAccount")

	// --- 扩展资源 ---
	crudRead(c+"/apis/apiextensions.k8s.io/v1/customresourcedefinitions", "CRD")
	crudRead(c+"/apis/apiregistration.k8s.io/v1/apiservices", "APIService")
	add([]string{"GET"}, c+"/apis/apiregistration.k8s.io/v1/apiservices/:name", "APIService-详情")

	// --- 事件 ---
	crud(c+"/api/v1/events", "Event（集群）")
	crud(c+"/api/v1/namespaces/:namespace/events", "Event（命名空间）")

	// --- Pixiu 指标（dashboard metrics）---
	add([]string{"GET"},
		c+"/apis/metrics.pixiu.io/v1beta1/api/v1/dashboard/nodes/:name/metrics/:metricsName/:subPath",
		"节点指标")
	add([]string{"GET"},
		c+"/apis/metrics.pixiu.io/v1beta1/api/v1/dashboard/namespaces/:namespace/pod-list/:name/metrics/:metricsName/:subPath",
		"Pod 指标")

	// --- YAML 创建 / 编辑：发现 API 资源 ---
	add([]string{"GET"}, c+"/api/:apiVersion", "发现 Core API 资源列表")
	add([]string{"GET"}, c+"/apis/:groupVersion", "发现 Group API 资源列表")

	return entries
}

func proxyAPIRegistryGroup() *apiregistry.Group {
	return &apiregistry.Group{
		Name:    "Kubernetes 资源",
		BaseURL: proxyBaseURL,
		Entries: buildProxyAPIEntries(),
	}
}
