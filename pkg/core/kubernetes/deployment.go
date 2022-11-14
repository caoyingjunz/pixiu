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

	pixiuerrors "github.com/caoyingjunz/gopixiu/pkg/errors"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type DeploymentsGetter interface {
	Deployments(cloud string) DeploymentInterface
}

type DeploymentInterface interface {
	Create(ctx context.Context, deployment *v1.Deployment) error
	Update(ctx context.Context, deployment *v1.Deployment) error
	Delete(ctx context.Context, namespaces string, objectName string) error
	List(ctx context.Context, namespaces string) ([]v1.Deployment, error)
}

type deployments struct {
	client *kubernetes.Clientset
	cloud  string
}

func NewDeployments(c *kubernetes.Clientset, cloud string) *deployments {
	return &deployments{
		client: c,
		cloud:  cloud,
	}
}

func (c *deployments) Create(ctx context.Context, deployment *v1.Deployment) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	if _, err := c.client.AppsV1().Deployments(deployment.Namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		log.Logger.Errorf("failed to create %s namespace %s: %v", c.cloud, deployment.Namespace, err)
		return err
	}

	return nil
}

func (c *deployments) Update(ctx context.Context, deployment *v1.Deployment) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	if _, err := c.client.AppsV1().
		Deployments(deployment.Namespace).
		Update(ctx, deployment, metav1.UpdateOptions{}); err != nil {
		log.Logger.Errorf("failed to update %s deployment: %v", c.cloud, err)
		return err
	}

	return nil
}

func (c *deployments) Delete(ctx context.Context, namespace string, objectName string) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	if err := c.client.AppsV1().Deployments(namespace).Delete(ctx, objectName, metav1.DeleteOptions{}); err != nil {
		log.Logger.Errorf("failed to delete %s deployment: %v", c.cloud, objectName, err)
		return err
	}

	return nil
}

func (c *deployments) List(ctx context.Context, namespace string) ([]v1.Deployment, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
	}
	d, err := c.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list %s deployments: %v", c.cloud, namespace, err)
		return nil, err
	}

	return d.Items, nil
}
