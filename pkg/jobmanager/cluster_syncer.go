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
	"context"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	logutil "github.com/caoyingjunz/pixiu/pkg/util/log"
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
	return "cluster-syncer"
}

func (cs *ClusterSyncer) CronSpec() string {
	return DefaultSyncInterval
}

func (cs *ClusterSyncer) LogLevel() logutil.LogLevel {
	return logutil.DebugLevel
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
	cleanLister(clusters)
	return nil
}

func doSync(f db.ShareDaoFactory, cluster model.Cluster) error {
	// 处理自建集群正在部署的集群
	if cluster.ClusterType == model.ClusterTypeCustom {
		// 自建环境，状态是部署未完成时，则直接不做同步，包含：部署中，等待部署，部署失败
		if cluster.ClusterStatus == model.ClusterStatusUnStart ||
			cluster.ClusterStatus == model.ClusterStatusDeploy ||
			cluster.ClusterStatus == model.ClusterStatusFailed {
			return nil
		}
	}

	var (
		kubernetesVersion string
		nodeData          string
		err               error
	)
	status := model.ClusterStatusRunning
	nodeData, kubernetesVersion, err = getNewestKubeStatus(cluster)
	if err != nil {
		status = model.ClusterStatusError
	}

	updates := make(map[string]interface{})
	parseStatus(updates, status, kubernetesVersion, nodeData, cluster)
	if len(updates) == 0 {
		return nil
	}

	if err = f.Cluster().InternalUpdate(context.TODO(), cluster.Id, updates); err != nil {
		klog.Error("failed to update cluster(%s) status: %v", cluster.Name, err)
	}
	return nil
}

func parseStatus(update map[string]interface{}, status model.ClusterStatus, kubernetesVersion string, nodeData string, cluster model.Cluster) {
	if status != cluster.ClusterStatus {
		update["status"] = status
	}
	if kubernetesVersion != cluster.KubernetesVersion {
		update["kubernetes_version"] = kubernetesVersion
	}
	if nodeData != cluster.Nodes {
		update["nodes"] = nodeData
	}
}

func getNewestKubeStatus(cluster model.Cluster) (string, string, error) {
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

func cleanLister(clusters []model.Cluster) {
	cs := make(map[string]bool)
	for _, cluster := range clusters {
		cs[cluster.Name] = true
	}

	for name := range indexer.List() {
		if cs[name] {
			continue
		}
		klog.Infof("lister %s will be delete from indexer", name)
		indexer.Delete(name)
	}
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
