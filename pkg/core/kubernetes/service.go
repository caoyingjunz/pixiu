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

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type ServicesGetter interface {
	Services(cloud string) ServiceInterface
}

type ServiceInterface interface {
	List(ctx context.Context, listOptions types.ListOptions) ([]v1.Service, error)
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

func (c *services) List(ctx context.Context, listOptions types.ListOptions) ([]v1.Service, error) {
	if c.client == nil {
		return nil, clientError
	}
	svc, err := c.client.CoreV1().
		Services(listOptions.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list %s %s services: %v", listOptions.CloudName, listOptions.Namespace, err)
		return nil, err
	}

	return svc.Items, nil
}
