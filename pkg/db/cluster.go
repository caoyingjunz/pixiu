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
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type ClusterInterface interface {
	Create(ctx context.Context, object *model.Cluster, fns ...func() error) (*model.Cluster, error)
	Update(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, cid int64) (*model.Cluster, error)
	Get(ctx context.Context, cid int64) (*model.Cluster, error)
	List(ctx context.Context) ([]model.Cluster, error)

	GetClusterByName(ctx context.Context, name string) (*model.Cluster, error)
}

// MySQL implementation
type clusterMySQL struct {
	db *gorm.DB
}

func newClusterMySQL(db *gorm.DB) ClusterInterface {
	return &clusterMySQL{db}
}

func (c *clusterMySQL) Create(ctx context.Context, object *model.Cluster, fns ...func() error) (*model.Cluster, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(object).Error; err != nil {
			return err
		}

		for _, fn := range fns {
			if err := fn(); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return object, nil
}

func (c *clusterMySQL) Update(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := c.db.WithContext(ctx).Model(&model.Cluster{}).Where("id = ? and resource_version = ?", cid, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}

	return nil
}

func (c *clusterMySQL) Delete(ctx context.Context, cid int64) (*model.Cluster, error) {
	// 仅当数据库支持回写功能时才能正常
	//if err := c.db.Clauses(clause.Returning{}).Where("id = ?", cid).Delete(&object).Error; err != nil {
	//	return nil, err
	//}
	object, err := c.Get(ctx, cid)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, nil
	}

	if object.Protected {
		return nil, fmt.Errorf("集群开启删除保护，不允许被删除")
	}
	if err = c.db.WithContext(ctx).Where("id = ?", cid).Delete(&model.Cluster{}).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (c *clusterMySQL) Get(ctx context.Context, cid int64) (*model.Cluster, error) {
	var object model.Cluster
	if err := c.db.WithContext(ctx).Where("id = ?", cid).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

func (c *clusterMySQL) List(ctx context.Context) ([]model.Cluster, error) {
	var cs []model.Cluster
	if err := c.db.WithContext(ctx).Find(&cs).Error; err != nil {
		return nil, err
	}

	return cs, nil
}

func (c *clusterMySQL) GetClusterByName(ctx context.Context, name string) (*model.Cluster, error) {
	var object model.Cluster
	if err := c.db.WithContext(ctx).Where("name = ?", name).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

// SQLite implementation
type clusterSQLite struct {
	db *gorm.DB
}

func newClusterSQLite(db *gorm.DB) ClusterInterface {
	return &clusterSQLite{db}
}

func (c *clusterSQLite) Create(ctx context.Context, object *model.Cluster, fns ...func() error) (*model.Cluster, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(object).Error; err != nil {
			return err
		}

		for _, fn := range fns {
			if err := fn(); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return object, nil
}

func (c *clusterSQLite) Update(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := c.db.WithContext(ctx).Model(&model.Cluster{}).Where("id = ? and resource_version = ?", cid, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}

	return nil
}

func (c *clusterSQLite) Delete(ctx context.Context, cid int64) (*model.Cluster, error) {
	// 仅当数据库支持回写功能时才能正常
	//if err := c.db.Clauses(clause.Returning{}).Where("id = ?", cid).Delete(&object).Error; err != nil {
	//	return nil, err
	//}
	object, err := c.Get(ctx, cid)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, nil
	}

	if object.Protected {
		return nil, fmt.Errorf("集群开启删除保护，不允许被删除")
	}
	if err = c.db.WithContext(ctx).Where("id = ?", cid).Delete(&model.Cluster{}).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (c *clusterSQLite) Get(ctx context.Context, cid int64) (*model.Cluster, error) {
	var object model.Cluster
	if err := c.db.WithContext(ctx).Where("id = ?", cid).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

func (c *clusterSQLite) List(ctx context.Context) ([]model.Cluster, error) {
	var cs []model.Cluster
	if err := c.db.WithContext(ctx).Find(&cs).Error; err != nil {
		return nil, err
	}

	return cs, nil
}

func (c *clusterSQLite) GetClusterByName(ctx context.Context, name string) (*model.Cluster, error) {
	var object model.Cluster
	if err := c.db.WithContext(ctx).Where("name = ?", name).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}
