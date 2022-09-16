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

type DeploymentsGetter interface {
	Deployments(cloud string) DeploymentInterface
}

type DeploymentInterface interface {
	Create(ctx context.Context, deployment *v1.Deployment) error
	Delete(ctx context.Context, deleteOptions types.GetOrDeleteOptions) error
	List(ctx context.Context, listOptions types.ListOptions) ([]v1.Deployment, error)
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
		return clientError
	}
	if _, err := c.client.AppsV1().
		Deployments(deployment.Namespace).
		Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		log.Logger.Errorf("failed to create %s namespace %s: %v", c.cloud, deployment.Namespace, err)

		return err
	}

	return nil
}

func (c *deployments) Delete(ctx context.Context, deleteOptions types.GetOrDeleteOptions) error {
	if c.client == nil {
		return clientError
	}
	if err := c.client.AppsV1().
		Deployments(deleteOptions.Namespace).
		Delete(ctx, deleteOptions.ObjectName, metav1.DeleteOptions{}); err != nil {
		log.Logger.Errorf("failed to delete %s deployment: %v", deleteOptions.Namespace, err)
		return err
	}

	return nil
}

func (c *deployments) List(ctx context.Context, listOptions types.ListOptions) ([]v1.Deployment, error) {
	if c.client == nil {
		return nil, clientError
	}
	deploy, err := c.client.AppsV1().
		Deployments(listOptions.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list %s deployments: %v", listOptions.Namespace, err)
		return nil, err
	}

	return deploy.Items, nil
}
