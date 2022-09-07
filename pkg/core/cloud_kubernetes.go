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

package core

import (
	"context"

	v1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

func (c *cloud) ListDeployments(ctx context.Context, listOptions types.ListOptions) ([]v1.Deployment, error) {
	clientSet := clientSets.Get(listOptions.CloudName)
	if clientSet == nil {
		return nil, clientError
	}
	deployments, err := clientSet.AppsV1().
		Deployments(listOptions.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list %s deployments: %v", listOptions.Namespace, err)
		return nil, err
	}

	return deployments.Items, nil
}

func (c *cloud) DeleteDeployment(ctx context.Context, deleteOptions types.GetOrDeleteOptions) error {
	// 获取 k8s 客户端
	clientSet := clientSets.Get(deleteOptions.CloudName)
	if clientSet == nil {
		return clientError
	}
	if err := clientSet.AppsV1().
		Deployments(deleteOptions.Namespace).
		Delete(ctx, deleteOptions.ObjectName, metav1.DeleteOptions{}); err != nil {
		log.Logger.Errorf("failed to delete %s deployment: %v", deleteOptions.Namespace, err)
		return err
	}

	return nil
}

func (c *cloud) ListJobs(ctx context.Context, listOptions types.ListOptions) ([]batchv1.Job, error) {
	clientSet := clientSets.Get(listOptions.CloudName)
	if clientSet == nil {
		return nil, clientError
	}
	jobs, err := clientSet.BatchV1().
		Jobs(listOptions.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to delete %s deployment: %v", listOptions.Namespace, err)
		return nil, err
	}

	return jobs.Items, nil
}

func (c *cloud) ListNamespaces(ctx context.Context, cloudName string) ([]corev1.Namespace, error) {
	clientSet := clientSets.Get(cloudName)
	if clientSet == nil {
		return nil, clientError
	}
	namespaces, err := clientSet.CoreV1().
		Namespaces().
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list namespaces: %v", cloudName, err)
		return nil, err
	}

	return namespaces.Items, err
}
