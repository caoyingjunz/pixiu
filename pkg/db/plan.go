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
	Create(ctx context.Context, object *model.Plan) (*model.Plan, error)
	Update(ctx context.Context, pid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, pid int64) (*model.Plan, error)
	Get(ctx context.Context, pid int64) (*model.Plan, error)
	List(ctx context.Context) ([]model.Plan, error)

	CreatNode(ctx context.Context, object *model.Node) (*model.Node, error)
	UpdateNode(ctx context.Context, pid int64, resourceVersion int64, updates map[string]interface{}) error
	DeleteNode(ctx context.Context, pid int64) (*model.Node, error)
	GetNode(ctx context.Context, pid int64) (*model.Node, error)
	ListNodes(ctx context.Context) ([]model.Node, error)

	CreatConfig(ctx context.Context, object *model.Config) (*model.Config, error)
	UpdateConfig(ctx context.Context, pid int64, resourceVersion int64, updates map[string]interface{}) error
	DeleteConfig(ctx context.Context, pid int64) (*model.Config, error)
	GetConfig(ctx context.Context, pid int64) (*model.Config, error)
	ListConfigs(ctx context.Context) ([]model.Config, error)
}

type plan struct {
	db *gorm.DB
}

func (p *plan) Create(ctx context.Context, object *model.Plan) (*model.Plan, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := p.db.WithContext(ctx).Create(object).Error; err != nil {
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

	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
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

func (p *plan) List(ctx context.Context) ([]model.Plan, error) {
	var objects []model.Plan
	if err := p.db.WithContext(ctx).Find(&objects).Error; err != nil {
		return nil, err
	}

	return objects, nil
}

func (p *plan) CreatNode(ctx context.Context, object *model.Node) (*model.Node, error) {
	return nil, nil
}

func (p *plan) UpdateNode(ctx context.Context, nodeId int64, resourceVersion int64, updates map[string]interface{}) error {
	return nil
}

func (p *plan) DeleteNode(ctx context.Context, nodeId int64) (*model.Node, error) {
	return nil, nil
}

func (p *plan) GetNode(ctx context.Context, nodeId int64) (*model.Node, error) {
	return nil, nil
}

func (p *plan) ListNodes(ctx context.Context) ([]model.Node, error) {
	return nil, nil
}

func (p *plan) CreatConfig(ctx context.Context, object *model.Config) (*model.Config, error) {
	return nil, nil
}

func (p *plan) UpdateConfig(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error {
	return nil
}

func (p *plan) DeleteConfig(ctx context.Context, cid int64) (*model.Config, error) {
	return nil, nil
}

func (p *plan) GetConfig(ctx context.Context, cid int64) (*model.Config, error) {
	return nil, nil
}

func (p *plan) ListConfigs(ctx context.Context) ([]model.Config, error) {
	return nil, nil
}

func newPlan(db *gorm.DB) *plan {
	return &plan{db}
}
