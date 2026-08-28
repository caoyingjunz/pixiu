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

// TaskInterface plan 子资源：部署任务（tasks 表）
type TaskInterface interface {
	Create(ctx context.Context, object *model.Task) (*model.Task, error)
	Update(ctx context.Context, pid int64, name string, updates map[string]interface{}) (*model.Task, error)
	Delete(ctx context.Context, pid int64) error
	List(ctx context.Context, pid int64, opts ...Options) ([]model.Task, error)

	GetByName(ctx context.Context, planId int64, name string) (*model.Task, error)
	GetByID(ctx context.Context, taskId int64) (*model.Task, error)
}

type task struct {
	db *gorm.DB
}

func (t *task) Create(ctx context.Context, object *model.Task) (*model.Task, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := t.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (t *task) Update(ctx context.Context, pid int64, name string, updates map[string]interface{}) (*model.Task, error) {
	f := t.db.WithContext(ctx).Model(&model.Task{}).Where("plan_id = ? and name = ?", pid, name).Updates(updates)
	if f.Error != nil {
		return nil, f.Error
	}
	if f.RowsAffected == 0 {
		return nil, errors.ErrRecordNotFound
	}

	return t.GetByName(ctx, pid, name)
}

func (t *task) Delete(ctx context.Context, pid int64) error {
	if err := t.db.WithContext(ctx).Where("plan_id = ?", pid).Delete(&model.Task{}).Error; err != nil {
		return err
	}

	return nil
}

func (t *task) List(ctx context.Context, pid int64, opts ...Options) ([]model.Task, error) {
	var objects []model.Task
	tx := t.db.WithContext(ctx).Where("plan_id = ?", pid)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}

	return objects, nil
}

func (t *task) GetByName(ctx context.Context, planId int64, name string) (*model.Task, error) {
	var object model.Task
	if err := t.db.WithContext(ctx).Where("plan_id = ? and name = ?", planId, name).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}

func (t *task) GetByID(ctx context.Context, taskId int64) (*model.Task, error) {
	var object model.Task
	if err := t.db.WithContext(ctx).Where("id = ?", taskId).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}
