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
	"strings"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/gopixiu/api/types"
	pixiuerrors "github.com/caoyingjunz/gopixiu/pkg/errors"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

const (
	labelNodeRole = "node-role.kubernetes.io"
)

type NodesGetter interface {
	Nodes(cloud string) NodeInterface
}

type NodeInterface interface {
	Get(ctx context.Context, nodeOptions types.NodeOptions) (*v1.Node, error)
	List(ctx context.Context) ([]types.Node, error)
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
		return nil, pixiuerrors.ErrCloudNotRegister
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

func (c *nodes) List(ctx context.Context) ([]types.Node, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
	}
	nodeList, err := c.client.CoreV1().
		Nodes().
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list node :%v", err)
		return nil, err
	}

	var nodeSlice []types.Node
	for _, item := range nodeList.Items {
		nodeSlice = append(nodeSlice, c.object2Type(item))
	}

	return nodeSlice, nil
}

// 将 k8s 的对象转换成 type node
func (c *nodes) object2Type(node v1.Node) types.Node {
	var (
		status     = "NotReady"
		internalIP string
		roles      []string
	)
	nodeStatus := node.Status
	nodeInfo := nodeStatus.NodeInfo

	// 获取创建时间
	createAt := node.CreationTimestamp.Format("2006-01-02 15:04:05")
	// 获取 roles
	for label := range node.Labels {
		if strings.HasPrefix(label, labelNodeRole) {
			parts := strings.Split(label, "/")
			// node-role.kubernetes.io/control-plane: ""
			// node-role.kubernetes.io/master: ""
			if len(parts) == 2 {
				roles = append(roles, parts[len(parts)-1])
			}
		}
	}
	// 获取节点状态
	for _, condition := range nodeStatus.Conditions {
		if condition.Type == "Ready" {
			if condition.Status == "True" {
				status = "Ready"
			}
		}
	}
	// 获取节点的 ip 地址
	for _, address := range nodeStatus.Addresses {
		if address.Type == "InternalIP" {
			internalIP = address.Address
		}
	}

	return types.Node{
		Name:             node.Name,
		Status:           status,
		Roles:            strings.Join(roles, ","),
		CreateAt:         createAt,
		Version:          nodeInfo.KubeletVersion,
		InternalIP:       internalIP,
		OsImage:          nodeInfo.OSImage,
		KernelVersion:    nodeInfo.KernelVersion,
		ContainerRuntime: nodeInfo.ContainerRuntimeVersion,
	}
}
