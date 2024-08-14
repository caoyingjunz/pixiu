/*
Copyright 2024 The Pixiu Authors.

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

package jobmanager

import (
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	NMDefaultSchedule = "@every 5s"
)

type NodeMetrics struct {
	cfg NodeMetricsOptions
	dao db.ShareDaoFactory
}

type NodeMetricsOptions struct {
	Schedule string `yaml:"schedule"`
}

type nodeMetricsInfo struct {
	kubernetesVersion string
	clusterName       string
	clusterStatus     model.ClusterStatus
	dao               db.ShareDaoFactory
	ctx               *JobContext
	c                 *model.Cluster
}

func NMDefaultOptions() NodeMetricsOptions {
	return NodeMetricsOptions{
		Schedule: NMDefaultSchedule,
	}
}

func NewNodeMetrics(cfg NodeMetricsOptions, dao db.ShareDaoFactory) *NodeMetrics {
	return &NodeMetrics{
		cfg: cfg,
		dao: dao,
	}
}

func (nm *NodeMetrics) Name() string {
	return "NodeMetrics"
}

func (nm *NodeMetrics) CronSpec() string {
	return nm.cfg.Schedule
}

func (nm *NodeMetrics) Do(ctx *JobContext) (err error) {
	cluster, err := nm.dao.Cluster().List(ctx)
	if err != nil {
		return err
	}

	var wg errgroup.Group
	for _, c := range cluster {
		// 创建一个局部变量并赋值以确保每个 goroutine 有自己的值副本
		clusterName := c.Name
		nmInfo := &nodeMetricsInfo{
			c:           &c,
			clusterName: clusterName,
			dao:         nm.dao,
			ctx:         ctx,
		}
		wg.Go(nmInfo.doAsync)
	}

	return wg.Wait()
}

func (nmi *nodeMetricsInfo) doAsync() error {
	object, err := nmi.dao.Cluster().GetClusterByName(nmi.ctx, nmi.clusterName)
	if err != nil {
		return err
	}
	if object == nil {
		return err
	}

	// TODO：临时构造 client，后续通过 informer 的方式维护缓存
	updates := make(map[string]interface{})
	status := model.ClusterStatusInterrupt
	kubeNode := &types.KubeNode{
		Ready:    make([]string, 0),
		NotReady: make([]string, 0),
	}

	newClusterSet, err := client.NewClusterSet(object.KubeConfig)
	if err != nil {
		updates["status"] = status
		return nmi.dao.Cluster().Update(nmi.ctx, nmi.c.Id, nmi.c.ResourceVersion, updates, false)
	}

	nodeList, err := newClusterSet.Client.CoreV1().Nodes().List(nmi.ctx, metav1.ListOptions{})
	if err == nil {
		nodes := nodeList.Items
		// 获取 kubernetes 版本
		if len(nodes) != 0 {
			updates["kubernetes_version"] = nodes[0].Status.NodeInfo.KubeletVersion
		}

		// 获取存储状态
		for _, node := range nodes {
			nodeStatus := parseKubeNodeStatus(node)
			switch nodeStatus {
			case "Ready":
				kubeNode.Ready = append(kubeNode.Ready, node.Name)
				status = model.ClusterStatusRunning
			case "NotReady":
				kubeNode.NotReady = append(kubeNode.NotReady, node.Name)
			}
			// 如果没有有一个节点是 Ready，则集群状态为 AllNotNodeHealthy
			if status != model.ClusterStatusRunning {
				status = model.ClusterStatusAllNotNodeHealthy
			}
		}
		data, err := kubeNode.Marshal()
		if err == nil {
			updates["nodes"] = data
		}
	}

	updates["status"] = status
	return nmi.dao.Cluster().Update(nmi.ctx, nmi.c.Id, nmi.c.ResourceVersion, updates, false)
}

func parseKubeNodeStatus(node v1.Node) string {
	status := "Ready"
	for _, condition := range node.Status.Conditions {
		if condition.Type != v1.NodeReady {
			continue
		}
		if condition.Status != "True" {
			status = "NotReady"
		}
	}

	return status
}
