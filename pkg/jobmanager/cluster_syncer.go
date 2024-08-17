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
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	//"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	DefaultSyncInterval = "@every 5s"
)

type ClusterSyncer struct {
	factory db.ShareDaoFactory
}

var indexer client.Cache

func init() {
	indexer = *client.NewClusterCache()
}

func NewClusterSyncer(f db.ShareDaoFactory) *ClusterSyncer {
	return &ClusterSyncer{
		factory: f,
	}
}

func (cs *ClusterSyncer) Name() string {
	return "ClusterSyncer"
}

func (cs *ClusterSyncer) CronSpec() string {
	return DefaultSyncInterval
}

func (cs *ClusterSyncer) Do(ctx *JobContext) (err error) {
	clusters, err := cs.factory.Cluster().List(ctx)
	if err != nil {
		klog.Error("[ClusterSyncer] failed to get clusters: %v", err)
		return err
	}

	diff := len(clusters)
	errCh := make(chan error, diff)
	var wg sync.WaitGroup
	wg.Add(diff)
	for _, cluster := range clusters {
		go func(c model.Cluster) {
			defer wg.Done()
			if err = doSync(cs.factory, c); err != nil {
				errCh <- err
			}
		}(cluster)
	}
	wg.Wait()

	select {
	case err = <-errCh:
		if err != nil {
			klog.Error("failed to sync cluster status: %v", err)
		}
	default:
	}

	// 清理过期 clusterSet
	return nil
}

func doSync(f db.ShareDaoFactory, cluster model.Cluster) error {
	var (
		status            string
		kubernetesVersion string
		nodeData          string
		err               error
	)

	nodeData, kubernetesVersion, err = parseStatus(cluster)
	if err != nil {

	} else {

	}

	fmt.Println("status", status)
	fmt.Println("nodes", nodeData)
	fmt.Println("kubernetesVersion", kubernetesVersion)

	return err
}

func parseStatus(cluster model.Cluster) (string, string, error) {
	name := cluster.Name

	var (
		cs client.ClusterSet
		ok bool
	)
	cs, ok = indexer.Get(name)
	if !ok {
		clusterSet, err := client.NewClusterSet(cluster.KubeConfig)
		if err != nil {
			return "", "", err
		}
		cs = *clusterSet
		indexer.Set(name, cs)
	}

	nodes, err := cs.Informer.NodesLister().List(labels.Everything())
	if err != nil {
		return "", "", err
	}
	kubeNode := &types.KubeNode{Ready: make([]string, 0), NotReady: make([]string, 0)}
	// 获取存储状态
	for _, node := range nodes {
		nodeStatus := parseKubeNodeStatus(node)
		switch nodeStatus {
		case "Ready":
			kubeNode.Ready = append(kubeNode.Ready, node.Name)
		case "NotReady":
			kubeNode.NotReady = append(kubeNode.NotReady, node.Name)
		}
	}

	nodeData, err := kubeNode.Marshal()
	if err != nil {
		return "", "", err
	}

	var kubernetesVersion string
	if len(nodes) != 0 {
		kubernetesVersion = nodes[0].Status.NodeInfo.KubeletVersion
	}

	return nodeData, kubernetesVersion, nil
}

//func (nmi *nodeMetricsInfo) doAsync() error {
//	object, err := nmi.dao.Cluster().GetClusterByName(nmi.ctx, nmi.clusterName)
//	if err != nil {
//		return err
//	}
//	if object == nil {
//		return err
//	}
//
//	// TODO：临时构造 client，后续通过 informer 的方式维护缓存
//	updates := make(map[string]interface{})
//	status := model.ClusterStatusRunning
//	kubeNode := &types.KubeNode{
//		Ready:    make([]string, 0),
//		NotReady: make([]string, 0),
//	}
//
//	newClusterSet, err := client.NewClusterSet(object.KubeConfig)
//	if err != nil {
//		updates["status"] = status
//		return nmi.dao.Cluster().InternalUpdate(nmi.ctx, nmi.c.Id, updates)
//	}
//
//	nodeList, err := newClusterSet.Client.CoreV1().Nodes().List(nmi.ctx, metav1.ListOptions{})
//	if err == nil {
//		nodes := nodeList.Items
//		// 获取 kubernetes 版本
//		if len(nodes) != 0 {
//			updates["kubernetes_version"] = nodes[0].Status.NodeInfo.KubeletVersion
//		}
//
//		// 获取存储状态
//		for _, node := range nodes {
//			nodeStatus := parseKubeNodeStatus(node)
//			switch nodeStatus {
//			case "Ready":
//				kubeNode.Ready = append(kubeNode.Ready, node.Name)
//				status = model.ClusterStatusRunning
//			case "NotReady":
//				kubeNode.NotReady = append(kubeNode.NotReady, node.Name)
//			}
//		}
//		data, err := kubeNode.Marshal()
//		if err == nil {
//			updates["nodes"] = data
//		}
//	}
//
//	updates["status"] = status
//	return nmi.dao.Cluster().InternalUpdate(nmi.ctx, nmi.c.Id, updates)
//}

func CleanCache() {

}

func parseKubeNodeStatus(node *v1.Node) string {
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
