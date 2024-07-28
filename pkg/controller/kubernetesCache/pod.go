package kubernetescache

import (
	"context"
	"fmt"

	"github.com/caoyingjunz/pixiu/pkg/client"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type PodGetter interface {
	Pod(clusterName, namespace string) PodInterface
}

type PodInterface interface {
	List(ctx context.Context) ([]*v1.Pod, error)
}

type Pod struct {
	c         *client.ClusterSet
	namespace string
}

func NewPod(c *client.ClusterSet, namespace string) *Pod {
	return &Pod{c: c, namespace: namespace}
}

var _ PodInterface = (*Pod)(nil)

func (p *Pod) List(ctx context.Context) ([]*v1.Pod, error) {
	if p.c == nil || !p.c.SyncCache {
		return nil, fmt.Errorf("未开启缓存，请先执行同步")
	}
	return p.c.SharedInformerFactory.Core().V1().Pods().Lister().List(labels.Everything())
}
