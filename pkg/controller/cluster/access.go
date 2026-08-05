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

package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// AuthorizeClusterAccess 校验用户是否可访问指定集群（按 id）。
func (c *cluster) AuthorizeClusterAccess(ctx context.Context, user *model.User, clusterId int64) (*model.Cluster, error) {
	if user == nil {
		return nil, errors.ErrUnauthorized
	}
	obj, err := c.factory.Cluster().Get(ctx, clusterId)
	if err != nil {
		klog.Errorf("failed to get cluster(%d): %v", clusterId, err)
		return nil, errors.ErrServerInternal
	}
	if obj == nil {
		return nil, errors.ErrClusterNotFound
	}
	if err = c.ensureClusterAccess(ctx, user, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// AuthorizeClusterAccessByName 校验用户是否可访问指定集群（按 name）。
func (c *cluster) AuthorizeClusterAccessByName(ctx context.Context, user *model.User, clusterName string) (*model.Cluster, error) {
	if user == nil {
		return nil, errors.ErrUnauthorized
	}
	obj, err := c.factory.Cluster().GetBy(ctx, db.WithName(clusterName))
	if err != nil {
		klog.Errorf("failed to get cluster(%s): %v", clusterName, err)
		return nil, errors.ErrServerInternal
	}
	if obj == nil {
		return nil, errors.ErrClusterNotFound
	}
	if err = c.ensureClusterAccess(ctx, user, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

const accessCacheTTL = 30 * time.Second

var accessCache sync.Map // map[string]time.Time  key: "userId_clusterId"

func (c *cluster) ensureClusterAccess(ctx context.Context, user *model.User, obj *model.Cluster) error {
	if user.Role == model.RoleRoot {
		return nil
	}
	if obj.UserId == user.Id {
		return nil
	}

	key := fmt.Sprintf("%d_%d", user.Id, obj.Id)
	if cached, ok := accessCache.Load(key); ok {
		if expiredAt, ok2 := cached.(time.Time); ok2 && time.Now().Before(expiredAt) {
			return nil
		}
		accessCache.Delete(key)
	}

	perms, err := c.factory.Permission().List(ctx, db.WithUser(user.Id), db.WithOwnerCluster(obj.Id))
	if err != nil {
		klog.Errorf("failed to list permissions for user(%d) cluster(%d): %v", user.Id, obj.Id, err)
		return errors.ErrServerInternal
	}
	if len(perms) == 0 {
		return errors.ErrForbidden
	}
	accessCache.Store(key, time.Now().Add(accessCacheTTL))
	return nil
}
