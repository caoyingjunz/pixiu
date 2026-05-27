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

package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type RoleAPIInterface interface {
	ListAPIIdsByRoleId(ctx context.Context, roleId int64) ([]int64, error)
	ReplaceByRoleId(ctx context.Context, roleId int64, apiIds []int64) error
}

type roleAPI struct {
	db *gorm.DB
}

func (r *roleAPI) ListAPIIdsByRoleId(ctx context.Context, roleId int64) ([]int64, error) {
	var apiIds []int64
	if err := r.db.WithContext(ctx).Model(&model.RoleAPI{}).
		Where("role_id = ?", roleId).
		Pluck("api_id", &apiIds).Error; err != nil {
		return nil, err
	}

	return apiIds, nil
}

func (r *roleAPI) ReplaceByRoleId(ctx context.Context, roleId int64, apiIds []int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleId).Delete(&model.RoleAPI{}).Error; err != nil {
			return err
		}
		if len(apiIds) == 0 {
			return nil
		}

		now := time.Now()
		records := make([]model.RoleAPI, len(apiIds))
		for i, apiId := range apiIds {
			records[i] = model.RoleAPI{
				RoleId: roleId,
				APIId:  apiId,
			}
			records[i].GmtCreate = now
			records[i].GmtModified = now
		}

		return tx.Create(&records).Error
	})
}

func newRoleAPI(db *gorm.DB) *roleAPI {
	return &roleAPI{db}
}
