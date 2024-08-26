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

package cluster

import (
	"context"
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	listersv1 "k8s.io/client-go/listers/apps/v1"
	listersbatchv1 "k8s.io/client-go/listers/batch/v1"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	ResourceNode        = "node"
	ResourcePod         = "pod"
	ResourceDeployment  = "deployment"
	ResourceStatefulSet = "statefulset"
	ResourceDaemonSet   = "daemonset"
	ResourceCronJob     = "cronjob"
)

func (c *cluster) GetIndexerResource(ctx context.Context, cluster string, resource string, namespace string, name string) (interface{}, error) {
	if len(namespace) == 0 || len(name) == 0 {
		return nil, fmt.Errorf("namespace or name is empty")
	}
	cs, err := c.GetClusterSetByName(ctx, cluster)
	if err != nil {
		return nil, err
	}

	// getter functions should be registered in NewCluster function
	fn, ok := c.getterFuncs[resource]
	if !ok {
		return nil, fmt.Errorf("unsupported resource type %s", resource)
	}
	return fn(ctx, cs.Informer, namespace, name)
}

func (c *cluster) GetPod(ctx context.Context, podsLister v1.PodLister, namespace string, name string) (interface{}, error) {
	pod, err := podsLister.Pods(namespace).Get(name)
	if err != nil {
		klog.Error("failed to get pod (%s/%s) from indexer: %v", namespace, name, err)
		return nil, err
	}

	return pod, nil
}

func (c *cluster) GetDeployment(ctx context.Context, deploymentsLister listersv1.DeploymentLister, namespace string, name string) (interface{}, error) {
	deploy, err := deploymentsLister.Deployments(namespace).Get(name)
	if err != nil {
		klog.Error("failed to get deployment (%s/%s) from indexer: %v", namespace, name, err)
		return nil, err
	}

	return deploy, nil
}

func (c *cluster) GetStatefulSet(ctx context.Context, statefulSetsLister listersv1.StatefulSetLister, namespace string, name string) (interface{}, error) {
	statefulSet, err := statefulSetsLister.StatefulSets(namespace).Get(name)
	if err != nil {
		klog.Error("failed to get statefulSet (%s/%s) from indexer: %v", namespace, name, err)
		return nil, err
	}

	return statefulSet, nil
}

func (c *cluster) GetDaemonSet(ctx context.Context, daemonSetsLister listersv1.DaemonSetLister, namespace string, name string) (interface{}, error) {
	daemonSet, err := daemonSetsLister.DaemonSets(namespace).Get(name)
	if err != nil {
		klog.Error("failed to get daemonset (%s/%s) from indexer: %v", namespace, name, err)
		return nil, err
	}

	return daemonSet, nil
}

func (c *cluster) GetCronJob(ctx context.Context, cronJobsLister listersbatchv1.CronJobLister, namespace string, name string) (interface{}, error) {
	cronJob, err := cronJobsLister.CronJobs(namespace).Get(name)
	if err != nil {
		klog.Error("failed to get cronjob (%s/%s) from indexer: %v", namespace, name, err)
		return nil, err
	}

	return cronJob, nil
}

func (c *cluster) GetNode(ctx context.Context, nodesLister v1.NodeLister, namespace string, name string) (interface{}, error) {
	node, err := nodesLister.Get(name)
	if err != nil {
		klog.Error("failed to get node (%s/%s) from indexer: %v", namespace, name, err)
		return nil, err
	}

	return node, nil
}

func (c *cluster) ListIndexerResources(ctx context.Context, cluster string, resource string, namespace string, pageOption types.PageRequest) (interface{}, error) {
	// 获取客户端缓存
	cs, err := c.GetClusterSetByName(ctx, cluster)
	if err != nil {
		return nil, err
	}

	// lister functions should be registered in NewCluster function
	fn, ok := c.listerFuncs[resource]
	if !ok {
		return nil, fmt.Errorf("unsupported resource type %s", resource)
	}
	return fn(ctx, cs.Informer, namespace, pageOption)
}

func (c *cluster) ListPods(ctx context.Context, podsLister v1.PodLister, namespace string, pageOption types.PageRequest) (interface{}, error) {
	// TODO: 验证缓存获取是 namespace 为空是否为全部
	pods, err := podsLister.Pods(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	sort.SliceStable(pods, func(i, j int) bool {
		return pods[i].ObjectMeta.GetName() < pods[j].ObjectMeta.GetName()
	})
	// 全量获取 pod 时，以命名空间排序
	if len(namespace) == 0 {
		sort.SliceStable(pods, func(i, j int) bool {
			return pods[i].ObjectMeta.GetNamespace() < pods[j].ObjectMeta.GetNamespace()
		})
	}

	return types.PageResponse{
		PageRequest: pageOption,
		Total:       len(pods),
		Items:       c.podsForPage(pods, pageOption),
	}, nil
}

func (c *cluster) podsForPage(pods []*corev1.Pod, pageOption types.PageRequest) interface{} {
	if !pageOption.IsPaged() {
		return pods
	}
	offset, end, err := pageOption.Offset(len(pods))
	if err != nil {
		return nil
	}

	return pods[offset:end]
}

// ListDeployments
// TODO: 后续优化
func (c *cluster) ListDeployments(ctx context.Context, deploymentsLister listersv1.DeploymentLister, namespace string, pageOption types.PageRequest) (interface{}, error) {
	deployments, err := deploymentsLister.Deployments(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	sort.SliceStable(deployments, func(i, j int) bool {
		return deployments[i].ObjectMeta.GetName() < deployments[j].ObjectMeta.GetName()
	})
	if len(namespace) == 0 {
		sort.SliceStable(deployments, func(i, j int) bool {
			return deployments[i].ObjectMeta.GetNamespace() < deployments[j].ObjectMeta.GetNamespace()
		})
	}

	return types.PageResponse{
		PageRequest: pageOption,
		Total:       len(deployments),
		Items:       c.deploymentsForPage(deployments, pageOption),
	}, nil
}

func (c *cluster) ListStatefulSets(ctx context.Context, statefulSetsLister listersv1.StatefulSetLister, namespace string, pageOption types.PageRequest) (interface{}, error) {
	statefulSets, err := statefulSetsLister.StatefulSets(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	sort.SliceStable(statefulSets, func(i, j int) bool {
		return statefulSets[i].ObjectMeta.GetName() < statefulSets[j].ObjectMeta.GetName()
	})
	if len(namespace) == 0 {
		sort.SliceStable(statefulSets, func(i, j int) bool {
			return statefulSets[i].ObjectMeta.GetNamespace() < statefulSets[j].ObjectMeta.GetNamespace()
		})
	}

	return types.PageResponse{
		PageRequest: pageOption,
		Total:       len(statefulSets),
		Items:       c.statefulSetsForPage(statefulSets, pageOption),
	}, nil
}

func (c *cluster) ListDaemonSets(ctx context.Context, daemonSetsLister listersv1.DaemonSetLister, namespace string, pageOption types.PageRequest) (interface{}, error) {
	daemonSets, err := daemonSetsLister.DaemonSets(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	sort.SliceStable(daemonSets, func(i, j int) bool {
		return daemonSets[i].ObjectMeta.GetName() < daemonSets[j].ObjectMeta.GetName()
	})
	if len(namespace) == 0 {
		sort.SliceStable(daemonSets, func(i, j int) bool {
			return daemonSets[i].ObjectMeta.GetNamespace() < daemonSets[j].ObjectMeta.GetNamespace()
		})
	}

	return types.PageResponse{
		PageRequest: pageOption,
		Total:       len(daemonSets),
		Items:       c.daemonSetsForPage(daemonSets, pageOption),
	}, nil
}

func (c *cluster) ListCronJobs(ctx context.Context, cronJobsLister listersbatchv1.CronJobLister, namespace string, pageOption types.PageRequest) (interface{}, error) {
	cronJobs, err := cronJobsLister.CronJobs(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	sort.SliceStable(cronJobs, func(i, j int) bool {
		return cronJobs[i].ObjectMeta.GetName() < cronJobs[j].ObjectMeta.GetName()
	})
	if len(namespace) == 0 {
		sort.SliceStable(cronJobs, func(i, j int) bool {
			return cronJobs[i].ObjectMeta.GetNamespace() < cronJobs[j].ObjectMeta.GetNamespace()
		})
	}

	return types.PageResponse{
		PageRequest: pageOption,
		Total:       len(cronJobs),
		Items:       c.cronJobsForPage(cronJobs, pageOption),
	}, nil
}

func (c *cluster) ListNodes(ctx context.Context, nodesLister v1.NodeLister, namespace string, pageOption types.PageRequest) (interface{}, error) {
	nodes, err := nodesLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].ObjectMeta.GetName() < nodes[j].ObjectMeta.GetName()
	})
	if len(namespace) == 0 {
		sort.SliceStable(nodes, func(i, j int) bool {
			return nodes[i].ObjectMeta.GetNamespace() < nodes[j].ObjectMeta.GetNamespace()
		})
	}

	return types.PageResponse{
		PageRequest: pageOption,
		Total:       len(nodes),
		Items:       c.nodesForPage(nodes, pageOption),
	}, nil
}

func (c *cluster) deploymentsForPage(deployments []*appsv1.Deployment, pageOption types.PageRequest) interface{} {
	if !pageOption.IsPaged() {
		return deployments
	}
	offset, end, err := pageOption.Offset(len(deployments))
	if err != nil {
		return nil
	}

	return deployments[offset:end]
}

func (c *cluster) statefulSetsForPage(statefulSets []*appsv1.StatefulSet, pageOption types.PageRequest) interface{} {
	if !pageOption.IsPaged() {
		return statefulSets
	}
	offset, end, err := pageOption.Offset(len(statefulSets))
	if err != nil {
		return nil
	}

	return statefulSets[offset:end]
}

func (c *cluster) daemonSetsForPage(daemonSets []*appsv1.DaemonSet, pageOption types.PageRequest) interface{} {
	if !pageOption.IsPaged() {
		return daemonSets
	}
	offset, end, err := pageOption.Offset(len(daemonSets))
	if err != nil {
		return nil
	}

	return daemonSets[offset:end]
}

func (c *cluster) cronJobsForPage(cronJobs []*batchv1.CronJob, pageOption types.PageRequest) interface{} {
	if !pageOption.IsPaged() {
		return cronJobs
	}
	offset, end, err := pageOption.Offset(len(cronJobs))
	if err != nil {
		return nil
	}

	return cronJobs[offset:end]
}

func (c *cluster) nodesForPage(nodes []*corev1.Node, pageOption types.PageRequest) interface{} {
	if !pageOption.IsPaged() {
		return nodes
	}
	offset, end, err := pageOption.Offset(len(nodes))
	if err != nil {
		return nil
	}

	return nodes[offset:end]
}
