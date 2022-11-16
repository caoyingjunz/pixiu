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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/gopixiu/api/meta"
	pixiuerrors "github.com/caoyingjunz/gopixiu/pkg/errors"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type EventsGetter interface {
	Events(cloud string) EventInterface
}

type EventInterface interface {
	List(ctx context.Context, listOptions meta.ListOptions) ([]corev1.Event, error)
}

type events struct {
	client *kubernetes.Clientset
	cloud  string
}

func NewEvents(c *kubernetes.Clientset, cloud string) *events {
	return &events{
		client: c,
		cloud:  cloud,
	}
}

func (c *events) List(ctx context.Context, listOptions meta.ListOptions) ([]corev1.Event, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
	}
	event, err := c.client.CoreV1().
		Events(listOptions.Namespace).
		List(ctx, metav1.ListOptions{
			// todo
		})
	if err != nil {
		log.Logger.Errorf("failed to list %s %s events: %v", listOptions.Cloud, listOptions.Namespace, err)
		return nil, err
	}

	// TODO: 过滤

	return event.Items, nil
}
