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

// UpgradeInterface 系统升级状态 DAO（单行记录）。
type UpgradeInterface interface {
	// Get 返回升级状态；记录不存在时返回 nil, nil。
	Get(ctx context.Context) (*model.UpgradeRecord, error)
	// Save 保存升级状态（Id 固定为 1，不存在则创建）。
	Save(ctx context.Context, object *model.UpgradeRecord) error
}

type upgrade struct {
	db *gorm.DB
}

func newUpgrade(db *gorm.DB) UpgradeInterface {
	return &upgrade{db: db}
}

func (u *upgrade) Get(ctx context.Context) (*model.UpgradeRecord, error) {
	var object model.UpgradeRecord
	if err := u.db.WithContext(ctx).Where("id = ?", 1).First(&object).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (u *upgrade) Save(ctx context.Context, object *model.UpgradeRecord) error {
	now := time.Now()
	object.Id = 1
	if object.GmtCreate.IsZero() {
		object.GmtCreate = now
	}
	object.GmtModified = now
	return u.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(object).Error
}
