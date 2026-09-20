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
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type EmailInterface interface {
	Create(ctx context.Context, object *model.Email) (*model.Email, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, id int64) (*model.Email, error)
	Get(ctx context.Context, id int64) (*model.Email, error)
	List(ctx context.Context, opts ...Options) ([]model.Email, error)
	Count(ctx context.Context, opts ...Options) (int64, error)

	// GetBy 按自定义条件过滤查询
	GetBy(ctx context.Context, opts ...Options) (*model.Email, error)
	// ClearDefaultExcept 将除指定 id 外的所有配置 is_default 置为 false（用于默认配置唯一性）
	ClearDefaultExcept(ctx context.Context, exceptID int64) error
}

type email struct {
	db *gorm.DB
}

func (e *email) Create(ctx context.Context, object *model.Email) (*model.Email, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := e.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (e *email) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := e.db.WithContext(ctx).Model(&model.Email{}).Where("id = ? and resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return utilerrors.ErrRecordNotUpdate
	}
	return nil
}

func (e *email) Delete(ctx context.Context, id int64) (*model.Email, error) {
	object, err := e.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, nil
	}
	if err = e.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Email{}).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (e *email) Get(ctx context.Context, id int64) (*model.Email, error) {
	var object model.Email
	if err := e.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

// GetBy 按自定义条件过滤查询；未命中返回 (nil, nil)。
func (e *email) GetBy(ctx context.Context, opts ...Options) (*model.Email, error) {
	var object model.Email
	tx := e.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.First(&object).Error; err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (e *email) List(ctx context.Context, opts ...Options) ([]model.Email, error) {
	var objects []model.Email
	tx := e.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (e *email) Count(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := e.db.WithContext(ctx).Model(&model.Email{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// ClearDefaultExcept 将除 exceptID 外的所有配置 is_default 置为 false。
// exceptID 传 0 表示清除全部（用于新建默认配置前的唯一性清理）。
func (e *email) ClearDefaultExcept(ctx context.Context, exceptID int64) error {
	return e.db.WithContext(ctx).
		Model(&model.Email{}).
		Where("id <> ?", exceptID).
		Update("is_default", false).Error
}

func newEmail(db *gorm.DB) *email {
	return &email{db}
}
