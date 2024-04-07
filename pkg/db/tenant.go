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

type TenantInterface interface {
	Create(ctx context.Context, object *model.Tenant) (*model.Tenant, error)
	Update(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, cid int64) (*model.Tenant, error)
	Get(ctx context.Context, cid int64) (*model.Tenant, error)
	List(ctx context.Context) ([]model.Tenant, error)

	GetTenantByName(ctx context.Context, name string) (*model.Tenant, error)
}

type tenant struct {
	db *gorm.DB
}

func (t *tenant) Create(ctx context.Context, object *model.Tenant) (*model.Tenant, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := t.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (t *tenant) Update(ctx context.Context, tid int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := t.db.WithContext(ctx).Model(&model.Tenant{}).Where("id = ? and resource_version = ?", tid, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}

	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}

	return nil
}

func (t *tenant) Delete(ctx context.Context, tid int64) (*model.Tenant, error) {
	object, err := t.Get(ctx, tid)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, nil
	}
	if err = t.db.WithContext(ctx).Where("id = ?", tid).Delete(&model.Tenant{}).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (t *tenant) Get(ctx context.Context, tid int64) (*model.Tenant, error) {
	var object model.Tenant
	if err := t.db.WithContext(ctx).Where("id = ?", tid).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

func (t *tenant) List(ctx context.Context) ([]model.Tenant, error) {
	var objects []model.Tenant
	if err := t.db.WithContext(ctx).Find(&objects).Error; err != nil {
		return nil, err
	}

	return objects, nil
}

func (t *tenant) GetTenantByName(ctx context.Context, name string) (*model.Tenant, error) {
	var object model.Tenant
	if err := t.db.WithContext(ctx).Where("name = ?", name).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

func newTenant(db *gorm.DB) *tenant {
	return &tenant{db}
}
