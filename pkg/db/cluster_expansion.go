/*
Copyright 2026 The Pixiu Authors.

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
)

// ClusterExpansionInterface 集群节点扩容记录 DAO。
type ClusterExpansionInterface interface {
	Create(ctx context.Context, object *model.ClusterExpansion) (*model.ClusterExpansion, error)
	Get(ctx context.Context, id int64) (*model.ClusterExpansion, error)
	List(ctx context.Context, opts ...Options) ([]model.ClusterExpansion, error)
	Count(ctx context.Context, opts ...Options) (int64, error)
	Update(ctx context.Context, id int64, updates map[string]interface{}) error
}

type clusterExpansion struct {
	db *gorm.DB
}

func newClusterExpansion(db *gorm.DB) ClusterExpansionInterface {
	return &clusterExpansion{db: db}
}

func (c *clusterExpansion) Create(ctx context.Context, object *model.ClusterExpansion) (*model.ClusterExpansion, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := c.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (c *clusterExpansion) Get(ctx context.Context, id int64) (*model.ClusterExpansion, error) {
	var object model.ClusterExpansion
	if err := c.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		return nil, err
	}
	return &object, nil
}

func (c *clusterExpansion) List(ctx context.Context, opts ...Options) ([]model.ClusterExpansion, error) {
	var objects []model.ClusterExpansion
	tx := c.db.WithContext(ctx).Model(&model.ClusterExpansion{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (c *clusterExpansion) Count(ctx context.Context, opts ...Options) (int64, error) {
	var count int64
	tx := c.db.WithContext(ctx).Model(&model.ClusterExpansion{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Update 内部更新扩容记录，不做乐观锁校验。
func (c *clusterExpansion) Update(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	return c.db.WithContext(ctx).Model(&model.ClusterExpansion{}).Where("id = ?", id).Updates(updates).Error
}
