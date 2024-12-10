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

type AuditInterface interface {
	List(ctx context.Context, opts ...Options) ([]model.Audit, error)
	Get(ctx context.Context, id int64) (*model.Audit, error)
	Create(ctx context.Context, object *model.Audit) (*model.Audit, error)
	BatchDelete(ctx context.Context, opts ...Options) (int64, error)

	Count(ctx context.Context, opts ...Options) (int64, error)
}

type audit struct {
	db *gorm.DB
}

func newAudit(db *gorm.DB) AuditInterface {
	return &audit{db: db}
}

func (a *audit) Create(ctx context.Context, object *model.Audit) (*model.Audit, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *audit) Get(ctx context.Context, aid int64) (*model.Audit, error) {
	var audit *model.Audit
	if err := a.db.WithContext(ctx).Where("id = ?", aid).First(audit).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return audit, nil
}

func (a *audit) List(ctx context.Context, opts ...Options) ([]model.Audit, error) {
	var audits []model.Audit
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&audits).Error; err != nil {
		return nil, err
	}

	return audits, nil
}

func (a *audit) BatchDelete(ctx context.Context, opts ...Options) (int64, error) {
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}

	err := tx.Delete(&model.Audit{}).Error
	return tx.RowsAffected, err
}

func (a *audit) Count(ctx context.Context, opts ...Options) (int64, error) {
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}

	var total int64
	err := tx.Model(&model.Audit{}).Count(&total).Error
	return total, err
}
