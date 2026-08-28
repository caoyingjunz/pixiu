/*
Copyright 2021 The Pixiu Authors.

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
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

// ConfigInterface plan 子资源：部署配置（configs 表）。
// 约定：tx 为 nil 时使用默认连接（自动提交）；传入事务 *gorm.DB 时在同一事务内执行。
type ConfigInterface interface {
	Create(ctx context.Context, tx *gorm.DB, object *model.Config) error
	Update(ctx context.Context, cfgId int64, resourceVersion int64, updates map[string]interface{}) error

	DeleteByPlan(ctx context.Context, planId int64) error
	GetByPlan(ctx context.Context, planId int64) (*model.Config, error)
}

type config struct {
	db *gorm.DB
}

func (c *config) Create(ctx context.Context, tx *gorm.DB, object *model.Config) error {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	return c.exec(ctx, tx).Create(object).Error
}

// exec 返回本次操作使用的连接：tx 非空优先，否则默认连接。
func (c *config) exec(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return c.db.WithContext(ctx)
}

func (c *config) Update(ctx context.Context, cfgId int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := c.db.WithContext(ctx).Model(&model.Config{}).Where("id = ? and resource_version = ?", cfgId, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}

	return nil
}

func (c *config) DeleteByPlan(ctx context.Context, planId int64) error {
	if err := c.db.WithContext(ctx).Where("plan_id = ?", planId).Delete(&model.Config{}).Error; err != nil {
		return err
	}
	return nil
}

func (c *config) GetByPlan(ctx context.Context, planId int64) (*model.Config, error) {
	var object model.Config
	if err := c.db.WithContext(ctx).Where("plan_id = ?", planId).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}
