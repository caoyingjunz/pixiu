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

type RoleInterface interface {
	Create(ctx context.Context, object *model.Role) (*model.Role, error)
	Update(ctx context.Context, rid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, rid int64) (*model.Role, error)
	Get(ctx context.Context, rid int64) (*model.Role, error)
	List(ctx context.Context, opts ...Options) ([]model.Role, error)
	Count(ctx context.Context, opts ...Options) (int64, error)

	GetRoleByTenantAndName(ctx context.Context, tenantId int64, name string) (*model.Role, error)
}

type role struct {
	db *gorm.DB
}

func (r *role) Create(ctx context.Context, object *model.Role) (*model.Role, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := r.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (r *role) Update(ctx context.Context, rid int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := r.db.WithContext(ctx).Model(&model.Role{}).Where("id = ? and resource_version = ?", rid, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}

	return nil
}

func (r *role) Delete(ctx context.Context, rid int64) (*model.Role, error) {
	object, err := r.Get(ctx, rid)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, nil
	}
	if err = r.db.WithContext(ctx).Where("id = ?", rid).Delete(&model.Role{}).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (r *role) Get(ctx context.Context, rid int64) (*model.Role, error) {
	var object model.Role
	if err := r.db.WithContext(ctx).Where("id = ?", rid).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

func (r *role) List(ctx context.Context, opts ...Options) ([]model.Role, error) {
	var objects []model.Role
	tx := r.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}

	return objects, nil
}

func (r *role) Count(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := r.db.WithContext(ctx).Model(&model.Role{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}

	return total, nil
}

func (r *role) GetRoleByTenantAndName(ctx context.Context, tenantId int64, name string) (*model.Role, error) {
	var object model.Role
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND name = ?", tenantId, name).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

func newRole(db *gorm.DB) *role {
	return &role{db}
}
