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
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func CurrentUserName(ctx context.Context) string {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil || user == nil {
		return ""
	}
	return user.Name
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

// CurrentUserID 返回当前登录用户 id（创建资源归属用，一律当前用户）。
func CurrentUserID(ctx context.Context) (int64, error) {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return 0, err
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
