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
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	defaultAgentSyncInterval = "@every 10s"
	// agentHeartbeatTimeout 心跳超时阈值，超过此时间未收到心跳则视为离线
	agentHeartbeatTimeout = 30 * time.Second
)

type AgentSyncer struct {
	factory db.ShareDaoFactory
}

func NewAgentSyncer(f db.ShareDaoFactory) *AgentSyncer {
	return &AgentSyncer{factory: f}
}

func (as *AgentSyncer) Name() string {
	return "agent-syncer"
}

func (as *AgentSyncer) CronSpec() string {
	return defaultAgentSyncInterval
}

func (as *AgentSyncer) LogLevel() AccessLogLevel {
	return AccessLogDebug
}

func (as *AgentSyncer) Do(ctx *JobContext) error {
	// 查询所有在线 agent
	agents, err := as.factory.Agent().List(ctx, db.WithStatus(model.AgentStatusOnline))
	if err != nil {
		klog.Errorf("[AgentSyncer] failed to list online agents: %v", err)
		return err
	}

	now := time.Now()
	var offlineCount int
	for _, agent := range agents {
		if now.Sub(agent.LastHeartbeat.StdTime()) > agentHeartbeatTimeout {
			if err := as.factory.Agent().InternalUpdate(ctx, agent.Id, map[string]interface{}{
				"status": model.AgentStatusOffline,
			}); err != nil {
				klog.Errorf("[AgentSyncer] failed to set agent %s(%d) offline: %v", agent.Name, agent.Id, err)
				continue
			}
			klog.V(2).Infof("[AgentSyncer] agent %s(%d) marked offline, last heartbeat: %s",
				agent.Name, agent.Id, agent.LastHeartbeat.Format(time.RFC3339))
			offlineCount++
		}
	}

	if offlineCount > 0 {
		klog.V(2).Infof("[AgentSyncer] marked %d agent(s) offline", offlineCount)
	}
	return nil
}
