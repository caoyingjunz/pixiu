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
	"gorm.io/gorm/clause"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

// AuthInterface 注册验证码持久化接口（不含业务编排）。
type AuthInterface interface {
	GetCodeByEmailForUpdate(ctx context.Context, email string) (*model.RegistrationCode, error)
	CreateCode(ctx context.Context, object *model.RegistrationCode) error
	UpdateCode(ctx context.Context, id int64, updates map[string]interface{}) error
	InvalidateCode(ctx context.Context, email, codeHash string) error
	MarkCodeUsed(ctx context.Context, id int64) error
}

type authStore struct {
	db *gorm.DB
}

func (r *authStore) GetCodeByEmailForUpdate(ctx context.Context, email string) (*model.RegistrationCode, error) {
	var object model.RegistrationCode
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("email = ?", email).
		First(&object).Error
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (r *authStore) CreateCode(ctx context.Context, object *model.RegistrationCode) error {
	now := time.Now()
	if object.SentAt.IsZero() {
		object.SentAt = now
	}
	object.GmtCreate = now
	object.GmtModified = now
	return r.db.WithContext(ctx).Create(object).Error
}

func (r *authStore) UpdateCode(ctx context.Context, id int64, updates map[string]interface{}) error {
	now := time.Now()
	updates["gmt_modified"] = now
	updates["resource_version"] = gorm.Expr("resource_version + 1")
	return r.db.WithContext(ctx).
		Model(&model.RegistrationCode{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// InvalidateCode 按 email 与 code_hash 精确失效，避免并发发送时误伤新验证码。
func (r *authStore) InvalidateCode(ctx context.Context, email, codeHash string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.RegistrationCode{}).
		Where("email = ? AND code_hash = ?", email, codeHash).
		Updates(map[string]interface{}{
			"expires_at":       now,
			"used_at":          now,
			"sent_at":          now.Add(-time.Hour),
			"gmt_modified":     now,
			"resource_version": gorm.Expr("resource_version + 1"),
		}).Error
}

func (r *authStore) MarkCodeUsed(ctx context.Context, id int64) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&model.RegistrationCode{}).
		Where("id = ? AND used_at IS NULL", id).
		Updates(map[string]interface{}{
			"used_at":          now,
			"gmt_modified":     now,
			"resource_version": gorm.Expr("resource_version + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrRegistrationCodeNotMarked
	}
	return nil
}

func newAuth(db *gorm.DB) *authStore {
	return &authStore{db: db}
}
