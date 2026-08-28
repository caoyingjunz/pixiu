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

type PlanInterface interface {
	Create(ctx context.Context, object *model.Plan, opts ...CreatePlanOption) (*model.Plan, error)
	Update(ctx context.Context, pid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, pid int64) (*model.Plan, error)
	Get(ctx context.Context, pid int64) (*model.Plan, error)
	List(ctx context.Context, opts ...Options) ([]model.Plan, error)
	Count(ctx context.Context, opts ...Options) (int64, error)

	// UpdateBy 按条件通用更新（不带 resource_version 乐观锁），字段由调用方组装
	UpdateBy(ctx context.Context, updates map[string]interface{}, opts ...Options) error

	// 子资源接口：节点 / 部署配置 / 部署任务
	Node() NodeInterface
	Config() ConfigInterface
	Task() TaskInterface
}

type plan struct {
	db *gorm.DB
}

type CreatePlanOption func(plan *model.Plan, tx *gorm.DB) (*gorm.DB, error)

func (p *plan) Create(ctx context.Context, object *model.Plan, opts ...CreatePlanOption) (*model.Plan, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if len(opts) == 0 {
		// no transaction
		if err := p.db.WithContext(ctx).Create(object).Error; err != nil {
			return nil, err
		}
		return object, nil
	}

	if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) (err error) {
		if err = tx.Create(object).Error; err != nil {
			return
		}

		for _, opt := range opts {
			if tx, err = opt(object, tx); err != nil {
				return
			}
		}

		return
	}); err != nil {
		return nil, err
	}
	return object, nil
}

func (p *plan) Update(ctx context.Context, pid int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := p.db.WithContext(ctx).Model(&model.Plan{}).Where("id = ? and resource_version = ?", pid, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}

	return nil
}

func (p *plan) Delete(ctx context.Context, pid int64) (*model.Plan, error) {
	object, err := p.Get(ctx, pid)
	if err != nil {
		return nil, err
	}
	if err = p.db.WithContext(ctx).Where("id = ?", pid).Delete(&model.Plan{}).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (p *plan) Get(ctx context.Context, pid int64) (*model.Plan, error) {
	var object model.Plan
	if err := p.db.WithContext(ctx).Where("id = ?", pid).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}

func (p *plan) List(ctx context.Context, opts ...Options) ([]model.Plan, error) {
	var objects []model.Plan
	tx := p.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}

	return objects, nil
}

func (p *plan) Count(ctx context.Context, opts ...Options) (int64, error) {
	var count int64
	tx := p.db.WithContext(ctx).Model(&model.Plan{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// UpdateBy 按条件通用更新：不带 resource_version 乐观锁，适合状态回写等无 rv 场景；
// updates 由调用方组装，gmt_modified 由本方法维护。条件通过 opts 传入（如 db.WithId）。
func (p *plan) UpdateBy(ctx context.Context, updates map[string]interface{}, opts ...Options) error {
	updates["gmt_modified"] = time.Now()

	tx := p.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	f := tx.Model(&model.Plan{}).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (p *plan) Node() NodeInterface {
	return &node{db: p.db}
}

func (p *plan) Config() ConfigInterface {
	return &config{db: p.db}
}

func (p *plan) Task() TaskInterface {
	return &task{db: p.db}
}

func newPlan(db *gorm.DB) *plan {
	return &plan{db}
}
