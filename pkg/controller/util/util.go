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

package util

import (
	"context"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/rbac/scope"
)

// UserNameFromContext 返回当前登录用户名；未登录或从上下文获取异常时返回空串。
func UserNameFromContext(ctx context.Context) (string, error) {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return "", err
	}

	return user.Name, nil
}

// EffectiveUserID 返回查询/创建实际生效的 user_id：
// 仅超级管理员（RoleRoot）可指定（0=全部）；管理员与普通用户强制为当前登录用户。
func EffectiveUserID(ctx context.Context, reqUserID int64) (int64, error) {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return 0, err
	}
	if user.Role == model.RoleRoot {
		return reqUserID, nil
	}
	return user.Id, nil
}

// CheckResourceOwner 校验当前用户是否有权操作该资源：非超级管理员必须为 owner。
func CheckResourceOwner(ctx context.Context, resourceOwnerID int64) error {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return err
	}
	if user.Role == model.RoleRoot {
		return nil
	}
	if user.Id != resourceOwnerID {
		return errors.ErrForbidden
	}
	return nil
}

// RequireRoot 仅允许超级管理员执行敏感变更（如角色 API / ACL 目录）。
func RequireRoot(ctx context.Context) error {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return err
	}
	if user.Role != model.RoleRoot {
		return errors.ErrForbidden
	}
	return nil
}

// CheckResourceAccess 校验当前用户是否有权访问 pixiu 资源：
// 超管放行 → owner 放行 → 角色 scope 命中放行 → 否则 403
func CheckResourceAccess(ctx context.Context, factory db.ShareDaoFactory, resourceOwnerID int64, resourceType string, resourceId int64) error {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return err
	}
	isRoot := user.Role == model.RoleRoot
	isOwner := user.Id == resourceOwnerID
	hasScope := false
	if !isRoot && !isOwner {
		ok, scopeErr := factory.Role().Scope().HasScope(ctx, int64(user.Role), resourceType, resourceId)
		if scopeErr != nil {
			return scopeErr
		}
		hasScope = ok
	}
	if scope.CanAccess(isRoot, isOwner, hasScope) {
		return nil
	}
	return errors.ErrForbidden
}

// AuthorizedResourceIDs 返回当前用户角色在指定资源类型下被授权的实例 ID。
// 超管返回 nil（表示不按 scope 额外过滤）；非超管返回 scope 列表（可能为空切片）。
func AuthorizedResourceIDs(ctx context.Context, factory db.ShareDaoFactory, resourceType string) ([]int64, error) {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if !scope.NeedFilter(user.Role == model.RoleRoot) {
		return nil, nil
	}
	return factory.Role().Scope().ListResourceIDsByRole(ctx, int64(user.Role), resourceType)
}
