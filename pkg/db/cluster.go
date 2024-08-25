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

type ClusterInterface interface {
	Create(ctx context.Context, object *model.Cluster, fns ...func(*model.Cluster) error) (*model.Cluster, error)
	Update(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, cluster *model.Cluster, fns ...func(*model.Cluster) error) error
	Get(ctx context.Context, cid int64, opts ...Options) (*model.Cluster, error)
	List(ctx context.Context, opts ...Options) ([]model.Cluster, error)

	// InternalUpdate 内部更新，不更新版本号
	InternalUpdate(ctx context.Context, cid int64, updates map[string]interface{}) error

	GetClusterByName(ctx context.Context, name string) (*model.Cluster, error)
	UpdateByPlan(ctx context.Context, planId int64, updates map[string]interface{}) error
}

type cluster struct {
	db *gorm.DB
}

func (c *cluster) Create(ctx context.Context, object *model.Cluster, fns ...func(*model.Cluster) error) (*model.Cluster, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(object).Error; err != nil {
			return err
		}

		for _, fn := range fns {
			if err := fn(object); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return object, nil
}

func (c *cluster) Update(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error {
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

// InternalUpdate 程序内部更新
func (c *cluster) InternalUpdate(ctx context.Context, cid int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	f := c.db.WithContext(ctx).Model(&model.Cluster{}).Where("id = ?", cid).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}

	return nil
}

func (c *cluster) Delete(ctx context.Context, cluster *model.Cluster, fns ...func(*model.Cluster) error) error {
	// 仅当数据库支持回写功能时才能正常
	//if err := c.db.Clauses(clause.Returning{}).Where("id = ?", cid).Delete(&object).Error; err != nil {
	//	return nil, err
	//}
	return c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(cluster).Error; err != nil {
			return err
		}

		for _, fn := range fns {
			if err := fn(cluster); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *cluster) Get(ctx context.Context, cid int64, opts ...Options) (*model.Cluster, error) {
	var object model.Cluster
	tx := c.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.First(&object, cid).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

func (c *cluster) List(ctx context.Context, opts ...Options) ([]model.Cluster, error) {
	var cs []model.Cluster
	tx := c.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&cs).Error; err != nil {
		return nil, err
	}

	return cs, nil
}

func (c *cluster) GetClusterByName(ctx context.Context, name string) (*model.Cluster, error) {
	var object model.Cluster
	if err := c.db.WithContext(ctx).Where("name = ?", name).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

func (c *cluster) UpdateByPlan(ctx context.Context, planId int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()

	f := c.db.WithContext(ctx).Model(&model.Cluster{}).Where("plan_id = ?", planId).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}

	return nil
}

func newCluster(db *gorm.DB) ClusterInterface {
	return &cluster{db}
}
