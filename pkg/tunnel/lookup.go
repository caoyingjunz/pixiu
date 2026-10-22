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

package tunnel

import (
	"context"

	"github.com/caoyingjunz/pixiu/pkg/agentshell"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// FactoryLookup resolves tunnel tokens via DAO.
type FactoryLookup struct {
	Factory db.ShareDaoFactory
}

// SessionKey resolves a token to a remotedialer session key.
// Cluster agents are namespaced under "cluster:"; deploy agents map to
// agentshell.SessionKey ("agent:") so the two key spaces cannot collide.
func (l FactoryLookup) SessionKey(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", nil
	}

	// 集群代理：使用独立命名空间，避免集群名伪造成 agent:<id> 与部署代理冲突。
	cluster, err := l.Factory.Cluster().GetBy(ctx, db.WithAgentToken(token))
	if err != nil {
		return "", err
	}
	if cluster != nil {
		return ClusterSessionKey(cluster.Name), nil
	}

	agentObj, err := l.Factory.Agent().GetBy(ctx, db.WithToken(token))
	if err != nil {
		return "", err
	}
	return agentSessionKey(agentObj), nil
}

// agentSessionKey maps a deploy agent to its tunnel session key.
// Other agent types (cluster agents) do not own a shell session.
func agentSessionKey(obj *model.Agent) string {
	if obj == nil || obj.AgentType != model.AgentTypeDeploy {
		return ""
	}
	return agentshell.SessionKey(obj.Id)
}
