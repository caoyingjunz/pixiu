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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	"github.com/caoyingjunz/gopixiu/api/meta"
	pixiuerrors "github.com/caoyingjunz/gopixiu/pkg/errors"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type ServicesGetter interface {
	Services(cloud string) ServiceInterface
}

type ServiceInterface interface {
	Create(ctx context.Context, service *v1.Service) error
	Update(ctx context.Context, service *v1.Service) error
	Delete(ctx context.Context, getOptions meta.DeleteOptions) error
	Get(ctx context.Context, getOptions meta.GetOptions) (*v1.Service, error)
	List(ctx context.Context, listOptions meta.ListOptions) ([]v1.Service, error)
}

type services struct {
	client *kubernetes.Clientset
	cloud  string
}

func NewServices(c *kubernetes.Clientset, cloud string) *services {
	return &services{
		client: c,
		cloud:  cloud,
	}
}

func (c *services) List(ctx context.Context, listOptions meta.ListOptions) ([]v1.Service, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
	}
	svc, err := c.client.CoreV1().
		Services(listOptions.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list %s %s services: %v", listOptions.Cloud, listOptions.Namespace, err)
		return nil, err
	}

	return svc.Items, nil
}

func (c *services) Create(ctx context.Context, service *v1.Service) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	if _, err := c.client.CoreV1().
		Services(service.Namespace).
		Create(ctx, service, metav1.CreateOptions{}); err != nil {
		log.Logger.Errorf("failed to create %s service %s: %v", c.cloud, service.Namespace, err)

		return err
	}

	return nil
}

func (c *services) Update(ctx context.Context, service *v1.Service) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	servicesClient := c.client.CoreV1().Services(service.Namespace)
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
		result, getErr := servicesClient.Get(context.TODO(), service.Name, metav1.GetOptions{})
		if getErr != nil {
			log.Logger.Errorf("Failed to get latest version of Service: %v", getErr)
			return getErr
		}
		result.Spec = service.Spec
		_, updateErr := servicesClient.Update(context.TODO(), result, metav1.UpdateOptions{})
		return updateErr
	})
	if retryErr != nil {
		log.Logger.Errorf("Update Service failed: %v", retryErr)
	}

	return nil
}

func (c *services) Delete(ctx context.Context, deleteOptions meta.DeleteOptions) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	err := c.client.CoreV1().
		Services(deleteOptions.Namespace).
		Delete(ctx, deleteOptions.ObjectName, metav1.DeleteOptions{})
	if err != nil {
		log.Logger.Errorf("failed to get %s serice: %v", c.cloud, err)
		return err
	}

	return err
}

func (c *services) Get(ctx context.Context, getOptions meta.GetOptions) (*v1.Service, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
	}
	svc, err := c.client.CoreV1().
		Services(getOptions.Namespace).
		Get(ctx, getOptions.ObjectName, metav1.GetOptions{})
	if err != nil {
		log.Logger.Errorf("failed to get %s serice: %v", c.cloud, err)
		return nil, err
	}

	return svc, err
}
