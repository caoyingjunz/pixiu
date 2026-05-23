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

type APIInterface interface {
	Create(ctx context.Context, object *model.API) (*model.API, error)
	Update(ctx context.Context, aid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, aid int64) (*model.API, error)
	Get(ctx context.Context, aid int64) (*model.API, error)
	List(ctx context.Context, opts ...Options) ([]model.API, error)
	Count(ctx context.Context, opts ...Options) (int64, error)

	GetByMethodAndPath(ctx context.Context, method, path string) (*model.API, error)
}

type apis struct {
	db *gorm.DB
}

func (a *apis) Create(ctx context.Context, object *model.API) (*model.API, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *apis) Update(ctx context.Context, aid int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := a.db.WithContext(ctx).Model(&model.API{}).Where("id = ? and resource_version = ?", aid, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}

	return nil
}

func (a *apis) Delete(ctx context.Context, aid int64) (*model.API, error) {
	object, err := a.Get(ctx, aid)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, nil
	}
	if err = a.db.WithContext(ctx).Where("id = ?", aid).Delete(&model.API{}).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (a *apis) Get(ctx context.Context, aid int64) (*model.API, error) {
	var object model.API
	if err := a.db.WithContext(ctx).Where("id = ?", aid).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

func (a *apis) List(ctx context.Context, opts ...Options) ([]model.API, error) {
	var objects []model.API
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}

	return objects, nil
}

func (a *apis) Count(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := a.db.WithContext(ctx).Model(&model.API{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}

	return total, nil
}

func (a *apis) GetByMethodAndPath(ctx context.Context, method, path string) (*model.API, error) {
	var object model.API
	if err := a.db.WithContext(ctx).Where("method = ? AND path = ?", method, path).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

func newAPIs(db *gorm.DB) *apis {
	return &apis{db}
}
