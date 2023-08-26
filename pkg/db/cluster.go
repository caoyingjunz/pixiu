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
	Create(ctx context.Context, object *model.Cluster) (*model.Cluster, error)
	Update(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, cid int64) error
	Get(ctx context.Context, cid int64) (*model.Cluster, error)
	List(ctx context.Context) ([]model.Cluster, error)
}

type cluster struct {
	db *gorm.DB
}

func (c *cluster) Create(ctx context.Context, object *model.Cluster) (*model.Cluster, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := c.db.Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil

}

func (c *cluster) Update(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := c.db.Model(&model.Cluster{}).Where("id = ? and resource_version = ?", cid, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}

	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}

	return nil
}

func (c *cluster) Delete(ctx context.Context, cid int64) error {
	if err := c.db.Where("id = ?", cid).Delete(&model.Cluster{}).Error; err != nil {
		return err
	}

	return nil
}

func (c *cluster) Get(ctx context.Context, cid int64) (*model.Cluster, error) {
	var object model.Cluster
	if err := c.db.Where("id = ?", cid).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}

func (c *cluster) List(ctx context.Context) ([]model.Cluster, error) {
	var cs []model.Cluster
	if err := c.db.Find(&cs).Error; err != nil {
		return nil, err
	}

	return cs, nil
}

func newCluster(db *gorm.DB) ClusterInterface {
	return &cluster{db}
}
