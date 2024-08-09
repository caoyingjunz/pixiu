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

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	appsv1 "k8s.io/client-go/listers/apps/v1"
	v1 "k8s.io/client-go/listers/core/v1"
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

	fmt.Println(cs)
	return nil, nil
}

func (c *cluster) ListIndexerResources(ctx context.Context, cluster string, resource string, namespace string) (interface{}, error) {
	// 获取客户端缓存
	cs, err := c.GetClusterSetByName(ctx, cluster)
	if err != nil {
		return nil, err
	}

	switch resource {
	case ResourcePod:
		return c.ListPods(ctx, cs.Informer.PodsLister(), namespace)
	case ResourceDeployment:
		return c.ListDeployments(ctx, cs.Informer.DeploymentsLister(), namespace)
	}

	return nil, fmt.Errorf("unsupported resource type %s", resource)
}

func (c *cluster) ListPods(ctx context.Context, podsLister v1.PodLister, namespace string) (interface{}, error) {
	var (
		pods []*corev1.Pod
		err  error
	)

	// namespace 为空则查询全部 pod
	if len(namespace) == 0 {
		pods, err = podsLister.List(labels.Everything())
	} else {
		pods, err = podsLister.Pods(namespace).List(labels.Everything())
	}
	if err != nil {
		return nil, err
	}

	return pods, nil
}

func (c *cluster) ListDeployments(ctx context.Context, deploymentsLister appsv1.DeploymentLister, namespace string) (interface{}, error) {
	var (
		deployments []*apps.Deployment
		err         error
	)
	if len(namespace) == 0 {
		deployments, err = deploymentsLister.List(labels.Everything())
	} else {
		deployments, err = deploymentsLister.Deployments(namespace).List(labels.Everything())
	}
	if err != nil {
		return nil, err
	}

	return deployments, nil
}
