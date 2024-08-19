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

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func MakeDbOptions(ctx context.Context) (opts []db.Options) {
	exists, ids := httputils.GetIdRangeFromListReq(ctx)
	if exists {
		opts = append(opts, db.WithIDIn(ids...))
	}
	return
}

func SetIdRangeContext(c *gin.Context, enforcer *casbin.SyncedEnforcer, user *model.User, obj string) error {
	// group
	policies, err := enforcer.GetFilteredNamedGroupingPolicy("g", 0, user.Name)
	if err != nil {
		return err
	}
	if model.HasAdminGroupPolicy(policies) {
		// This user is an admin/root, it's unnecessary to set object IDs list to context.
		return nil
	}

	if policies, err = enforcer.GetFilteredNamedPolicy("p", 0, user.Name, obj); err != nil {
		return err
	}
	if all, ids := model.GetIdRangeFromPolicies(policies); !all {
		// Set a list of object IDs to context.
		httputils.SetIdRangeContext(c, ids)
	}
	// If policy with all operation(*) exists, it's unnecessary to set object IDs list to context.
	return nil
}
