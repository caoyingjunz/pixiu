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
	"fmt"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func MakeDbOptions(ctx context.Context) (opts []db.Options) {
	exists, ids := httputils.GetIdRangeFromListReq(ctx)
	if exists {
		opts = append(opts, db.WithIDIn(ids...))
	}

	// 超级管理员可以查看所有租户的资源
	user, err := httputils.GetUserFromRequest(ctx)
	fmt.Printf("[DEBUG MakeDbOptions] user - Id: %d, Name: %s, TenantId: %d, Role: %d, err: %v\n",
		user.Id, user.Name, user.TenantId, user.Role, err)
	if err != nil {
		return
	}
	if user.Role != model.RoleRoot {
		fmt.Printf("[DEBUG MakeDbOptions] user.Role(%d) != RoleRoot(%d), user.TenantId: %d\n",
			user.Role, model.RoleRoot, user.TenantId)
		if user.TenantId > 0 {
			opts = append(opts, db.WithTenantId(user.TenantId))
		}
	}
	return
}
