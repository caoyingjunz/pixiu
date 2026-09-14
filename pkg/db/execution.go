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

type ExecutionInterface interface {
	List(ctx context.Context, conversationID int64, page, limit int) ([]model.Execution, int64, error)
	Create(ctx context.Context, object *model.Execution) (*model.Execution, error)
}

type execution struct {
	db *gorm.DB
}

func newExecution(db *gorm.DB) ExecutionInterface {
	return &execution{db}
}

func (a *execution) Create(ctx context.Context, object *model.Execution) (*model.Execution, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *execution) List(ctx context.Context, conversationID int64, page, limit int) ([]model.Execution, int64, error) {
	var items []model.Execution
	var total int64
	tx := a.db.WithContext(ctx).Model(&model.Execution{}).Where("conversation_id = ?", conversationID)
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("id ASC").Offset((page - 1) * limit).Limit(limit).Find(&items).Error
	return items, total, err
}
