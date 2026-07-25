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

	"github.com/caoyingjunz/pixiu/pkg/db"
)

// FactoryLookup looks up cluster names by agent tunnel token via DAO.
type FactoryLookup struct {
	Factory db.ShareDaoFactory
}

func (l FactoryLookup) LookupClusterNameByAgentToken(ctx context.Context, token string) (string, error) {
	obj, err := l.Factory.Cluster().GetClusterByAgentToken(ctx, token)
	if err != nil {
		return "", err
	}
	if obj == nil {
		return "", nil
	}
	return obj.Name, nil
}
