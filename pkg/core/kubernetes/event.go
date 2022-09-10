package kubernetes

import (
	"context"
	"github.com/caoyingjunz/gopixiu/api/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type EventGetter interface {
	Events(cloud string) EventInterface
}

type EventInterface interface {
	ListEventsOfDeploymentByName(ctx context.Context, listOptions types.GetOrDeleteOptions) ([]corev1.Event, error)
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

func (c *events) ListEventsOfDeploymentByName(ctx context.Context, listOptions types.GetOrDeleteOptions) ([]corev1.Event, error) {
	if c.client == nil {
		return nil, clientError
	}
	events, err := c.client.CoreV1().
		Events(listOptions.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list %s %s services: %v", listOptions.CloudName, listOptions.Namespace, err)
		return nil, err
	}

	// 过滤特定deployment产生的事件
	var evts []corev1.Event
	for _, event := range events.Items {
		if event.InvolvedObject.Kind == DeploymentType && event.InvolvedObject.Name == listOptions.ObjectName {
			evts = append(evts, event)
		}
	}

	return evts, nil
}
