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

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/tunnel"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

const defaultTunnelSyncInterval = "@every 30s"

// TunnelSyncer 定期探测集群 Agent 反向隧道连通性，并根据连通性状态更新集群状态。
type TunnelSyncer struct {
	factory db.ShareDaoFactory
}

func NewTunnelSyncer(f db.ShareDaoFactory) *TunnelSyncer {
	return &TunnelSyncer{factory: f}
}

func (ts *TunnelSyncer) Name() string {
	return "tunnel-syncer"
}

func (ts *TunnelSyncer) CronSpec() string {
	return defaultTunnelSyncInterval
}

func (ts *TunnelSyncer) LogLevel() AccessLogLevel {
	return AccessLogDebug
}

func (ts *TunnelSyncer) Do(ctx *JobContext) error {
	tm := tunnel.Default()
	if tm == nil {
		return nil
	}

	// 仅同步隧道集群且非授权集群（授权集群状态由主集群维护）
	clusters, err := ts.factory.Cluster().List(ctx,
		db.WithConnectMode(int(model.ConnectModeTunnel)),
		db.WithPermissionID(0),
	)
	if err != nil {
		klog.Errorf("[TunnelSyncer] list tunnel clusters failed: %v", err)
		return err
	}

	for i := range clusters {
		obj := &clusters[i]
		if err = checkTunnelCluster(ctx, tm, ts.factory, obj); err != nil {
			klog.Errorf("[TunnelSyncer] cluster %s: %v", obj.Name, err)
		}
	}
	return nil
}

func checkTunnelCluster(ctx context.Context, tm *tunnel.Manager, factory db.ShareDaoFactory, obj *model.Cluster) error {
	// 授权（共享/子）集群的状态由主集群维护，不单独检测隧道连通性
	if obj.PermissionId != 0 {
		return nil
	}

	// 跳过仍在部署/未启动/已失败的集群；Pending（等待接入）需继续探测隧道连通性
	switch obj.ClusterStatus {
	case model.ClusterStatusDeploy, model.ClusterStatusUnStart, model.ClusterStatusFailed:
		tm.SetAgentConnected(obj.Name, false)
		return nil
	}

	// 多 server 共享 db 场景：agent 隧道可能连到其他 server 实例（如生产）。
	// 本进程从未接入该集群（无 session 且未授权过）时不修改共享 db 状态，
	// 避免本进程把集群误判为失联/等待接入。
	if !tm.HasSession(obj.Name) && !tm.SeenSession(obj.Name) {
		tm.SetAgentConnected(obj.Name, false)
		return nil
	}

	apiURL := ""
	if obj.KubeConfig != "" {
		if cfgBytes, err := client.ParseKubeConfigBytes(obj.KubeConfig); err == nil {
			if restCfg, err := clientcmd.RESTConfigFromKubeConfig(cfgBytes); err == nil {
				apiURL = restCfg.Host
			}
		}
	}

	connected := tm.Probe(ctx, obj.Name, apiURL)
	prev := tm.AgentConnected(obj.Name)
	tm.SetAgentConnected(obj.Name, connected)

	desired := obj.ClusterStatus
	switch {
	case connected && (obj.ClusterStatus == model.ClusterStatusPending || obj.ClusterStatus == model.ClusterStatusError):
		desired = model.ClusterStatusRunning
	case !connected && obj.ClusterStatus == model.ClusterStatusRunning:
		desired = model.ClusterStatusError
	}

	if desired == obj.ClusterStatus {
		if prev != connected {
			klog.V(2).Infof("[TunnelSyncer] cluster %s agent_connected=%v", obj.Name, connected)
		}
		return nil
	}

	if err := factory.Cluster().InternalUpdate(ctx, obj.Id, map[string]interface{}{"status": desired}); err != nil {
		if utilerrors.IsNotUpdated(err) {
			return nil
		}
		return err
	}

	klog.V(2).Infof("[TunnelSyncer] cluster %s status %d -> %d (agent_connected=%v)", obj.Name, obj.ClusterStatus, desired, connected)
	return nil
}
