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

type LogDatasourceInterface interface {
	Create(ctx context.Context, object *model.ClusterLogDatasource) (*model.ClusterLogDatasource, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.ClusterLogDatasource, error)
	ListByCluster(ctx context.Context, clusterName string) ([]model.ClusterLogDatasource, error)
	GetDefaultByCluster(ctx context.Context, clusterName string) (*model.ClusterLogDatasource, error)
	UpdateDefaultByCluster(ctx context.Context, clusterName string, datasourceId int64) error
}

type logDatasource struct {
	db *gorm.DB
}

func newLogDatasource(db *gorm.DB) LogDatasourceInterface {
	return &logDatasource{db}
}

func (l *logDatasource) Create(ctx context.Context, object *model.ClusterLogDatasource) (*model.ClusterLogDatasource, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := l.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (l *logDatasource) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := l.db.WithContext(ctx).Model(&model.ClusterLogDatasource{}).Where("id = ? and resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (l *logDatasource) Delete(ctx context.Context, id int64) error {
	f := l.db.WithContext(ctx).Where("id = ?", id).Delete(&model.ClusterLogDatasource{})
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (l *logDatasource) Get(ctx context.Context, id int64) (*model.ClusterLogDatasource, error) {
	var datasource model.ClusterLogDatasource
	if err := l.db.WithContext(ctx).Where("id = ?", id).First(&datasource).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &datasource, nil
}

func (l *logDatasource) ListByCluster(ctx context.Context, clusterName string) ([]model.ClusterLogDatasource, error) {
	var datasources []model.ClusterLogDatasource
	if err := l.db.WithContext(ctx).Where("cluster_name = ?", clusterName).Order("id desc").Find(&datasources).Error; err != nil {
		return nil, err
	}
	return datasources, nil
}

func (l *logDatasource) GetDefaultByCluster(ctx context.Context, clusterName string) (*model.ClusterLogDatasource, error) {
	var datasource model.ClusterLogDatasource
	if err := l.db.WithContext(ctx).Where("cluster_name = ? and is_default = ?", clusterName, true).First(&datasource).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &datasource, nil
}

func (l *logDatasource) UpdateDefaultByCluster(ctx context.Context, clusterName string, datasourceId int64) error {
	return l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ClusterLogDatasource{}).
			Where("cluster_name = ?", clusterName).
			Updates(map[string]interface{}{"is_default": false, "gmt_modified": time.Now()}).Error; err != nil {
			return err
		}

		f := tx.Model(&model.ClusterLogDatasource{}).
			Where("cluster_name = ? and id = ?", clusterName, datasourceId).
			Updates(map[string]interface{}{
				"is_default":       true,
				"gmt_modified":     time.Now(),
				"resource_version": gorm.Expr("resource_version + 1"),
			})
		if f.Error != nil {
			return f.Error
		}
		if f.RowsAffected == 0 {
			return errors.ErrRecordNotFound
		}
		return nil
	})
}
