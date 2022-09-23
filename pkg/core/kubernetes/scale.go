package kubernetes

import (
	"context"
	"k8s.io/client-go/kubernetes"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/scale"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/util"
)

type ScaleGetter interface {
	Scales(cloud string) ScaleInterface
}

type ScaleInterface interface {
	Update(ctx context.Context, updateOption types.UpdateOption, scaleOption types.ScaleOption) error
}

type scales struct {
	cloud string
	//这有问题
	scale *kubernetes.Clientset
	//todo
	scale1 scale.ScaleInterface
}

func NewScale(c *kubernetes.Clientset, cloud string) *scales {
	return &scales{
		scale: c,
		cloud: cloud,
	}
}

func (c *scales) Update(ctx context.Context, updateOption types.UpdateOption, scaleOption types.ScaleOption) error {
	if c.scale == nil {
		return clientError
	}
	groupResource := util.GroupResources(scaleOption)

	_, err := c.scale1(scaleOption.Namespace).Update(ctx, groupResource, &autoscalingv1.Scale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scaleOption.ObjectName,
			Namespace: scaleOption.Namespace,
		},
		Spec: autoscalingv1.ScaleSpec{
			Replicas: updateOption.Replicas,
		},
	}, metav1.UpdateOptions{})
	if err != nil {
		log.Logger.Errorf("failed to update %s namespace %s: %v", c.cloud, scaleOption.Namespace, err)
		return err
	}
	return nil
}
