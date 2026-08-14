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
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/tunnel"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

const (
	defaultTunnelSyncInterval = "@every 30s"
	// tunnelL7ProbeTimeout 经隧道探测 kube-apiserver 的 L7 请求超时。
	tunnelL7ProbeTimeout = 10 * time.Second
)

// TunnelSyncer 定期探测集群 Agent 反向隧道连通性，并根据连通性状态更新集群状态。
type TunnelSyncer struct {
	factory db.ShareDaoFactory
	backoff *probeBackoff
}

func NewTunnelSyncer(f db.ShareDaoFactory) *TunnelSyncer {
	return &TunnelSyncer{
		factory: f,
		backoff: newProbeBackoff(),
	}
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

	exist := make(map[string]struct{}, len(clusters))
	for i := range clusters {
		obj := &clusters[i]
		exist[obj.Name] = struct{}{}
		if err = ts.checkTunnelCluster(ctx, tm, obj); err != nil {
			klog.Errorf("[TunnelSyncer] cluster %s: %v", obj.Name, err)
		}
	}
	ts.backoff.cleanup(exist)
	return nil
}

func (ts *TunnelSyncer) checkTunnelCluster(ctx context.Context, tm *tunnel.Manager, obj *model.Cluster) error {
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

	// 指数退避：距上次探测未到退避间隔则跳过本轮
	now := time.Now()
	if !ts.backoff.shouldProbe(obj.Name, now) {
		return nil
	}

	connected, reason, message := probeTunnelL7(ctx, tm, obj)
	ts.backoff.markResult(obj.Name, connected, now)

	prev := tm.AgentConnected(obj.Name)
	tm.SetAgentConnected(obj.Name, connected)

	desired := obj.ClusterStatus
	switch {
	case connected && (obj.ClusterStatus == model.ClusterStatusPending || obj.ClusterStatus == model.ClusterStatusError):
		desired = model.ClusterStatusRunning
	case !connected && obj.ClusterStatus == model.ClusterStatusRunning:
		desired = model.ClusterStatusError
	}

	updates := probeConditionUpdates(connected, reason, message, now)
	if desired != obj.ClusterStatus {
		updates["status"] = desired
	}

	if prev != connected {
		klog.V(2).Infof("[TunnelSyncer] cluster %s agent_connected=%v", obj.Name, connected)
	}
	if err := ts.factory.Cluster().InternalUpdate(ctx, obj.Id, updates); err != nil {
		if utilerrors.IsNotUpdated(err) {
			return nil
		}
		return err
	}

	klog.V(2).Infof("[TunnelSyncer] cluster %s status %d -> %d (agent_connected=%v)", obj.Name, obj.ClusterStatus, desired, connected)
	return nil
}

// probeTunnelL7 经 Agent 隧道发送真实 kube API 请求（GET /version）探测控制面连通性。
// 隧道模式 NewClusterSetWithOptions 会自动注入 ClusterDialContext（remotedialer），
// 因此请求实际经隧道转发到下游 kube-apiserver。
func probeTunnelL7(ctx context.Context, tm *tunnel.Manager, obj *model.Cluster) (connected bool, reason, message string) {
	// session 已消失（但本进程曾见过，见 SeenSession 前置）：直接判定失联，避免空转拨测
	if !tm.HasSession(obj.Name) {
		return false, "AgentDisconnected", "agent 隧道未连接"
	}

	cs, err := client.NewClusterSetWithOptions(obj.KubeConfig, client.ClusterSetOptions{
		ClusterName: obj.Name,
		ConnectMode: obj.ConnectMode,
	})
	if err != nil {
		return false, "KubeConfigInvalid", err.Error()
	}

	probeCtx, cancel := context.WithTimeout(ctx, tunnelL7ProbeTimeout)
	defer cancel()
	// client-go v0.26 的 Discovery().ServerVersion() 内部使用 context.TODO()，
	// 无法透传探测超时，因此改用 RESTClient 以真正受 probeCtx 约束。
	if _, err = cs.Client.Discovery().RESTClient().Get().AbsPath("/version").Do(probeCtx).Raw(); err != nil {
		return false, "ProbeFailed", err.Error()
	}
	return true, "Healthy", ""
}
