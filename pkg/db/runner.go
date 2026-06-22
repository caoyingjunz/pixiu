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

type RunnerInterface interface {
	Create(ctx context.Context, object *model.Runner) (*model.Runner, error)
	Update(ctx context.Context, rid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, rid int64) (*model.Runner, error)
	Get(ctx context.Context, rid int64) (*model.Runner, error)
	List(ctx context.Context, opts ...Options) ([]model.Runner, error)
	Count(ctx context.Context, opts ...Options) (int64, error)

	GetBy(ctx context.Context, name string) (*model.Runner, error)
}

type runner struct {
	db *gorm.DB
}

func (r *runner) Create(ctx context.Context, object *model.Runner) (*model.Runner, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := r.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (r *runner) Update(ctx context.Context, rid int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := r.db.WithContext(ctx).Model(&model.Runner{}).Where("id = ? and resource_version = ?", rid, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (r *runner) Delete(ctx context.Context, rid int64) (*model.Runner, error) {
	object, err := r.Get(ctx, rid)
	if err != nil {
		return nil, err
	}
	if err = r.db.WithContext(ctx).Where("id = ?", rid).Delete(&model.Runner{}).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (r *runner) Get(ctx context.Context, rid int64) (*model.Runner, error) {
	var object model.Runner
	if err := r.db.WithContext(ctx).Where("id = ?", rid).First(&object).Error; err != nil {
		return nil, err
	}
	return &object, nil
}

func (r *runner) GetBy(ctx context.Context, name string) (*model.Runner, error) {
	var object model.Runner
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&object).Error; err != nil {
		return nil, err
	}
	return &object, nil
}

func (r *runner) List(ctx context.Context, opts ...Options) ([]model.Runner, error) {
	var objects []model.Runner
	tx := r.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (r *runner) Count(ctx context.Context, opts ...Options) (int64, error) {
	var count int64
	tx := r.db.WithContext(ctx).Model(&model.Runner{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func newRunner(db *gorm.DB) *runner {
	return &runner{db}
}
