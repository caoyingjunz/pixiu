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

package audit

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
)

// Interface 审计数据访问层
type Interface interface {
	Create(ctx context.Context, obj *model.Event) (*model.Event, error)
	Delete(ctx context.Context, eventId int64) error
	List(ctx context.Context, page, limit int) ([]model.Event, error)
}

type audit struct {
	db *gorm.DB
}

func NewAudit(db *gorm.DB) Interface {
	return &audit{db}
}

func (audit *audit) Create(ctx context.Context, obj *model.Event) (*model.Event, error) {
	now := time.Now()
	obj.GmtCreate = now
	obj.GmtModified = now
	if err := audit.db.Create(obj).Error; err != nil {
		return nil, err
	}
	return obj, nil
}

func (audit *audit) Delete(ctx context.Context, eventId int64) error {
	return nil
}

func (audit *audit) List(ctx context.Context, page, limit int) ([]model.Event, error) {
	return nil, nil
}
