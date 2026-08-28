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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	DefaultSyncInterval = "@every 30s"
	// clusterProbeTimeout 直连集群拨测超时，避免 apiserver 无响应时 goroutine 长时间挂起
	clusterProbeTimeout = 15 * time.Second
)

type ClusterSyncer struct {
	factory  db.ShareDaoFactory
	disabled bool
	backoff  *probeBackoff
}

func NewClusterSyncer(f db.ShareDaoFactory, disabled bool) *ClusterSyncer {
	return &ClusterSyncer{
		factory:  f,
		disabled: disabled,
		backoff:  newProbeBackoff(),
	}
}

func (cs *ClusterSyncer) Name() string {
	return "cluster-syncer"
}

func (cs *ClusterSyncer) CronSpec() string {
	return DefaultSyncInterval
}

func (cs *ClusterSyncer) LogLevel() AccessLogLevel {
	return AccessLogDebug
}

func (cs *ClusterSyncer) Do(ctx *JobContext) (err error) {
	if cs.disabled {
		klog.V(2).Info("[ClusterSyncer] disabled")
		return nil
	}

	// 仅同步直连集群（隧道集群由 TunnelSyncer 维护）且非授权集群
	clusters, err := cs.factory.Cluster().List(ctx,
		db.WithPermissionID(0),
		db.WithConnectMode(int(model.ConnectModeDirect)),
	)
	if err != nil {
		klog.Errorf("[ClusterSyncer] failed to get clusters: %v", err)
		return err
	}

	diff := len(clusters)
	errCh := make(chan error, diff)
	var wg sync.WaitGroup
	wg.Add(diff)
	exist := make(map[string]struct{}, diff)
	for _, cluster := range clusters {
		exist[cluster.Name] = struct{}{}
		go func(c model.Cluster) {
			defer wg.Done()
			if err = cs.doSync(c); err != nil {
				errCh <- err
			}
		}(cluster)
	}
	wg.Wait()
	cs.backoff.cleanup(exist)

	select {
	case err = <-errCh:
		if err != nil {
			klog.Errorf("failed to sync cluster status: %v", err)
		}
	default:
	}

	return nil
}

func (cs *ClusterSyncer) doSync(cluster model.Cluster) error {
	if cluster.PermissionId != 0 {
		klog.V(2).Infof("authorized cluster %s(%d) needs no checking", cluster.AliasName, cluster.Id)
		return nil
	}

	// 处理自建集群正在部署的集群
	if cluster.ClusterType == model.ClusterTypeCustom {
		// 自建环境，状态是部署未完成时，则直接不做同步，包含：部署中，等待部署，部署失败
		if cluster.ClusterStatus == model.ClusterStatusUnStart ||
			cluster.ClusterStatus == model.ClusterStatusDeploy ||
			cluster.ClusterStatus == model.ClusterStatusFailed ||
			cluster.ClusterStatus == model.ClusterStatusPending {
			return nil
		}
	}

	// 指数退避：距上次探测未到退避间隔则跳过本轮
	now := time.Now()
	if !cs.backoff.shouldProbe(cluster.Name, now) {
		return nil
	}

	var (
		kubernetesVersion string
		nodeData          string
		err               error
	)
	status := model.ClusterStatusRunning
	connected := true
	probeReason := "Healthy"
	probeMessage := ""
	nodeData, kubernetesVersion, err = getNewestKubeStatus(cluster)
	if err != nil {
		klog.Errorf("[getNewestKubeStatus] %s failed: %v, cluster status will be marked as unavailable", cluster.AliasName, err)
		status = model.ClusterStatusError
		connected = false
		probeReason = "ProbeFailed"
		probeMessage = err.Error()
	}
	cs.backoff.markResult(cluster.Name, connected, now)

	updates := make(map[string]interface{})
	parseStatus(updates, status, kubernetesVersion, nodeData, cluster)
	// 每次探测都写入连通性 condition 并刷新探测时间（不受"无变化跳过"影响）
	for k, v := range probeConditionUpdates(connected, probeReason, probeMessage, now) {
		updates[k] = v
	}

	if err = cs.factory.Cluster().InternalUpdate(context.TODO(), cluster.Id, updates); err != nil {
		klog.Errorf("failed to update cluster(%s) status: %v", cluster.Name, err)
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
	clusterSet, err := client.NewClusterSetWithOptions(cluster.KubeConfig, client.ClusterSetOptions{
		ClusterName: cluster.Name,
		ConnectMode: cluster.ConnectMode,
	})
	if err != nil {
		return "", "", err
	}

	probeCtx, cancel := context.WithTimeout(context.Background(), clusterProbeTimeout)
	defer cancel()

	// 只拉 1 个节点：用 RemainingItemCount 推算集群节点总数，并从该节点读取版本
	nodeList, err := clusterSet.Client.CoreV1().Nodes().List(probeCtx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return "", "", err
	}

	if len(nodeList.Items) == 0 {
		nodeData, marshalErr := (&types.KubeNode{}).Marshal()
		if marshalErr != nil {
			return "", "", marshalErr
		}
		return nodeData, "", nil
	}

	total := 1
	if nodeList.RemainingItemCount != nil {
		total = 1 + int(*nodeList.RemainingItemCount)
	}

	nodeData, err := (&types.KubeNode{Total: total}).Marshal()
	if err != nil {
		return "", "", err
	}

	return nodeData, nodeList.Items[0].Status.NodeInfo.KubeletVersion, nil
}
