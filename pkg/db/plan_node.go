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

// NodeInterface plan 子资源：主机节点（nodes 表）。
// 约定：tx 为 nil 时使用默认连接（自动提交）；传入事务 *gorm.DB 时在同一事务内执行。
type NodeInterface interface {
	Create(ctx context.Context, tx *gorm.DB, object *model.Node) (*model.Node, error)
	Update(ctx context.Context, tx *gorm.DB, nodeId int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, nodeId int64) (*model.Node, error)
	Get(ctx context.Context, nodeId int64) (*model.Node, error)
	List(ctx context.Context, opts ...Options) ([]model.Node, error)

	GetByIP(ctx context.Context, ip string) (*model.Node, error)
	Count(ctx context.Context, opts ...Options) (int64, error)
}

type node struct {
	db *gorm.DB
}

func (n *node) Create(ctx context.Context, tx *gorm.DB, object *model.Node) (*model.Node, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := n.exec(ctx, tx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (n *node) Update(ctx context.Context, tx *gorm.DB, nodeId int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := n.exec(ctx, tx).Model(&model.Node{}).Where("id = ? and resource_version = ?", nodeId, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

// exec 返回本次操作使用的连接：tx 非空优先，否则默认连接。
func (n *node) exec(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return n.db.WithContext(ctx)
}

func (n *node) Delete(ctx context.Context, nodeId int64) (*model.Node, error) {
	object, err := n.Get(ctx, nodeId)
	if err != nil {
		return nil, err
	}
	if err = n.db.WithContext(ctx).Where("id = ?", nodeId).Delete(&model.Node{}).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (n *node) Get(ctx context.Context, nodeId int64) (*model.Node, error) {
	var object model.Node
	if err := n.db.WithContext(ctx).Where("id = ?", nodeId).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}

func (n *node) GetByIP(ctx context.Context, ip string) (*model.Node, error) {
	var object model.Node
	if err := n.db.WithContext(ctx).Where("ip = ?", ip).First(&object).Error; err != nil {
		return nil, err
	}
	return &object, nil
}

func (n *node) List(ctx context.Context, opts ...Options) ([]model.Node, error) {
	var objects []model.Node
	tx := n.db.WithContext(ctx).Model(&model.Node{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (n *node) Count(ctx context.Context, opts ...Options) (int64, error) {
	var count int64
	tx := n.db.WithContext(ctx).Model(&model.Node{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
