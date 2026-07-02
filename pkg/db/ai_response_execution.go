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

type AIResponseExecutionInterface interface {
	Create(ctx context.Context, object *model.AIResponseExecution) (*model.AIResponseExecution, error)
}

type aiResponseExecution struct {
	db *gorm.DB
}

func newAIResponseExecution(db *gorm.DB) AIResponseExecutionInterface {
	return &aiResponseExecution{db: db}
}

func (a *aiResponseExecution) Create(ctx context.Context, object *model.AIResponseExecution) (*model.AIResponseExecution, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}
