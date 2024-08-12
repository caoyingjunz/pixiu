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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	appsv1 "k8s.io/client-go/listers/apps/v1"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	ResourcePod        = "pod"
	ResourceDeployment = "deployment"
)

func (c *cluster) GetIndexerResource(ctx context.Context, cluster string, resource string, namespace string, name string) (interface{}, error) {
	if len(namespace) == 0 || len(name) == 0 {
		return nil, fmt.Errorf("namespace or name is empty")
	}
	cs, err := c.GetClusterSetByName(ctx, cluster)
	if err != nil {
		return nil, err
	}

	// TODO: 后续优化 switch
	switch resource {
	case ResourcePod:
		return c.GetPod(ctx, cs.Informer.PodsLister(), namespace, name)
	case ResourceDeployment:
		return c.GetDeployment(ctx, cs.Informer.DeploymentsLister(), namespace, name)
	}

	return nil, fmt.Errorf("unsupported resource type %s", resource)
}

func (c *cluster) GetPod(ctx context.Context, podsLister v1.PodLister, namespace string, name string) (interface{}, error) {
	pod, err := podsLister.Pods(namespace).Get(name)
	if err != nil {
		klog.Error("failed to get pod (%s/%s) from indexer: %v", namespace, name, err)
		return nil, err
	}

	return pod, nil
}

func (c *cluster) GetDeployment(ctx context.Context, deploymentsLister appsv1.DeploymentLister, namespace string, name string) (interface{}, error) {
	deploy, err := deploymentsLister.Deployments(namespace).Get(name)
	if err != nil {
		klog.Error("failed to get deployment (%s/%s) from indexer: %v", namespace, name, err)
		return nil, err
	}

	return deploy, nil
}

func (c *cluster) ListIndexerResources(ctx context.Context, cluster string, resource string, namespace string, pageOption types.PageRequest) (interface{}, error) {
	// 获取客户端缓存
	cs, err := c.GetClusterSetByName(ctx, cluster)
	if err != nil {
		return nil, err
	}

	switch resource {
	case ResourcePod:
		return c.ListPods(ctx, cs.Informer.PodsLister(), namespace, pageOption)
	case ResourceDeployment:
		return c.ListDeployments(ctx, cs.Informer.DeploymentsLister(), namespace, pageOption)
	}

	return nil, fmt.Errorf("unsupported resource type %s", resource)
}

func (c *cluster) ListPods(ctx context.Context, podsLister v1.PodLister, namespace string, pageOption types.PageRequest) (interface{}, error) {
	// TODO: 验证缓存获取是 namespace 为空是否为全部
	pods, err := podsLister.Pods(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	sort.Slice(pods, func(i, j int) bool {
		// sort by creation timestamp in descending order
		if pods[j].ObjectMeta.GetCreationTimestamp().Time.Before(pods[i].ObjectMeta.GetCreationTimestamp().Time) {
			return true
		} else if pods[i].ObjectMeta.GetCreationTimestamp().Time.Before(pods[j].ObjectMeta.GetCreationTimestamp().Time) {
			return false
		}

		// if the creation timestamps are equal, sort by name in ascending order
		return pods[i].ObjectMeta.GetName() < pods[j].ObjectMeta.GetName()
	})
	if len(namespace) != 0 {
		sort.Slice(pods, func(i, j int) bool { return pods[i].ObjectMeta.GetNamespace() < pods[j].ObjectMeta.GetNamespace() })
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

	total := len(pods)
	offset := (pageOption.Page - 1) * pageOption.Limit
	if offset > total {
		return nil
	}
	end := offset + pageOption.Limit
	if end > total {
		end = total
	}

	return pods[offset:end]
}

func (c *cluster) ListDeployments(ctx context.Context, deploymentsLister appsv1.DeploymentLister, namespace string, pageOption types.PageRequest) (interface{}, error) {
	deployments, err := deploymentsLister.Deployments(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	return deployments, nil
}
