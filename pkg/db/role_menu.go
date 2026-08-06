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

package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type MenuInterface interface {
	ListMenuCodesByRoleId(ctx context.Context, roleId int64) ([]string, error)
	ReplaceByRoleId(ctx context.Context, roleId int64, menuCodes []string) error
}

type roleMenu struct {
	db *gorm.DB
}

func (r *roleMenu) ListMenuCodesByRoleId(ctx context.Context, roleId int64) ([]string, error) {
	var codes []string
	if err := r.db.WithContext(ctx).Model(&model.RoleMenu{}).
		Where("role_id = ?", roleId).
		Pluck("menu_code", &codes).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

func (r *roleMenu) ReplaceByRoleId(ctx context.Context, roleId int64, menuCodes []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleId).Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		if len(menuCodes) == 0 {
			return nil
		}

		now := time.Now()
		records := make([]model.RoleMenu, len(menuCodes))
		for i, code := range menuCodes {
			records[i] = model.RoleMenu{
				RoleId:   roleId,
				MenuCode: code,
			}
			records[i].GmtCreate = now
			records[i].GmtModified = now
		}
		return tx.Create(&records).Error
	})
}

func newRoleMenu(db *gorm.DB) *roleMenu {
	return &roleMenu{db: db}
}
