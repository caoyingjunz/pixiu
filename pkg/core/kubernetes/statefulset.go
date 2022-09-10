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

package kubernetes

import (
	"context"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type StatefulSetGetter interface {
	StatefulSets(cloud string) StatefulSetInterface
}

type StatefulSetInterface interface {
	Create(ctx context.Context, statefulSet *v1.StatefulSet) error
	Update(ctx context.Context, statefulSet *v1.StatefulSet) error
	Delete(ctx context.Context, deleteOptions types.GetOrDeleteOptions) error
	Get(ctx context.Context, getOptions types.GetOrDeleteOptions) (*v1.StatefulSet, error)
	List(ctx context.Context, listOptions types.ListOptions) ([]v1.StatefulSet, error)
}

type statefulSets struct {
	client *kubernetes.Clientset
	cloud  string
}

func NewStatefulSets(c *kubernetes.Clientset, cloud string) *statefulSets {
	return &statefulSets{
		client: c,
		cloud:  cloud,
	}
}

func (c *statefulSets) Create(ctx context.Context, statefulSet *v1.StatefulSet) error {
	if c.client == nil {
		return clientError
	}
	if _, err := c.client.AppsV1().
		StatefulSets(statefulSet.Namespace).
		Create(ctx, statefulSet, metav1.CreateOptions{}); err != nil {
		log.Logger.Errorf("failed to create %s statefulSet: %v", c.cloud, err)
		return err
	}

	return nil
}

func (c *statefulSets) Update(ctx context.Context, statefulSet *v1.StatefulSet) error {
	if c.client == nil {
		return clientError
	}
	if _, err := c.client.AppsV1().
		StatefulSets(statefulSet.Namespace).
		Update(ctx, statefulSet, metav1.UpdateOptions{}); err != nil {
		log.Logger.Errorf("failed to update %s statefulSet: %v", c.cloud, err)
		return err
	}

	return nil
}

func (c *statefulSets) Delete(ctx context.Context, deleteOptions types.GetOrDeleteOptions) error {
	if c.client == nil {
		return clientError
	}
	if err := c.client.AppsV1().
		StatefulSets(deleteOptions.Namespace).
		Delete(ctx, deleteOptions.ObjectName, metav1.DeleteOptions{}); err != nil {
		log.Logger.Errorf("failed to delete %s statefulSet: %v", deleteOptions.CloudName, err)
		return err
	}

	return nil
}

func (c *statefulSets) Get(ctx context.Context, getOptions types.GetOrDeleteOptions) (*v1.StatefulSet, error) {
	if c.client == nil {
		return nil, clientError
	}
	sts, err := c.client.AppsV1().
		StatefulSets(getOptions.Namespace).
		Get(ctx, getOptions.ObjectName, metav1.GetOptions{})
	if err != nil {
		log.Logger.Errorf("failed to get %s statefulSets: %v", getOptions.CloudName, err)
		return nil, err
	}

	return sts, err
}

func (c *statefulSets) List(ctx context.Context, listOptions types.ListOptions) ([]v1.StatefulSet, error) {
	if c.client == nil {
		return nil, clientError
	}
	sts, err := c.client.AppsV1().
		StatefulSets(listOptions.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list statefulsets: %v", listOptions.Namespace, err)
		return nil, err
	}

	return sts.Items, err
}
