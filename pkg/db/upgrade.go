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
// 状态机与单行记录约定均在 pkg/controller/upgrade 中实现。
type UpgradeInterface interface {
	// Get 按 id 返回升级状态；记录不存在时返回 nil, nil。
	Get(ctx context.Context, id int64) (*model.UpgradeRecord, error)
	// Create 创建升级状态记录（已存在时不覆盖）。
	Create(ctx context.Context, object *model.UpgradeRecord) error
	// Update 按 id 更新记录，opts 为附加过滤条件；返回受影响行数。
	Update(ctx context.Context, id int64, updates map[string]interface{}, opts ...Options) (int64, error)
	// Delete 按 id 删除记录。
	Delete(ctx context.Context, id int64) error
}

type upgrade struct {
	db *gorm.DB
}

func newUpgrade(db *gorm.DB) UpgradeInterface {
	return &upgrade{db: db}
}

func (u *upgrade) Get(ctx context.Context, id int64) (*model.UpgradeRecord, error) {
	var object model.UpgradeRecord
	if err := u.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (u *upgrade) Create(ctx context.Context, object *model.UpgradeRecord) error {
	now := time.Now()
	if object.GmtCreate.IsZero() {
		object.GmtCreate = now
	}
	object.GmtModified = now
	// 并发下可能已被另一协程创建；DoNothing 避免覆盖已有状态
	return u.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(object).Error
}

func (u *upgrade) Update(ctx context.Context, id int64, updates map[string]interface{}, opts ...Options) (int64, error) {
	result := u.db.WithContext(ctx).Model(&model.UpgradeRecord{}).Where("id = ?", id)
	for _, opt := range opts {
		result = opt(result)
	}
	result = result.Updates(updates)
	return result.RowsAffected, result.Error
}

func (u *upgrade) Delete(ctx context.Context, id int64) error {
	return u.db.WithContext(ctx).Where("id = ?", id).Delete(&model.UpgradeRecord{}).Error
}
