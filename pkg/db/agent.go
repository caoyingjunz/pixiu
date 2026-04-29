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

type AgentInterface interface {
	Create(ctx context.Context, object *model.Agent) (*model.Agent, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.Agent, error)
	List(ctx context.Context, opts ...Options) ([]model.Agent, error)
	Count(ctx context.Context, opts ...Options) (int64, error)
}

type agent struct {
	db *gorm.DB
}

func newAgent(db *gorm.DB) AgentInterface {
	return &agent{db: db}
}

func (a *agent) Create(ctx context.Context, object *model.Agent) (*model.Agent, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *agent) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["resource_version"] = resourceVersion + 1
	updates["gmt_modified"] = time.Now()

	result := a.db.WithContext(ctx).
		Model(&model.Agent{}).
		Where("id = ? AND resource_version = ?", id, resourceVersion).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}
	return nil
}

func (a *agent) Delete(ctx context.Context, id int64) error {
	return a.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Agent{}).Error
}

func (a *agent) Get(ctx context.Context, id int64) (*model.Agent, error) {
	var object model.Agent
	if err := a.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *agent) List(ctx context.Context, opts ...Options) ([]model.Agent, error) {
	var agents []model.Agent
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&agents).Error; err != nil {
		return nil, err
	}
	return agents, nil
}

func (a *agent) Count(ctx context.Context, opts ...Options) (int64, error) {
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}

	var total int64
	err := tx.Model(&model.Agent{}).Count(&total).Error
	return total, err
}
