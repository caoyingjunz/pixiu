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

type RoleAPIScopeInterface interface {
	ListByRoleId(ctx context.Context, roleId int64) ([]model.RoleAPIScope, error)
	ReplaceByRoleId(ctx context.Context, roleId int64, scopes []model.RoleAPIScope) error
}

type roleAPIScope struct {
	db *gorm.DB
}

func (r *roleAPIScope) ListByRoleId(ctx context.Context, roleId int64) ([]model.RoleAPIScope, error) {
	var scopes []model.RoleAPIScope
	if err := r.db.WithContext(ctx).Model(&model.RoleAPIScope{}).
		Where("role_id = ?", roleId).
		Find(&scopes).Error; err != nil {
		return nil, err
	}
	return scopes, nil
}

func (r *roleAPIScope) ReplaceByRoleId(ctx context.Context, roleId int64, scopes []model.RoleAPIScope) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleId).Delete(&model.RoleAPIScope{}).Error; err != nil {
			return err
		}
		if len(scopes) == 0 {
			return nil
		}

		now := time.Now()
		for i := range scopes {
			scopes[i].RoleId = roleId
			scopes[i].GmtCreate = now
			scopes[i].GmtModified = now
		}
		return tx.Create(&scopes).Error
	})
}

func newRoleAPIScope(db *gorm.DB) *roleAPIScope {
	return &roleAPIScope{db}
}
