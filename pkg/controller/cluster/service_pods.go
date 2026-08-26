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

package cluster

import (
	"context"
	"sort"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// ListServicePods 返回 Service 关联的 Pod 列表。
// 优先从 Endpoints TargetRef 解析（与 upstream_auth_proxy 一致）；
// Endpoints 不存在或为空时，回退到 Service.spec.selector 列表。
// includeNotReady=true 时同时纳入 NotReadyAddresses。
func (c *cluster) ListServicePods(ctx context.Context, cluster, namespace, name string, includeNotReady bool) (*v1.PodList, error) {
	clusterSet, err := c.GetClusterSetByName(ctx, cluster)
	if err != nil {
		return nil, err
	}
	client := clusterSet.Client

	svc, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get service (%s/%s) in cluster %s: %v", namespace, name, cluster, err)
		return nil, err
	}

	podNames := collectEndpointPodNames(ctx, client, namespace, name, includeNotReady)
	if len(podNames) == 0 && len(svc.Spec.Selector) > 0 {
		selector := labels.Set(svc.Spec.Selector).AsSelector().String()
		listed, listErr := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector,
			Limit:         500,
		})
		if listErr != nil {
			klog.Errorf("failed to list pods by service selector (%s/%s): %v", namespace, name, listErr)
			return nil, listErr
		}
		return listed, nil
	}

	pods := make([]v1.Pod, 0, len(podNames))
	for _, podName := range podNames {
		pod, getErr := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if getErr != nil {
			if apierrors.IsNotFound(getErr) {
				klog.V(2).Infof("pod %s/%s referenced by service %s endpoints not found, skipping", namespace, podName, name)
				continue
			}
			klog.Errorf("failed to get pod %s/%s for service %s: %v", namespace, podName, name, getErr)
			return nil, getErr
		}
		pods = append(pods, *pod)
	}

	sort.Slice(pods, func(i, j int) bool {
		return pods[i].Name < pods[j].Name
	})

	return &v1.PodList{
		TypeMeta: metav1.TypeMeta{Kind: "PodList", APIVersion: "v1"},
		Items:    pods,
	}, nil
}

func collectEndpointPodNames(ctx context.Context, client kubernetes.Interface, namespace, name string, includeNotReady bool) []string {
	eps, err := client.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Warningf("failed to get endpoints %s/%s: %v", namespace, name, err)
		return nil
	}

	seen := make(map[string]struct{})
	var names []string
	add := func(addrs []v1.EndpointAddress) {
		for _, addr := range addrs {
			if addr.TargetRef == nil || addr.TargetRef.Kind != "Pod" || addr.TargetRef.Name == "" {
				continue
			}
			podName := addr.TargetRef.Name
			if _, ok := seen[podName]; ok {
				continue
			}
			seen[podName] = struct{}{}
			names = append(names, podName)
		}
	}

	for _, subset := range eps.Subsets {
		add(subset.Addresses)
		if includeNotReady {
			add(subset.NotReadyAddresses)
		}
	}
	return names
}
