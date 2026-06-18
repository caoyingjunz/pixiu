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
	Create(ctx context.Context, object *model.ClusterDatasource) (*model.ClusterDatasource, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.ClusterDatasource, error)
	ListByCluster(ctx context.Context, clusterName string) ([]model.ClusterDatasource, error)
	GetDefaultByCluster(ctx context.Context, clusterName string, datasourceType model.DatasourceType) (*model.ClusterDatasource, error)
	UpdateDefaultByCluster(ctx context.Context, clusterName string, datasourceType model.DatasourceType, datasourceId int64) error
}

type datasource struct {
	db *gorm.DB
}

func newDatasource(db *gorm.DB) DatasourceInterface {
	return &datasource{db}
}

func (l *datasource) Create(ctx context.Context, object *model.ClusterDatasource) (*model.ClusterDatasource, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := l.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (l *datasource) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := l.db.WithContext(ctx).Model(&model.ClusterDatasource{}).Where("id = ? and resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (l *datasource) Delete(ctx context.Context, id int64) error {
	f := l.db.WithContext(ctx).Where("id = ?", id).Delete(&model.ClusterDatasource{})
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (l *datasource) Get(ctx context.Context, id int64) (*model.ClusterDatasource, error) {
	var datasource model.ClusterDatasource
	if err := l.db.WithContext(ctx).Where("id = ?", id).First(&datasource).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &datasource, nil
}

func (l *datasource) ListByCluster(ctx context.Context, clusterName string) ([]model.ClusterDatasource, error) {
	var datasources []model.ClusterDatasource
	if err := l.db.WithContext(ctx).Where("cluster_name = ?", clusterName).Order("id desc").Find(&datasources).Error; err != nil {
		return nil, err
	}
	return datasources, nil
}

func (l *datasource) GetDefaultByCluster(ctx context.Context, clusterName string, datasourceType model.DatasourceType) (*model.ClusterDatasource, error) {
	var datasource model.ClusterDatasource
	if err := l.db.WithContext(ctx).Where("cluster_name = ? and type = ? and is_default = ?", clusterName, datasourceType, true).First(&datasource).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &datasource, nil
}

func (l *datasource) UpdateDefaultByCluster(ctx context.Context, clusterName string, datasourceType model.DatasourceType, datasourceId int64) error {
	return l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ClusterDatasource{}).
			Where("cluster_name = ? and type = ?", clusterName, datasourceType).
			Updates(map[string]interface{}{"is_default": false, "gmt_modified": time.Now()}).Error; err != nil {
			return err
		}

		f := tx.Model(&model.ClusterDatasource{}).
			Where("cluster_name = ? and type = ? and id = ?", clusterName, datasourceType, datasourceId).
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
