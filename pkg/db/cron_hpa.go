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
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type CronHpaInterface interface {
	Create(ctx context.Context, object *model.CronHpa) (*model.CronHpa, error)
	Get(ctx context.Context, id int64) (*model.CronHpa, error)
	List(ctx context.Context, opts ...Options) ([]model.CronHpa, error)
	InternalUpdate(ctx context.Context, id int64, updates map[string]interface{}) error
	// ClaimAndUpdate 以 resource_version 乐观锁推进更新，仅当版本匹配时生效；
	// 返回 false 表示已被其他实例认领，调用方应放弃本轮触发，保证至多执行一次
	ClaimAndUpdate(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) (bool, error)
	Delete(ctx context.Context, id int64) error

	AppendHistory(ctx context.Context, object *model.CronHpaHistory) error
	ListHistories(ctx context.Context, cronHpaId int64, limit int) ([]model.CronHpaHistory, error)
	CleanHistories(ctx context.Context, before time.Time) error
}

type cronHpa struct{ db *gorm.DB }

func newCronHpa(db *gorm.DB) CronHpaInterface { return &cronHpa{db: db} }

func (c *cronHpa) Create(ctx context.Context, object *model.CronHpa) (*model.CronHpa, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if object.Status == "" {
		object.Status = model.CronHpaStatusActive
	}
	if err := c.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (c *cronHpa) Get(ctx context.Context, id int64) (*model.CronHpa, error) {
	var object model.CronHpa
	if err := c.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (c *cronHpa) List(ctx context.Context, opts ...Options) ([]model.CronHpa, error) {
	var objects []model.CronHpa
	tx := c.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Order("id ASC").Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (c *cronHpa) InternalUpdate(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	return c.db.WithContext(ctx).Model(&model.CronHpa{}).Where("id = ?", id).Updates(updates).Error
}

func (c *cronHpa) ClaimAndUpdate(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) (bool, error) {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1
	f := c.db.WithContext(ctx).Model(&model.CronHpa{}).
		Where("id = ? AND resource_version = ?", id, resourceVersion).
		Updates(updates)
	if f.Error != nil {
		return false, f.Error
	}
	return f.RowsAffected == 1, nil
}

func (c *cronHpa) Delete(ctx context.Context, id int64) error {
	// 规则与执行历史一并清理
	return c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", id).Delete(&model.CronHpa{}).Error; err != nil {
			return err
		}
		return tx.Where("cron_hpa_id = ?", id).Delete(&model.CronHpaHistory{}).Error
	})
}

func (c *cronHpa) AppendHistory(ctx context.Context, object *model.CronHpaHistory) error {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	return c.db.WithContext(ctx).Create(object).Error
}

func (c *cronHpa) ListHistories(ctx context.Context, cronHpaId int64, limit int) ([]model.CronHpaHistory, error) {
	var objects []model.CronHpaHistory
	tx := c.db.WithContext(ctx).Where("cron_hpa_id = ?", cronHpaId).Order("id DESC")
	if limit > 0 {
		tx = tx.Limit(limit)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (c *cronHpa) CleanHistories(ctx context.Context, before time.Time) error {
	return c.db.WithContext(ctx).Where("executed_at < ?", before).Delete(&model.CronHpaHistory{}).Error
}
