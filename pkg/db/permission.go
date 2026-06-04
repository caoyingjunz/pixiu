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
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type PermissionInterface interface {
	Create(ctx context.Context, object *model.Permission) (*model.Permission, error)
	Update(ctx context.Context, pid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, pid int64) (*model.Permission, error)
	Get(ctx context.Context, pid int64) (*model.Permission, error)
	List(ctx context.Context, opts ...Options) ([]model.Permission, error)
	Count(ctx context.Context, opts ...Options) (int64, error)
}

type permission struct {
	db *gorm.DB
}

func (p *permission) Create(ctx context.Context, object *model.Permission) (*model.Permission, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := p.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (p *permission) Update(ctx context.Context, pid int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := p.db.WithContext(ctx).Model(&model.Permission{}).
		Where("id = ? and resource_version = ?", pid, resourceVersion).
		Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}
	return nil
}

func (p *permission) Delete(ctx context.Context, pid int64) (*model.Permission, error) {
	object, err := p.Get(ctx, pid)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, nil
	}
	if err = p.db.WithContext(ctx).Where("id = ?", pid).Delete(&model.Permission{}).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (p *permission) Get(ctx context.Context, pid int64) (*model.Permission, error) {
	var object model.Permission
	if err := p.db.WithContext(ctx).Where("id = ?", pid).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (p *permission) List(ctx context.Context, opts ...Options) ([]model.Permission, error) {
	var objects []model.Permission
	tx := p.db.WithContext(ctx).Model(&model.Permission{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (p *permission) Count(ctx context.Context, opts ...Options) (int64, error) {
	var count int64
	tx := p.db.WithContext(ctx).Model(&model.Permission{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func newPermission(db *gorm.DB) PermissionInterface {
	return &permission{db: db}
}
