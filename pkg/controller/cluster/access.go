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
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// AuthorizeClusterAccess 校验用户是否可访问指定集群（按 id）。
// 放行：root / 集群行 owner / 角色 scope 命中该 cluster id。
// 注意：Permission（主集群授权记录）不再作为访问主集群的依据；被授权人应访问自己的子集群行。
// 本方法不校验「能否加载主集群 admin kubeconfig」；用户态按名加载凭证请用 AuthorizeClusterAccessByName。
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

// AuthorizeClusterAccessByName 按集群名鉴权，供用户态加载 kubeconfig / 代理 / exec / 日志等路径使用。
// 返回值是「实际用于加载 kubeconfig 的集群行」：
//   - 直接有权访问且允许使用其 kubeconfig 时，返回该集群本身；
//   - 请求名是主集群、但调用方不能使用主集群 admin kubeconfig 时，若其在该主集群下有授权子集群，
//     则返回子集群行（scoped kubeconfig），从而可访问主集群上的数据源（如 Prometheus），
//     又不会借主集群名拿到 admin 凭证。
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

	directErr := c.ensureClusterAccess(ctx, user, obj)
	if directErr == nil {
		if err = controllerutil.CheckMasterKubeconfigAccess(ctx, obj); err == nil {
			return obj, nil
		}
		// 对主集群有 scope 等访问权但不能用 admin kubeconfig：尝试回落到本人的授权子集群
	}

	if obj.PermissionId == 0 {
		child, findErr := c.findUserAuthorizedChild(ctx, user.Id, obj.Id)
		if findErr != nil {
			return nil, findErr
		}
		if child != nil {
			klog.V(2).Infof("proxy cluster %s: user(%d) resolved to authorized child %s", clusterName, user.Id, child.Name)
			return child, nil
		}
	}

	if directErr != nil {
		return nil, directErr
	}
	return nil, errors.ErrForbidden
}

// findUserAuthorizedChild 查找用户在指定主集群下的授权子集群（PermissionId!=0 且 OwnerReference=master）。
func (c *cluster) findUserAuthorizedChild(ctx context.Context, userId, masterClusterId int64) (*model.Cluster, error) {
	children, err := c.factory.Cluster().List(ctx, db.WithOwnerReference(masterClusterId), db.WithUser(userId))
	if err != nil {
		klog.Errorf("failed to list authorized children of master(%d) for user(%d): %v", masterClusterId, userId, err)
		return nil, errors.ErrServerInternal
	}
	for i := range children {
		if children[i].PermissionId != 0 {
			return &children[i], nil
		}
	}
	return nil, nil
}

// AuthorizeClusterKubeAccess 按 id 鉴权并禁止非 owner/root 加载主集群 admin kubeconfig。
// 用于签发 proxy-kubeconfig 等按 id 且会暴露/绑定集群凭证的路径；
// CloudShell 等仅做资源访问、不在此校验凭证归属的场景请用 AuthorizeClusterAccess。
func (c *cluster) AuthorizeClusterKubeAccess(ctx context.Context, user *model.User, clusterId int64) (*model.Cluster, error) {
	obj, err := c.AuthorizeClusterAccess(ctx, user, clusterId)
	if err != nil {
		return nil, err
	}
	if err = controllerutil.CheckMasterKubeconfigAccess(ctx, obj); err != nil {
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

	ok, err := c.factory.Role().Scope().HasScope(ctx, int64(user.Role), types.ResourceTypeCluster, obj.Id)
	if err != nil {
		klog.Errorf("failed to check cluster scope for user(%d) cluster(%d): %v", user.Id, obj.Id, err)
		return errors.ErrServerInternal
	}
	if !ok {
		return errors.ErrForbidden
	}
	accessCache.Store(key, time.Now().Add(accessCacheTTL))
	return nil
}

// InvalidateClusterAccessCache 在权限/归属变更后清除指定用户对集群的访问缓存。
func InvalidateClusterAccessCache(userId, clusterId int64) {
	accessCache.Delete(fmt.Sprintf("%d_%d", userId, clusterId))
}
