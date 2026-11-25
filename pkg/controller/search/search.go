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

// Package search 实现集群内 K8s 资源的全局搜索（issue #829）。
//
// 设计要点：
//   - 只读：不改动任何集群对象，不落库；
//   - 支持单集群与跨集群：跨集群时逐集群鉴权、逐集群取客户端，单个集群失败不影响其他集群；
//   - 鉴权与代理路径一致：AuthorizeClusterAccessByName 会把「不能使用主集群 admin kubeconfig 的被授权人」
//     回落到其授权子集群的 scoped kubeconfig；
//   - 资源类型白名单：只允许搜索显式登记的类型，避免把资源类型当成探测任意 API 的参数；
//   - 单类（或单集群）失败不拖垮整体：失败的组合记录到 unavailable。
package search

import (
	"context"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	clustercontroller "github.com/caoyingjunz/pixiu/pkg/controller/cluster"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type Getter interface{ Search() Interface }

type Interface interface {
	// Search 在指定集群内按名称关键字/标签搜索 K8s 资源。
	// user 取自会话，用于集群访问鉴权；clusterNames 为集群名（不是 id），与代理路径一致，
	// 支持多个集群，上限 types.MaxSearchClusters。
	Search(ctx context.Context, user *model.User, clusterNames []string, opt types.SearchOptions) (*types.SearchResult, error)
	// Kinds 返回支持搜索的资源类型清单，供前端渲染「资源类型」下拉。
	Kinds() []types.SearchKindInfo
}

type controller struct {
	clusters clustercontroller.Interface
}

func New(clusters clustercontroller.Interface) Interface {
	return &controller{clusters: clusters}
}

func (c *controller) Kinds() []types.SearchKindInfo {
	defaultKinds := make(map[string]struct{}, len(types.DefaultSearchKinds))
	for _, kind := range types.DefaultSearchKinds {
		defaultKinds[strings.ToLower(kind)] = struct{}{}
	}

	kinds := make([]types.SearchKindInfo, 0, len(kindRegistry))
	for key, spec := range kindRegistry {
		_, isDefault := defaultKinds[key]
		kinds = append(kinds, types.SearchKindInfo{
			Resource:      spec.Resource,
			Kind:          spec.Kind,
			APIVersion:    spec.APIVersion,
			ClusterScoped: spec.ClusterScoped,
			Default:       isDefault,
		})
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i].Resource < kinds[j].Resource })
	return kinds
}

func (c *controller) Search(ctx context.Context, user *model.User, clusterNames []string, opt types.SearchOptions) (*types.SearchResult, error) {
	opt, err := normalizeOptions(opt)
	if err != nil {
		return nil, err
	}
	specs, err := resolveKinds(opt.Kinds)
	if err != nil {
		return nil, err
	}
	names, err := normalizeClusters(clusterNames)
	if err != nil {
		return nil, err
	}

	result := &types.SearchResult{
		Clusters: names,
		Items:    make([]types.SearchItem, 0),
	}
	if len(names) == 1 {
		result.Cluster = names[0]
	}

	// 逐集群鉴权并取客户端；单个集群不可用不影响其他集群
	targets := make([]clusterTarget, 0, len(names))
	var firstErr error
	for _, name := range names {
		credCluster, err := c.clusters.AuthorizeClusterAccessByName(ctx, user, name)
		if err != nil {
			result.Unavailable = append(result.Unavailable, types.SearchKindError{Cluster: name, Message: err.Error()})
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		clusterSet, err := c.clusters.GetClusterSetByName(ctx, credCluster.Name)
		if err != nil {
			result.Unavailable = append(result.Unavailable, types.SearchKindError{Cluster: name, Message: err.Error()})
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		targets = append(targets, clusterTarget{name: credCluster.Name, client: clusterSet.Client})
	}

	// 所有集群都不可用时直接返回错误，避免调用方把「无权限」误解为「没有结果」
	if len(targets) == 0 {
		return nil, firstErr
	}

	searchClusterTargets(ctx, targets, specs, opt, result)
	result.Total = len(result.Items)
	return result, nil
}

// normalizeOptions 校验并归一化搜索条件。
func normalizeOptions(opt types.SearchOptions) (types.SearchOptions, error) {
	opt.Keyword = strings.TrimSpace(opt.Keyword)
	opt.LabelSelector = strings.TrimSpace(opt.LabelSelector)
	opt.Namespace = strings.TrimSpace(opt.Namespace)
	if opt.Keyword == "" && opt.LabelSelector == "" {
		return opt, fmt.Errorf("keyword 与 labelSelector 至少提供一个")
	}
	// 与集群资源列表页保持一致的「全部命名空间」写法
	if opt.Namespace == "all_namespaces" {
		opt.Namespace = ""
	}
	if opt.Limit <= 0 {
		opt.Limit = types.DefaultSearchKindLimit
	}
	if opt.Limit > types.MaxSearchKindLimit {
		opt.Limit = types.MaxSearchKindLimit
	}
	return opt, nil
}

// normalizeClusters 集群名去空、去重并做数量限制。
func normalizeClusters(clusterNames []string) ([]string, error) {
	names := make([]string, 0, len(clusterNames))
	seen := make(map[string]struct{}, len(clusterNames))
	for _, name := range clusterNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("cluster 与 clusters 至少提供一个")
	}
	if len(names) > types.MaxSearchClusters {
		return nil, fmt.Errorf("一次最多搜索 %d 个集群，当前 %d 个", types.MaxSearchClusters, len(names))
	}
	return names, nil
}

// objectRef 搜索结果所需的最小对象视图，避免把整个对象带到上层。
type objectRef struct {
	Namespace string
	Name      string
	Labels    map[string]string
	CreatedAt metav1.Time
}

// kindSpec 一类资源的搜索登记项。
type kindSpec struct {
	Resource      string // 复数资源名，前端跳转用
	Kind          string // 展示用 Kind
	APIVersion    string
	ClusterScoped bool // 集群级资源忽略 namespace
	list          func(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error)
}

// kindRegistry 允许被搜索的资源类型白名单。
var kindRegistry = map[string]kindSpec{
	"pods": {
		Resource: "pods", Kind: "Pod", APIVersion: "v1", list: listPods,
	},
	"services": {
		Resource: "services", Kind: "Service", APIVersion: "v1", list: listServices,
	},
	"endpoints": {
		Resource: "endpoints", Kind: "Endpoints", APIVersion: "v1", list: listEndpoints,
	},
	"configmaps": {
		Resource: "configmaps", Kind: "ConfigMap", APIVersion: "v1", list: listConfigMaps,
	},
	"secrets": {
		Resource: "secrets", Kind: "Secret", APIVersion: "v1", list: listSecrets,
	},
	"serviceaccounts": {
		Resource: "serviceaccounts", Kind: "ServiceAccount", APIVersion: "v1", list: listServiceAccounts,
	},
	"persistentvolumeclaims": {
		Resource: "persistentvolumeclaims", Kind: "PersistentVolumeClaim", APIVersion: "v1", list: listPVCs,
	},
	"nodes": {
		Resource: "nodes", Kind: "Node", APIVersion: "v1", ClusterScoped: true, list: listNodes,
	},
	"namespaces": {
		Resource: "namespaces", Kind: "Namespace", APIVersion: "v1", ClusterScoped: true, list: listNamespaces,
	},
	"events": {
		Resource: "events", Kind: "Event", APIVersion: "v1", list: listEvents,
	},
	"deployments": {
		Resource: "deployments", Kind: "Deployment", APIVersion: "apps/v1", list: listDeployments,
	},
	"statefulsets": {
		Resource: "statefulsets", Kind: "StatefulSet", APIVersion: "apps/v1", list: listStatefulSets,
	},
	"daemonsets": {
		Resource: "daemonsets", Kind: "DaemonSet", APIVersion: "apps/v1", list: listDaemonSets,
	},
	"replicasets": {
		Resource: "replicasets", Kind: "ReplicaSet", APIVersion: "apps/v1", list: listReplicaSets,
	},
	"jobs": {
		Resource: "jobs", Kind: "Job", APIVersion: "batch/v1", list: listJobs,
	},
	"cronjobs": {
		Resource: "cronjobs", Kind: "CronJob", APIVersion: "batch/v1", list: listCronJobs,
	},
	"ingresses": {
		Resource: "ingresses", Kind: "Ingress", APIVersion: "networking.k8s.io/v1", list: listIngresses,
	},
}

// SupportedKinds 返回白名单内的资源类型（已排序，便于错误信息与文档展示）。
func SupportedKinds() []string {
	kinds := make([]string, 0, len(kindRegistry))
	for k := range kindRegistry {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

func resolveKinds(kinds []string) ([]kindSpec, error) {
	if len(kinds) == 0 {
		kinds = types.DefaultSearchKinds
	}

	specs := make([]kindSpec, 0, len(kinds))
	seen := make(map[string]struct{}, len(kinds))
	for _, kind := range kinds {
		key := strings.ToLower(strings.TrimSpace(kind))
		if key == "" {
			continue
		}
		spec, ok := kindRegistry[key]
		if !ok {
			return nil, fmt.Errorf("不支持的资源类型 %q，可选值: %s", kind, strings.Join(SupportedKinds(), ", "))
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		specs = append(specs, spec)
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("kinds 不能为空，可选值: %s", strings.Join(SupportedKinds(), ", "))
	}
	return specs, nil
}

// clusterTarget 一个待搜索的集群及其客户端。
type clusterTarget struct {
	name   string
	client kubernetes.Interface
}

// searchClusterTargets 逐集群、逐类型执行搜索并写入 result。
// 与集群控制器解耦（只依赖 kubernetes.Interface），便于用 fake clientset 做单测。
func searchClusterTargets(ctx context.Context, targets []clusterTarget, specs []kindSpec, opt types.SearchOptions, result *types.SearchResult) {
	listLimit := int64(types.SearchListPageSize)

	for _, target := range targets {
		for _, spec := range specs {
			namespace := opt.Namespace
			if spec.ClusterScoped {
				namespace = ""
			}

			objects, err := spec.list(ctx, target.client, namespace, listLimit)
			if err != nil {
				result.Unavailable = append(result.Unavailable, types.SearchKindError{
					Cluster: target.name,
					Kind:    spec.Resource,
					Message: err.Error(),
				})
				continue
			}
			// 拉取量触顶时，后面的对象没有参与匹配，结果可能不完整
			if len(objects) >= types.SearchListPageSize {
				result.Truncated = true
			}

			matches := make([]matchResult, 0, len(objects))
			for _, obj := range objects {
				rank, ok := matchObject(obj, opt)
				if !ok {
					continue
				}
				matches = append(matches, matchResult{rank: rank, object: obj})
			}

			sortMatches(matches)
			if len(matches) > opt.Limit {
				matches = matches[:opt.Limit]
				result.Truncated = true
			}

			for _, m := range matches {
				result.Items = append(result.Items, types.SearchItem{
					Cluster:    target.name,
					Kind:       spec.Kind,
					APIVersion: spec.APIVersion,
					Resource:   spec.Resource,
					Namespace:  m.object.Namespace,
					Name:       m.object.Name,
					Labels:     m.object.Labels,
					CreatedAt:  m.object.CreatedAt.Time,
				})
			}
		}
	}
}

type matchResult struct {
	rank   int
	object objectRef
}

// 匹配优先级：完全匹配 > 前缀匹配 > 包含匹配 > 仅标签匹配
const (
	rankExact = iota
	rankPrefix
	rankContains
	rankLabelOnly
)

// matchObject 判断对象是否命中搜索条件，并返回排序优先级。
// 名称匹配大小写不敏感；标签匹配沿用列表页的「格式化标签包含」语义。
func matchObject(obj objectRef, opt types.SearchOptions) (int, bool) {
	if opt.LabelSelector != "" && !strings.Contains(labels.FormatLabels(obj.Labels), opt.LabelSelector) {
		return 0, false
	}
	if opt.Keyword == "" {
		return rankLabelOnly, true
	}

	name := strings.ToLower(obj.Name)
	keyword := strings.ToLower(opt.Keyword)
	switch {
	case name == keyword:
		return rankExact, true
	case strings.HasPrefix(name, keyword):
		return rankPrefix, true
	case strings.Contains(name, keyword):
		return rankContains, true
	}
	return 0, false
}

func sortMatches(matches []matchResult) {
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].rank != matches[j].rank {
			return matches[i].rank < matches[j].rank
		}
		if matches[i].object.Namespace != matches[j].object.Namespace {
			return matches[i].object.Namespace < matches[j].object.Namespace
		}
		return matches[i].object.Name < matches[j].object.Name
	})
}

func listPods(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		pod := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Labels:    pod.Labels,
			CreatedAt: pod.CreationTimestamp,
		})
	}
	return refs, nil
}

func listServices(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		svc := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: svc.Namespace,
			Name:      svc.Name,
			Labels:    svc.Labels,
			CreatedAt: svc.CreationTimestamp,
		})
	}
	return refs, nil
}

func listEndpoints(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.CoreV1().Endpoints(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		endpoints := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: endpoints.Namespace,
			Name:      endpoints.Name,
			Labels:    endpoints.Labels,
			CreatedAt: endpoints.CreationTimestamp,
		})
	}
	return refs, nil
}

func listConfigMaps(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		cm := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: cm.Namespace,
			Name:      cm.Name,
			Labels:    cm.Labels,
			CreatedAt: cm.CreationTimestamp,
		})
	}
	return refs, nil
}

// 说明：Secret 只返回元数据（名称/命名空间/标签），不返回 data，避免把敏感内容带到搜索接口。
func listSecrets(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		secret := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: secret.Namespace,
			Name:      secret.Name,
			Labels:    secret.Labels,
			CreatedAt: secret.CreationTimestamp,
		})
	}
	return refs, nil
}

func listServiceAccounts(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		sa := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: sa.Namespace,
			Name:      sa.Name,
			Labels:    sa.Labels,
			CreatedAt: sa.CreationTimestamp,
		})
	}
	return refs, nil
}

func listPVCs(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		pvc := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: pvc.Namespace,
			Name:      pvc.Name,
			Labels:    pvc.Labels,
			CreatedAt: pvc.CreationTimestamp,
		})
	}
	return refs, nil
}

func listNodes(ctx context.Context, client kubernetes.Interface, _ string, limit int64) ([]objectRef, error) {
	list, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		node := &list.Items[i]
		refs = append(refs, objectRef{
			Name:      node.Name,
			Labels:    node.Labels,
			CreatedAt: node.CreationTimestamp,
		})
	}
	return refs, nil
}

func listNamespaces(ctx context.Context, client kubernetes.Interface, _ string, limit int64) ([]objectRef, error) {
	list, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		ns := &list.Items[i]
		refs = append(refs, objectRef{
			Name:      ns.Name,
			Labels:    ns.Labels,
			CreatedAt: ns.CreationTimestamp,
		})
	}
	return refs, nil
}

// listEvents 事件是最常用的排障入口。
// 部分事件由 events.k8s.io 侧写入，CreationTimestamp 可能为零值，此时回落到 LastTimestamp。
func listEvents(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		event := &list.Items[i]
		createdAt := event.CreationTimestamp
		if createdAt.IsZero() {
			createdAt = event.LastTimestamp
		}
		refs = append(refs, objectRef{
			Namespace: event.Namespace,
			Name:      event.Name,
			Labels:    event.Labels,
			CreatedAt: createdAt,
		})
	}
	return refs, nil
}

func listDeployments(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		deploy := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: deploy.Namespace,
			Name:      deploy.Name,
			Labels:    deploy.Labels,
			CreatedAt: deploy.CreationTimestamp,
		})
	}
	return refs, nil
}

func listStatefulSets(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		sts := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: sts.Namespace,
			Name:      sts.Name,
			Labels:    sts.Labels,
			CreatedAt: sts.CreationTimestamp,
		})
	}
	return refs, nil
}

func listDaemonSets(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		ds := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: ds.Namespace,
			Name:      ds.Name,
			Labels:    ds.Labels,
			CreatedAt: ds.CreationTimestamp,
		})
	}
	return refs, nil
}

func listReplicaSets(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		rs := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: rs.Namespace,
			Name:      rs.Name,
			Labels:    rs.Labels,
			CreatedAt: rs.CreationTimestamp,
		})
	}
	return refs, nil
}

func listJobs(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		job := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: job.Namespace,
			Name:      job.Name,
			Labels:    job.Labels,
			CreatedAt: job.CreationTimestamp,
		})
	}
	return refs, nil
}

func listCronJobs(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		cronjob := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: cronjob.Namespace,
			Name:      cronjob.Name,
			Labels:    cronjob.Labels,
			CreatedAt: cronjob.CreationTimestamp,
		})
	}
	return refs, nil
}

func listIngresses(ctx context.Context, client kubernetes.Interface, namespace string, limit int64) ([]objectRef, error) {
	list, err := client.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	refs := make([]objectRef, 0, len(list.Items))
	for i := range list.Items {
		ingress := &list.Items[i]
		refs = append(refs, objectRef{
			Namespace: ingress.Namespace,
			Name:      ingress.Name,
			Labels:    ingress.Labels,
			CreatedAt: ingress.CreationTimestamp,
		})
	}
	return refs, nil
}
