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

	CreateNode(ctx context.Context, object *model.Node) (*model.Node, error)
	TxCreateNode(ctx context.Context, tx *gorm.DB, object *model.Node) error
	UpdateNode(ctx context.Context, nodeId int64, resourceVersion int64, updates map[string]interface{}) error
	DeleteNode(ctx context.Context, nodeId int64) (*model.Node, error)
	GetNode(ctx context.Context, nodeId int64) (*model.Node, error)
	ListNodes(ctx context.Context, pid int64, opts ...Options) ([]model.Node, error)

	DeleteNodesByPlan(ctx context.Context, planId int64) error
	GetNodeByName(ctx context.Context, planId int64, name string) (*model.Node, error)

	DeleteNodesByNames(ctx context.Context, planId int64, names []string) error

	CreateConfig(ctx context.Context, object *model.Config) (*model.Config, error)
	TxCreateConfig(ctx context.Context, tx *gorm.DB, object *model.Config) error
	UpdateConfig(ctx context.Context, cfgId int64, resourceVersion int64, updates map[string]interface{}) error
	DeleteConfig(ctx context.Context, cfgId int64) (*model.Config, error)
	GetConfig(ctx context.Context, cfgId int64) (*model.Config, error)
	ListConfigs(ctx context.Context, opts ...Options) ([]model.Config, error)

	DeleteConfigByPlan(ctx context.Context, planId int64) error
	GetConfigByPlan(ctx context.Context, planId int64) (*model.Config, error)

	CreateTask(ctx context.Context, object *model.Task) (*model.Task, error)
	UpdateTask(ctx context.Context, pid int64, name string, updates map[string]interface{}) (*model.Task, error)
	DeleteTask(ctx context.Context, pid int64) error
	ListTasks(ctx context.Context, pid int64, opts ...Options) ([]model.Task, error)

	GetNewestTask(ctx context.Context, pid int64) (*model.Task, error)
	GetTaskByName(ctx context.Context, planId int64, name string) (*model.Task, error)
	GetTaskById(ctx context.Context, taskId int64) (*model.Task, error)
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

func (p *plan) CreateNode(ctx context.Context, object *model.Node) (*model.Node, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := p.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

// TxCreateNode creates a node object in the given transaction.
func (p *plan) TxCreateNode(ctx context.Context, tx *gorm.DB, object *model.Node) error {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	return tx.WithContext(ctx).Create(object).Error
}

func (p *plan) UpdateNode(ctx context.Context, nodeId int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := p.db.WithContext(ctx).Model(&model.Node{}).Where("id = ? and resource_version = ?", nodeId, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}

	return nil
}

func (p *plan) DeleteNode(ctx context.Context, nodeId int64) (*model.Node, error) {
	object, err := p.GetNode(ctx, nodeId)
	if err != nil {
		return nil, err
	}
	if err = p.db.WithContext(ctx).Where("id = ?", nodeId).Delete(&model.Node{}).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (p *plan) DeleteNodesByPlan(ctx context.Context, planId int64) error {
	if err := p.db.WithContext(ctx).Where("plan_id = ?", planId).Delete(&model.Node{}).Error; err != nil {
		return err
	}

	return nil
}

func (p *plan) DeleteNodesByNames(ctx context.Context, planId int64, names []string) error {
	if err := p.db.WithContext(ctx).Where("plan_id = ? and name in (?)", planId, names).Delete(&model.Node{}).Error; err != nil {
		return err
	}

	return nil
}

func (p *plan) GetNodeByName(ctx context.Context, planId int64, name string) (*model.Node, error) {
	var object model.Node
	if err := p.db.WithContext(ctx).Where("plan_id = ? and name = ?", planId, name).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}

func (p *plan) GetNode(ctx context.Context, nodeId int64) (*model.Node, error) {
	var object model.Node
	if err := p.db.WithContext(ctx).Where("id = ?", nodeId).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}

func (p *plan) ListNodes(ctx context.Context, pid int64, opts ...Options) ([]model.Node, error) {
	var objects []model.Node
	tx := p.db.WithContext(ctx).Where("plan_id = ?", pid)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}

	return objects, nil
}

func (p *plan) CreateConfig(ctx context.Context, object *model.Config) (*model.Config, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := p.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

// TxCreateConfig creates a config object in the given transaction.
func (p *plan) TxCreateConfig(ctx context.Context, tx *gorm.DB, object *model.Config) error {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	return tx.WithContext(ctx).Create(object).Error
}

func (p *plan) UpdateConfig(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := p.db.WithContext(ctx).Model(&model.Config{}).Where("id = ? and resource_version = ?", cid, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}

	return nil
}

func (p *plan) DeleteConfig(ctx context.Context, cid int64) (*model.Config, error) {
	object, err := p.GetConfig(ctx, cid)
	if err != nil {
		return nil, err
	}
	if err = p.db.WithContext(ctx).Where("id = ?", cid).Delete(&model.Config{}).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (p *plan) DeleteConfigByPlan(ctx context.Context, planId int64) error {
	if err := p.db.WithContext(ctx).Where("plan_id = ?", planId).Delete(&model.Config{}).Error; err != nil {
		return err
	}
	return nil
}

func (p *plan) GetConfig(ctx context.Context, cid int64) (*model.Config, error) {
	var object model.Config
	if err := p.db.WithContext(ctx).Where("id = ?", cid).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}

func (p *plan) ListConfigs(ctx context.Context, opts ...Options) ([]model.Config, error) {
	var objects []model.Config
	tx := p.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}

	return objects, nil
}

func (p *plan) GetConfigByPlan(ctx context.Context, planId int64) (*model.Config, error) {
	var object model.Config
	if err := p.db.WithContext(ctx).Where("plan_id = ?", planId).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}

func (p *plan) CreateTask(ctx context.Context, object *model.Task) (*model.Task, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := p.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (p *plan) UpdateTask(ctx context.Context, pid int64, name string, updates map[string]interface{}) (*model.Task, error) {
	f := p.db.WithContext(ctx).Model(&model.Task{}).Where("plan_id = ? and name = ?", pid, name).Updates(updates)
	if f.Error != nil {
		return nil, f.Error
	}
	if f.RowsAffected == 0 {
		return nil, errors.ErrRecordNotFound
	}

	return p.GetTaskByName(ctx, pid, name)
}

func (p *plan) DeleteTask(ctx context.Context, pid int64) error {
	if err := p.db.WithContext(ctx).Where("plan_id = ?", pid).Delete(&model.Task{}).Error; err != nil {
		return err
	}

	return nil
}

func (p *plan) ListTasks(ctx context.Context, pid int64, opts ...Options) ([]model.Task, error) {
	var objects []model.Task
	tx := p.db.WithContext(ctx).Where("plan_id = ?", pid)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}

	return objects, nil
}

func (p *plan) GetNewestTask(ctx context.Context, pid int64) (*model.Task, error) {
	var objects []model.Task
	if err := p.db.WithContext(ctx).Where("plan_id = ?", pid).Order("id DESC").Limit(1).Find(&objects).Error; err != nil {
		return nil, err
	}

	if len(objects) == 0 {
		return nil, errors.ErrRecordNotFound
	}
	return &objects[0], nil
}

func (p *plan) GetTaskById(ctx context.Context, taskId int64) (*model.Task, error) {
	var object model.Task
	if err := p.db.WithContext(ctx).Where("id = ?", taskId).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}

func (p *plan) GetTaskByName(ctx context.Context, planId int64, name string) (*model.Task, error) {
	var object model.Task
	if err := p.db.WithContext(ctx).Where("plan_id = ? and name = ?", planId, name).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}

func newPlan(db *gorm.DB) *plan {
	return &plan{db}
}
