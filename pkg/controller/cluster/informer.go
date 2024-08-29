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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ResourceJob         = "job"
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

func (c *cluster) GetJob(ctx context.Context, cronJobsLister listersbatchv1.JobLister, namespace string, name string) (interface{}, error) {
	job, err := cronJobsLister.Jobs(namespace).Get(name)
	if err != nil {
		klog.Error("failed to get job (%s/%s) from indexer: %v", namespace, name, err)
		return nil, err
	}

	return job, nil
}

func (c *cluster) GetNode(ctx context.Context, nodesLister v1.NodeLister, namespace string, name string) (interface{}, error) {
	node, err := nodesLister.Get(name)
	if err != nil {
		klog.Error("failed to get node (%s/%s) from indexer: %v", namespace, name, err)
		return nil, err
	}

	return node, nil
}

func (c *cluster) ListIndexerResources(ctx context.Context, cluster string, resource string, namespace string, listOption types.ListOptions) (interface{}, error) {
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

	if namespace == "all_namespaces" {
		namespace = ""
	}
	return fn(ctx, cs.Informer, namespace, listOption)
}

func (c *cluster) ListPods(ctx context.Context, podsLister v1.PodLister, namespace string, listOption types.ListOptions) (interface{}, error) {
	pods, err := podsLister.Pods(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	// 构造通用的 objects
	objects := make([]metav1.Object, 0)
	for _, pod := range pods {
		objects = append(objects, pod)
	}

	return c.listObjects(objects, namespace, listOption)
}

// ListDeployments 从缓存中获取 deployment 列表
func (c *cluster) ListDeployments(ctx context.Context, deploymentsLister listersv1.DeploymentLister, namespace string, listOption types.ListOptions) (interface{}, error) {
	deployments, err := deploymentsLister.Deployments(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	objects := make([]metav1.Object, 0)
	for _, deployment := range deployments {
		objects = append(objects, deployment)
	}

	return c.listObjects(objects, namespace, listOption)
}

func (c *cluster) ListStatefulSets(ctx context.Context, statefulSetsLister listersv1.StatefulSetLister, namespace string, listOption types.ListOptions) (interface{}, error) {
	statefulSets, err := statefulSetsLister.StatefulSets(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	objects := make([]metav1.Object, 0)
	for _, statefulSet := range statefulSets {
		objects = append(objects, statefulSet)
	}

	return c.listObjects(objects, namespace, listOption)
}

func (c *cluster) ListDaemonSets(ctx context.Context, daemonSetsLister listersv1.DaemonSetLister, namespace string, listOption types.ListOptions) (interface{}, error) {
	daemonSets, err := daemonSetsLister.DaemonSets(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	objects := make([]metav1.Object, 0)
	for _, daemonSet := range daemonSets {
		objects = append(objects, daemonSet)
	}

	return c.listObjects(objects, namespace, listOption)
}

func (c *cluster) ListCronJobs(ctx context.Context, cronJobsLister listersbatchv1.CronJobLister, namespace string, listOption types.ListOptions) (interface{}, error) {
	cronJobs, err := cronJobsLister.CronJobs(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	objects := make([]metav1.Object, 0)
	for _, cronJob := range cronJobs {
		objects = append(objects, cronJob)
	}

	return c.listObjects(objects, namespace, listOption)
}

func (c *cluster) ListJobs(ctx context.Context, jobsLister listersbatchv1.JobLister, namespace string, listOption types.ListOptions) (interface{}, error) {
	jobs, err := jobsLister.Jobs(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	objects := make([]metav1.Object, 0)
	for _, job := range jobs {
		objects = append(objects, job)
	}
	return c.listObjects(objects, namespace, listOption)
}

func (c *cluster) ListNodes(ctx context.Context, nodesLister v1.NodeLister, namespace string, listOption types.ListOptions) (interface{}, error) {
	nodes, err := nodesLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	objects := make([]metav1.Object, 0)
	for _, node := range nodes {
		objects = append(objects, node)
	}
	return c.listObjects(objects, namespace, listOption)
}
