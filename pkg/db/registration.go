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
	"crypto/subtle"
	goerrors "errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

var (
	ErrRegistrationCodeTooFrequent = goerrors.New("registration code sent too frequently")
	ErrRegistrationCodeInvalid     = goerrors.New("invalid registration code")
	ErrRegistrationCodeExpired     = goerrors.New("registration code expired")
	ErrRegistrationCodeUsed        = goerrors.New("registration code already used")
	ErrRegistrationCodeAttempts    = goerrors.New("too many registration code attempts")
	ErrRegistrationEmailExists     = goerrors.New("registration email already exists")
	ErrRegistrationUserExists      = goerrors.New("registration user already exists")
	ErrRegistrationRoleUnavailable = goerrors.New("registration role unavailable")
	ErrRegistrationRoleConflict    = goerrors.New("multiple registration roles found")
)

const registrationRoleName = "普通用户"

type RegistrationInterface interface {
	StoreCode(ctx context.Context, object *model.RegistrationCode, cooldown time.Duration) error
	InvalidateCode(ctx context.Context, email, codeHash string) error
	Register(ctx context.Context, email, codeHash string, maxAttempts int, user *model.User) error
}

type registration struct {
	db *gorm.DB
}

func (r *registration) StoreCode(ctx context.Context, object *model.RegistrationCode, cooldown time.Duration) error {
	now := object.SentAt
	if now.IsZero() {
		now = time.Now()
		object.SentAt = now
	}
	object.GmtCreate = now
	object.GmtModified = now

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.RegistrationCode
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("email = ?", object.Email).First(&current).Error
		if err != nil && !utilerrors.IsRecordNotFound(err) {
			return err
		}
		if err == nil {
			if current.SentAt.Add(cooldown).After(now) {
				return ErrRegistrationCodeTooFrequent
			}
			return tx.Model(&model.RegistrationCode{}).Where("id = ?", current.Id).Updates(map[string]interface{}{
				"code_hash":        object.CodeHash,
				"expires_at":       object.ExpiresAt,
				"used_at":          nil,
				"failed_attempts":  0,
				"sent_at":          object.SentAt,
				"request_ip":       object.RequestIP,
				"gmt_modified":     now,
				"resource_version": gorm.Expr("resource_version + 1"),
			}).Error
		}
		return tx.Create(object).Error
	})
	if utilerrors.IsUniqueConstraintError(err) {
		return ErrRegistrationCodeTooFrequent
	}
	return err
}

// InvalidateCode 仅失效本次发送的验证码，避免并发发送时误伤更新后的验证码。
func (r *registration) InvalidateCode(ctx context.Context, email, codeHash string) error {
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

// Register 在同一事务中校验并消费验证码、检查账号冲突、创建普通用户。
func (r *registration) Register(ctx context.Context, email, codeHash string, maxAttempts int, user *model.User) error {
	var businessErr error
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		var code model.RegistrationCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("email = ?", email).First(&code).Error; err != nil {
			if utilerrors.IsRecordNotFound(err) {
				businessErr = ErrRegistrationCodeInvalid
				return nil
			}
			return err
		}
		if code.UsedAt != nil {
			businessErr = ErrRegistrationCodeUsed
			return nil
		}
		if !now.Before(code.ExpiresAt) {
			businessErr = ErrRegistrationCodeExpired
			return nil
		}
		if code.FailedAttempts >= maxAttempts {
			businessErr = ErrRegistrationCodeAttempts
			return nil
		}
		if subtle.ConstantTimeCompare([]byte(code.CodeHash), []byte(codeHash)) != 1 {
			attempts := code.FailedAttempts + 1
			if err := tx.Model(&model.RegistrationCode{}).Where("id = ?", code.Id).Updates(map[string]interface{}{
				"failed_attempts":  attempts,
				"gmt_modified":     now,
				"resource_version": gorm.Expr("resource_version + 1"),
			}).Error; err != nil {
				return err
			}
			if attempts >= maxAttempts {
				businessErr = ErrRegistrationCodeAttempts
			} else {
				businessErr = ErrRegistrationCodeInvalid
			}
			return nil
		}

		var roles []model.Role
		if err := tx.Where("name = ?", registrationRoleName).Limit(2).Find(&roles).Error; err != nil {
			return err
		}
		switch len(roles) {
		case 0:
			businessErr = ErrRegistrationRoleUnavailable
			return nil
		case 1:
			user.TenantId = roles[0].TenantId
			user.Role = model.UserLevel(roles[0].Id)
		default:
			businessErr = ErrRegistrationRoleConflict
			return nil
		}

		var count int64
		if err := tx.Model(&model.User{}).Where("name = ?", user.Name).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			businessErr = ErrRegistrationUserExists
			return nil
		}
		if err := tx.Model(&model.User{}).Where("LOWER(email) = ?", email).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			businessErr = ErrRegistrationEmailExists
			return nil
		}

		user.GmtCreate = now
		user.GmtModified = now
		if err := tx.Create(user).Error; err != nil {
			if utilerrors.IsUniqueConstraintError(err) {
				businessErr = ErrRegistrationUserExists
				return nil
			}
			return err
		}
		result := tx.Model(&model.RegistrationCode{}).
			Where("id = ? AND used_at IS NULL", code.Id).
			Updates(map[string]interface{}{
				"used_at":          now,
				"gmt_modified":     now,
				"resource_version": gorm.Expr("resource_version + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRegistrationCodeUsed
		}
		return nil
	})
	if err != nil {
		return err
	}
	return businessErr
}

func newRegistration(db *gorm.DB) *registration {
	return &registration{db: db}
}
