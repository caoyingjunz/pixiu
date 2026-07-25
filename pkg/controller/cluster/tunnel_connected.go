/*
Copyright 2026 The Pixiu Authors.

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

package cluster

import (
	"context"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/tunnel"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

const tunnelCheckInterval = 15 * time.Second

// syncTunnelConnectivity periodically probes Agent reverse tunnels (Rancher-style
// control-plane check) and persists Running/Error when connectivity flips.
func (c *cluster) syncTunnelConnectivity(ctx context.Context) {
	tm := tunnel.Default()
	if tm == nil {
		return
	}

	clusters, err := c.factory.Cluster().List(ctx)
	if err != nil {
		klog.Errorf("tunnel check: list clusters failed: %v", err)
		return
	}

	for i := range clusters {
		obj := &clusters[i]
		if obj.ConnectMode != model.ConnectModeTunnel {
			continue
		}
		if err := c.checkTunnelCluster(ctx, tm, obj); err != nil {
			klog.Errorf("tunnel check: cluster %s: %v", obj.Name, err)
		}
	}
}

func (c *cluster) checkTunnelCluster(ctx context.Context, tm *tunnel.Manager, obj *model.Cluster) error {
	// Skip clusters that are still being provisioned / not expected to be online.
	switch obj.ClusterStatus {
	case model.ClusterStatusDeploy, model.ClusterStatusUnStart, model.ClusterStatusFailed:
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
	case connected && obj.ClusterStatus == model.ClusterStatusError:
		desired = model.ClusterStatusRunning
	case !connected && obj.ClusterStatus == model.ClusterStatusRunning:
		desired = model.ClusterStatusError
	}

	if desired == obj.ClusterStatus {
		if prev != connected {
			klog.Infof("tunnel check: cluster %s agent_connected=%v", obj.Name, connected)
		}
		return nil
	}

	if err := c.factory.Cluster().InternalUpdate(ctx, obj.Id, map[string]interface{}{
		"status": desired,
	}); err != nil {
		if utilerrors.IsNotUpdated(err) {
			return nil
		}
		return err
	}
	klog.Infof("tunnel check: cluster %s status %d -> %d (agent_connected=%v)",
		obj.Name, obj.ClusterStatus, desired, connected)
	return nil
}
