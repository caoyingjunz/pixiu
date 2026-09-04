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
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type RegistrationCodeInterface interface {
	Get(ctx context.Context, id int64) (*model.RegistrationCode, error)
	Create(ctx context.Context, object *model.RegistrationCode) error
	Update(ctx context.Context, id int64, updates map[string]interface{}) error

	GetBy(ctx context.Context, opts ...Options) (*model.RegistrationCode, error)
	UpdateBy(ctx context.Context, opts []Options, updates map[string]interface{}) (int64, error)
}

type registrationCode struct {
	db *gorm.DB
}

func (r *registrationCode) Get(ctx context.Context, id int64) (*model.RegistrationCode, error) {
	var object model.RegistrationCode
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (r *registrationCode) GetBy(ctx context.Context, opts ...Options) (*model.RegistrationCode, error) {
	var object model.RegistrationCode
	tx := r.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.First(&object).Error; err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (r *registrationCode) Create(ctx context.Context, object *model.RegistrationCode) error {
	now := time.Now()
	if object.SentAt.IsZero() {
		object.SentAt = now
	}
	object.GmtCreate = now
	object.GmtModified = now
	return r.db.WithContext(ctx).Create(object).Error
}

func (r *registrationCode) Update(ctx context.Context, id int64, updates map[string]interface{}) error {
	now := time.Now()
	updates["gmt_modified"] = now
	updates["resource_version"] = gorm.Expr("resource_version + 1")
	return r.db.WithContext(ctx).
		Model(&model.RegistrationCode{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *registrationCode) UpdateBy(ctx context.Context, opts []Options, updates map[string]interface{}) (int64, error) {
	now := time.Now()
	updates["gmt_modified"] = now
	updates["resource_version"] = gorm.Expr("resource_version + 1")

	tx := r.db.WithContext(ctx).Model(&model.RegistrationCode{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	result := tx.Updates(updates)
	return result.RowsAffected, result.Error
}

func newRegistrationCode(db *gorm.DB) *registrationCode {
	return &registrationCode{db: db}
}
