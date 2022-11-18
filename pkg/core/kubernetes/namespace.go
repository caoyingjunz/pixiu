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

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	pixiuerrors "github.com/caoyingjunz/gopixiu/pkg/errors"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type NamespacesGetter interface {
	Namespaces(cloud string) NamespaceInterface
}

type NamespaceInterface interface {
	Create(ctx context.Context, namespace v1.Namespace) error
	Update(ctx context.Context, namespace v1.Namespace) (*v1.Namespace, error)
	Delete(ctx context.Context, namespace string) error
	Get(ctx context.Context, namespace string) (*v1.Namespace, error)
	List(ctx context.Context) ([]v1.Namespace, error)
}

type namespaces struct {
	client *kubernetes.Clientset
	cloud  string
}

func NewNamespaces(c *kubernetes.Clientset, cloud string) *namespaces {
	return &namespaces{
		client: c,
		cloud:  cloud,
	}
}

func (c *namespaces) Create(ctx context.Context, namespace v1.Namespace) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	if _, err := c.client.CoreV1().
		Namespaces().
		Create(ctx, &namespace, metav1.CreateOptions{}); err != nil {
		log.Logger.Errorf("failed to create %s namespace %s: %v", c.cloud, namespace.Name, err)
		return err
	}

	return nil
}

func (c *namespaces) Delete(ctx context.Context, namespace string) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	if err := c.client.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{}); err != nil {
		log.Logger.Errorf("failed to delete %s namespace %s: %v", c.cloud, namespace, err)
		return err
	}

	return nil
}

func (c *namespaces) Get(ctx context.Context, namespace string) (*v1.Namespace, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
	}
	ns, err := c.client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		log.Logger.Errorf("failed to get namespaces: %v", err)
		return nil, err
	}

	return ns, nil
}

func (c *namespaces) List(ctx context.Context) ([]v1.Namespace, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
	}

	ns, err := c.client.CoreV1().
		Namespaces().
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list namespaces: %v", err)
		return nil, err
	}

	return ns.Items, nil
}

func (c *namespaces) Update(ctx context.Context, namespace v1.Namespace) (*v1.Namespace, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
	}
	ns, err := c.client.CoreV1().Namespaces().Update(ctx, &namespace, metav1.UpdateOptions{})
	if err != nil {
		log.Logger.Errorf("failed to update namespaces: %v", err)
		return nil, err
	}

	return ns, nil
}
