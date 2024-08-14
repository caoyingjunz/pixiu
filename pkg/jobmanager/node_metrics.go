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
	"encoding/json"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	NMDefaultSchedule            = "@every 5s"
	LabelNodeRoleControlPlane    = "node-role.kubernetes.io/control-plane"
	LabelNodeRoleOldControlPlane = "node-role.kubernetes.io/master"
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
		var k8sVersion string
		nmInfo := &nodeMetricsInfo{
			c:                 &c,
			clusterName:       clusterName,
			clusterStatus:     model.ClusterStatusRunning,
			kubernetesVersion: k8sVersion,
			dao:               nm.dao,
			ctx:               ctx,
		}
		wg.Go(nmInfo.doAsync)
	}

	return wg.Wait()
}

func (nmi *nodeMetricsInfo) doAsync() error {
	// TODO：临时构造 client，后续通过 informer 的方式维护缓存
	object, err := nmi.dao.Cluster().GetClusterByName(nmi.ctx, nmi.clusterName)
	if err != nil {
		return err
	}
	if object == nil {
		return err
	}
	newClusterSet, err := client.NewClusterSet(object.KubeConfig)
	if err != nil {
		nmi.clusterStatus = model.ClusterStatusInterrupt
	}

	nodeInfo := make(map[string]interface{})
	updates := make(map[string]interface{})

	if nmi.clusterStatus != model.ClusterStatusInterrupt {
		nodeList, err := newClusterSet.Client.CoreV1().Nodes().List(nmi.ctx, metav1.ListOptions{})
		if err != nil {
			nmi.clusterStatus = model.ClusterStatusInterrupt
		}
		if nodeList != nil && len(nodeList.Items) != 0 {
			for _, node := range nodeList.Items {
				status := model.HealthyNodeHealthy
				role := model.NodeRole

				if len(node.Status.Conditions) > 0 {
					lastCondition := node.Status.Conditions[len(node.Status.Conditions)-1]
					if lastCondition.Type == v1.NodeReady && lastCondition.Status != v1.ConditionTrue {
						status = model.UnhealthyNodeHealthy
						nmi.clusterStatus = model.ClusterStatusNotAllNodeHealthy
					}
				}
				if node.Labels[LabelNodeRoleControlPlane] == "" || node.Labels[LabelNodeRoleOldControlPlane] == "" {
					role = model.MasterRole
					nmi.kubernetesVersion = node.Status.NodeInfo.KubeletVersion
				}

				nodeInfo[node.Name] = &types.NodeInfo{
					Status: status,
					Role:   role,
				}
			}
		}

	}

	nodeBytes, err := json.Marshal(nodeInfo)
	if err != nil {
		return err
	}
	if len(nodeBytes) > 0 {
		updates["nodes"] = nodeBytes
	}
	if object.KubernetesVersion == "" {
		updates["kubernetes_version"] = nmi.kubernetesVersion
	}

	updates["cluster_status"] = nmi.clusterStatus

	return nmi.dao.Cluster().Update(nmi.ctx, nmi.c.Id, nmi.c.ResourceVersion, updates, false)
}
