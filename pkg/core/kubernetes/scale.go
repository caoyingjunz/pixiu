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
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/scale"

	autoscalingapi "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
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

	scaleClient := scale.New(c.client.RESTClient(),
		restmapper.NewDeferredDiscoveryRESTMapper(cacheddiscovery.NewMemCacheClient(c.client)),
		dynamic.LegacyAPIPathResolverFunc,
		scale.NewDiscoveryScaleKindResolver(c.client.Discovery()))

	gr := schema.GroupResource{
		Group:    "apps",
		Resource: opts.Object,
	}
	scaleSpec, err := scaleClient.Scales(opts.Namespace).Get(context.TODO(), gr, opts.ObjectName, metav1.GetOptions{})
	if err != nil {
		log.Logger.Errorf("failed to get %s replicas: %v", opts.ObjectName, err)
		return nil, err
	}

	return scaleSpec, nil
}

func (c *scales) Update(ctx context.Context, opts pixiutypes.ScaleOptions) error {
	if c.client == nil {
		return clientError
	}
	// 如果副本少于 0， 则判定为0
	if opts.Replicas < 0 {
		opts.Replicas = 0
	}
	// TODO: 处理资源类型的转换

	scaleClient := scale.New(c.client.RESTClient(),
		restmapper.NewDeferredDiscoveryRESTMapper(cacheddiscovery.NewMemCacheClient(c.client)),
		dynamic.LegacyAPIPathResolverFunc,
		scale.NewDiscoveryScaleKindResolver(c.client.Discovery()))

	// 执行 scale
	if _, err := scaleClient.Scales(opts.Namespace).Update(context.TODO(), schema.GroupResource{
		Group:    "apps", // TODO: 后续增加更多的资源类型
		Resource: opts.Object,
	}, &autoscalingapi.Scale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.ObjectName,
			Namespace: opts.Namespace,
		},
		Spec: autoscalingapi.ScaleSpec{
			Replicas: opts.Replicas,
		},
	}, metav1.UpdateOptions{}); err != nil {
		log.Logger.Errorf("failed to scale %s to replicas %d: %v", opts.ObjectName, opts.Replicas, err)
		return err
	}

	return nil
}

func (c *scales) Patch(ctx context.Context, gvr schema.GroupVersionResource, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions) (*autoscalingapi.Scale, error) {
	return nil, nil
}
