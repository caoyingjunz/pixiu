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
	"gorm.io/gorm/clause"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// UpgradeInterface 系统升级状态 DAO，只提供简单 CRUD，不包含业务语义。
// 状态机与版本记录约定均在 pkg/controller/upgrade 中实现。
type UpgradeInterface interface {
	Create(ctx context.Context, object *model.ServerVersion) error
	Update(ctx context.Context, id int64, updates map[string]interface{}, opts ...Options) (int64, error)
	List(ctx context.Context, opts ...Options) ([]model.ServerVersion, error)

	// GetBy 按 opts 条件返回单条升级记录；不存在时返回 nil, nil。
	GetBy(ctx context.Context, opts ...Options) (*model.ServerVersion, error)
	// MarkCompleted 原子回写升级完成（status 2→3）并将该记录标记为当前使用版本。
	MarkCompleted(ctx context.Context, id int64, upgradeID, version string) (int64, error)
}

type upgrade struct {
	db *gorm.DB
}

func newUpgrade(db *gorm.DB) UpgradeInterface {
	return &upgrade{db: db}
}

func (u *upgrade) GetBy(ctx context.Context, opts ...Options) (*model.ServerVersion, error) {
	var object model.ServerVersion
	result := u.db.WithContext(ctx)
	for _, opt := range opts {
		result = opt(result)
	}
	if err := result.Take(&object).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (u *upgrade) MarkCompleted(ctx context.Context, id int64, upgradeID, version string) (int64, error) {
	var affected int64
	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.ServerVersion{}).
			Where("id = ? AND status = ? AND upgrade_id = ? AND version_tag = ?",
				id, model.UpgradeStatusUpgrading, upgradeID, version).
			Updates(map[string]interface{}{
				"status":        model.UpgradeStatusCompleted,
				"error_message": "",
				"gmt_modified":  time.Now(),
			})
		affected = result.RowsAffected
		if result.Error != nil {
			return result.Error
		}
		if affected != 1 {
			return nil
		}
		if err := tx.Model(&model.ServerVersion{}).
			Where("is_current = ? AND id <> ?", true, id).
			Update("is_current", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.ServerVersion{}).
			Where("id = ?", id).
			Update("is_current", true).Error
	})
	return affected, err
}

func (u *upgrade) List(ctx context.Context, opts ...Options) ([]model.ServerVersion, error) {
	var objects []model.ServerVersion
	result := u.db.WithContext(ctx)
	for _, opt := range opts {
		result = opt(result)
	}
	if err := result.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (u *upgrade) Create(ctx context.Context, object *model.ServerVersion) error {
	now := time.Now()
	if object.GmtCreate.IsZero() {
		object.GmtCreate = now
	}
	object.GmtModified = now
	// 并发下可能已被另一协程创建；DoNothing 避免覆盖已有状态
	return u.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(object).Error
}

func (u *upgrade) Update(ctx context.Context, id int64, updates map[string]interface{}, opts ...Options) (int64, error) {
	result := u.db.WithContext(ctx).Model(&model.ServerVersion{}).Where("id = ?", id)
	for _, opt := range opts {
		result = opt(result)
	}
	result = result.Updates(updates)
	return result.RowsAffected, result.Error
}
