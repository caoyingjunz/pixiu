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

type DatasourceInterface interface {
	Create(ctx context.Context, object *model.Datasource) (*model.Datasource, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.Datasource, error)
	List(ctx context.Context, opts ...Options) ([]model.Datasource, error)
	Count(ctx context.Context, opts ...Options) (int64, error)
}

type datasource struct {
	db *gorm.DB
}

func newDatasource(db *gorm.DB) DatasourceInterface {
	return &datasource{db}
}

func (l *datasource) Create(ctx context.Context, object *model.Datasource) (*model.Datasource, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if !object.IsDefault {
		if err := l.db.WithContext(ctx).Create(object).Error; err != nil {
			return nil, err
		}
		return object, nil
	}

	err := l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(object).Error; err != nil {
			return err
		}

		return tx.Model(&model.Datasource{}).
			Where("cluster_name = ? AND type = ? AND is_default = ? AND id <> ?", object.ClusterName, object.Type, true, object.Id).
			Updates(map[string]interface{}{
				"is_default":       false,
				"gmt_modified":     now,
				"resource_version": gorm.Expr("resource_version + ?", 1),
			}).Error
	})
	if err != nil {
		return nil, err
	}
	return object, nil
}

func (l *datasource) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := l.db.WithContext(ctx).Model(&model.Datasource{}).Where("id = ? and resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (l *datasource) Delete(ctx context.Context, id int64) error {
	f := l.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Datasource{})
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (l *datasource) Get(ctx context.Context, id int64) (*model.Datasource, error) {
	var datasource model.Datasource
	if err := l.db.WithContext(ctx).Where("id = ?", id).First(&datasource).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &datasource, nil
}

func (l *datasource) List(ctx context.Context, opts ...Options) ([]model.Datasource, error) {
	var datasources []model.Datasource
	tx := l.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&datasources).Error; err != nil {
		return nil, err
	}
	return datasources, nil
}

func (l *datasource) Count(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := l.db.WithContext(ctx).Model(&model.Datasource{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
