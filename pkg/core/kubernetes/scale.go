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

	autoscalingapi "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	pixiutypes "github.com/caoyingjunz/gopixiu/api/types"
)

// ScalesGetter can produce a ScaleInterface
type ScalesGetter interface {
	Scales(cloud string) ScaleInterface
}

type ScaleInterface interface {
	Get(ctx context.Context, opts pixiutypes.ScaleOptions) (*autoscalingapi.Scale, error)
	Update(ctx context.Context, opts pixiutypes.ScaleOptions) error
	Patch(ctx context.Context, gvr schema.GroupVersionResource, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions) (*autoscalingapi.Scale, error)
}

type scales struct {
	client *kubernetes.Clientset
	cloud  string
}

func NewScales(c *kubernetes.Clientset, cloud string) *scales {
	return &scales{
		client: c,
		cloud:  cloud,
	}
}

func (c *scales) Get(ctx context.Context, opts pixiutypes.ScaleOptions) (*autoscalingapi.Scale, error) {
	if c.client == nil {
		return nil, clientError
	}

	return nil, nil
}

func (c *scales) Update(ctx context.Context, opts pixiutypes.ScaleOptions) error {
	if c.client == nil {
		return clientError
	}

	return nil
}

func (c *scales) Patch(ctx context.Context, gvr schema.GroupVersionResource, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions) (*autoscalingapi.Scale, error) {
	return nil, nil
}
