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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

func (c *cloud) ListStatefulsets(ctx context.Context, listOptions types.ListOptions) ([]v1.StatefulSet, error) {
	clientSet := clientSets.Get(listOptions.CloudName)
	if clientSet == nil {
		return nil, clientError
	}
	statefulsets, err := clientSet.AppsV1().
		StatefulSets(listOptions.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list statefulsets: %v", listOptions.Namespace, err)
		return nil, err
	}

	return statefulsets.Items, err
}

func (c *cloud) GetStatefulset(ctx context.Context, getOptions types.GetOrDeleteOptions) (*v1.StatefulSet, error) {
	clientSet := clientSets.Get(getOptions.CloudName)
	if clientSet == nil {
		return nil, clientError
	}
	statefulset, err := clientSet.AppsV1().
		StatefulSets(getOptions.Namespace).
		Get(ctx, getOptions.ObjectName, metav1.GetOptions{})
	if err != nil {
		log.Logger.Errorf("failed to get statefulsets: %v", getOptions.CloudName, err)
		return nil, err
	}

	return statefulset, err
}

func (c *cloud) DeleteStatefulset(ctx context.Context, deleteOptions types.GetOrDeleteOptions) error {
	clientSet := clientSets.Get(deleteOptions.CloudName)
	if clientSet == nil {
		return clientError
	}
	err := clientSet.AppsV1().
		StatefulSets(deleteOptions.Namespace).
		Delete(ctx, deleteOptions.ObjectName, metav1.DeleteOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list statefulsets: %v", deleteOptions.CloudName, err)
		return err
	}

	return err
}

func (c *cloud) UpdateStatefulset(ctx context.Context, cloudName string, statefulset *v1.StatefulSet) error {
	clientSet := clientSets.Get(cloudName)
	if clientSet == nil {
		return clientError
	}
	_, err := clientSet.AppsV1().
		StatefulSets(statefulset.Namespace).
		Update(ctx, statefulset, metav1.UpdateOptions{})
	if err != nil {
		log.Logger.Errorf("failed to update statefulsets: %v", cloudName, err)
		return err
	}

	return nil
}

func (c *cloud) CreateStatefulset(ctx context.Context, cloudName string, statefulset *v1.StatefulSet) error {
	clientSet := clientSets.Get(cloudName)
	if clientSet == nil {
		return clientError
	}
	_, err := clientSet.AppsV1().
		StatefulSets(statefulset.Namespace).
		Create(ctx, statefulset, metav1.CreateOptions{})
	if err != nil {
		log.Logger.Errorf("failed to create statefulsets: %v", cloudName, err)
		return err
	}

	return nil
}

func (c *cloud) ListServices(ctx context.Context, listOptions types.ListOptions) ([]corev1.Service, error) {
	clientSet := clientSets.Get(listOptions.CloudName)
	if clientSet == nil {
		return nil, clientError
	}
	services, err := clientSet.CoreV1().
		Services(listOptions.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list %s %s services: %v", listOptions.CloudName, listOptions.Namespace, err)
		return nil, err
	}
	return services.Items, nil
}
