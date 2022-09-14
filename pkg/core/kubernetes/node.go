package kubernetes

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/util"
)

type NodesGetter interface {
	Nodes(cloud string) NodeInterface
}

type NodeInterface interface {
	List(ctx context.Context) ([]*types.Nodes, error)
	Get(ctx context.Context, nodeOptions types.NodeOptions) (*v1.Node, error)
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

func (c *nodes) Get(ctx context.Context, nodeOptions types.NodeOptions) (*v1.Node, error) {
	if c.client == nil {
		return nil, clientError
	}
	node, err := c.client.CoreV1().
		Nodes().
		Get(ctx, nodeOptions.ObjectName, metav1.GetOptions{})
	if err != nil {
		log.Logger.Errorf("failed to get node :%v", err)
		return nil, err
	}

	return node, nil
}

func (c *nodes) List(ctx context.Context) ([]*types.Nodes, error) {
	if c.client == nil {
		return nil, clientError
	}
	nodeList, err := c.client.CoreV1().
		Nodes().
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list node :%v", err)
		return nil, err
	}

	ref := make([]*types.Nodes, 0)
	for _, item := range nodeList.Items {
		for _, status := range item.Status.Conditions {
			if status.Type == "Ready" {
				ref = append(ref, &types.Nodes{
					Name:                    item.Name,
					Status:                  util.IsNodeReady(status.Status),
					Roles:                   item.Labels["kubernetes.io/role"],
					Age:                     int(time.Now().Sub(item.ObjectMeta.CreationTimestamp.Time).Hours() / 24),
					KubeletVersion:          item.Status.NodeInfo.KubeletVersion,
					ContainerRuntimeVersion: item.Status.NodeInfo.ContainerRuntimeVersion,
					KernelVersion:           item.Status.NodeInfo.KernelVersion,
					InternalIP:              item.Status.Addresses[0].Address,
					ExternalIP:              item.Spec.PodCIDR,
					OsImage:                 item.Status.NodeInfo.OSImage,
				})
			}
		}

	}
	return ref, nil
}
