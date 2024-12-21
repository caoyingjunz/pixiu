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

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
	"gorm.io/gorm"
)

type RepositoryInterface interface {
	Create(ctx context.Context, object *model.Repository) (*model.Repository, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.Repository, error)
	GetByName(ctx context.Context, name string) (*model.Repository, error)
	List(ctx context.Context) ([]*model.Repository, error)
}

type repository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) RepositoryInterface {
	return &repository{db}
}

var _ RepositoryInterface = &repository{}

func (r *repository) Create(ctx context.Context, object *model.Repository) (*model.Repository, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := r.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (r *repository) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := r.db.WithContext(ctx).Model(&model.Repository{}).Where("id = ? and resource_version = ? ", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}

	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}

	return nil

}

func (r *repository) Delete(ctx context.Context, id int64) error {
	f := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Repository{})
	if f.Error != nil {
		return f.Error
	}

	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}

	return nil
}

func (r *repository) Get(ctx context.Context, id int64) (*model.Repository, error) {
	var repo model.Repository
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&repo).Error; err != nil {
		return nil, err
	}

	return &repo, nil
}

func (r *repository) GetByName(ctx context.Context, name string) (*model.Repository, error) {
	var repo model.Repository
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&repo).Error; err != nil {
		return nil, err
	}

	return &repo, nil
}

func (r *repository) List(ctx context.Context) ([]*model.Repository, error) {
	var repos []*model.Repository
	if err := r.db.WithContext(ctx).Find(&repos).Error; err != nil {
		return nil, err
	}

	return repos, nil
}
