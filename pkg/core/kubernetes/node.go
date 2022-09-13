package kubernetes

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type NodesGetter interface {
	Nodes(cloud string) NodeInterface
}

type NodeInterface interface {
	List(ctx context.Context) ([]v1.Node, error)
}

type nodes struct {
	client *kubernetes.Clientset
	cloud  string
}

func NewNodes(c *kubernetes.Clientset, cloud string) *nodes {
	return &nodes{
		client: c,
		cloud:  cloud,
	}
}

func (c *nodes) List(ctx context.Context) ([]v1.Node, error) {
	if c.client == nil {
		return nil, clientError
	}
	nodeList, err := c.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list node :%v", err)
		return nil, err
	}
	return nodeList.Items, nil
}
