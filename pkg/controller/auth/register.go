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

package auth

import (
	"context"
	"crypto/subtle"
	goerrors "errors"
	"time"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

func registrationCodeLockOpts(email string) []db.Options {
	return []db.Options{db.WithEmail(email), db.WithForUpdate()}
}

func (c *controller) issueRegistrationCode(ctx context.Context, factory db.ShareDaoFactory, object *model.RegistrationCode, cooldown time.Duration) error {
	now := object.SentAt
	if now.IsZero() {
		now = time.Now()
		object.SentAt = now
	}

	current, err := factory.RegistrationCode().GetBy(ctx, registrationCodeLockOpts(object.Email)...)
	if err != nil {
		return err
	}
	if current != nil {
		if current.SentAt.Add(cooldown).After(now) {
			return errCodeTooFrequent
		}
		return factory.RegistrationCode().Update(ctx, current.Id, map[string]interface{}{
			"code_hash":       object.CodeHash,
			"expires_at":      object.ExpiresAt,
			"used_at":         nil,
			"failed_attempts": 0,
			"sent_at":         object.SentAt,
			"request_ip":      object.RequestIP,
		})
	}

	if err = factory.RegistrationCode().Create(ctx, object); err != nil {
		if utilerrors.IsUniqueConstraintError(err) {
			return errCodeTooFrequent
		}
		return err
	}
	return nil
}

// expireUnsentRegistrationCode 作废未发送成功的验证码：置为已过期/已使用并回拨发送时间，
// 既杜绝该验证码被使用，也释放冷却窗口允许用户立即重试。
func (c *controller) expireUnsentRegistrationCode(ctx context.Context, factory db.ShareDaoFactory, email, codeHash string) error {
	now := time.Now()
	_, err := factory.RegistrationCode().UpdateBy(ctx, []db.Options{
		db.WithEmail(email),
		db.WithCodeHash(codeHash),
	}, map[string]interface{}{
		"expires_at": now,
		"used_at":    now,
		"sent_at":    now.Add(-time.Hour),
	})
	return err
}

func (c *controller) registerUser(ctx context.Context, factory db.ShareDaoFactory, email, codeHash string, user *model.User) error {
	now := time.Now()

	code, err := factory.RegistrationCode().GetBy(ctx, registrationCodeLockOpts(email)...)
	if err != nil {
		return err
	}
	if code == nil {
		return errCodeInvalid
	}
	if code.UsedAt != nil {
		return errCodeUsed
	}
	if !now.Before(code.ExpiresAt) {
		return errCodeExpired
	}
	if code.FailedAttempts >= maxCodeAttempts {
		return errCodeAttempts
	}
	if subtle.ConstantTimeCompare([]byte(code.CodeHash), []byte(codeHash)) != 1 {
		attempts := code.FailedAttempts + 1
		if err = factory.RegistrationCode().Update(ctx, code.Id, map[string]interface{}{"failed_attempts": attempts}); err != nil {
			return err
		}
		if attempts >= maxCodeAttempts {
			return errCodeAttempts
		}
		return errCodeInvalid
	}

	if existing, err := factory.User().GetBy(ctx, db.WithName(user.Name)); err != nil {
		return err
	} else if existing != nil {
		return errUserExists
	}

	if existing, err := factory.User().GetBy(ctx, db.WithEmail(email)); err != nil {
		return err
	} else if existing != nil {
		return errEmailExists
	}

	role, err := resolveRegistrationRole(ctx, factory)
	if err != nil {
		return err
	}

	user.TenantId = role.TenantId
	user.Role = model.UserLevel(role.Id)
	user.GmtCreate = now
	user.GmtModified = now
	if _, err = factory.User().Create(ctx, user); err != nil {
		if utilerrors.IsUniqueConstraintError(err) {
			return errUserExists
		}
		return err
	}

	rows, err := factory.RegistrationCode().UpdateBy(ctx, []db.Options{
		db.WithId(code.Id),
		db.WithNullUsedAt(),
	}, map[string]interface{}{
		"used_at": now,
	})
	if err != nil {
		return err
	}
	if rows != 1 {
		return errCodeUsed
	}
	return nil
}

func resolveRegistrationRole(ctx context.Context, factory db.ShareDaoFactory) (*model.Role, error) {
	roles, err := factory.Role().List(ctx, db.WithName(model.DefaultRoleName))
	if err != nil {
		return nil, err
	}
	switch len(roles) {
	case 0:
		return nil, errRoleUnavailable
	case 1:
		return &roles[0], nil
	default:
		return nil, errRoleConflict
	}
}

func mapRegistrationError(err error) error {
	switch {
	case goerrors.Is(err, errCodeTooFrequent):
		return apierrors.ErrRegistrationCodeTooFrequent
	case goerrors.Is(err, errCodeInvalid):
		return apierrors.ErrRegistrationCodeInvalid
	case goerrors.Is(err, errCodeExpired):
		return apierrors.ErrRegistrationCodeExpired
	case goerrors.Is(err, errCodeUsed):
		return apierrors.ErrRegistrationCodeUsed
	case goerrors.Is(err, errCodeAttempts):
		return apierrors.ErrRegistrationCodeAttempts
	case goerrors.Is(err, errEmailExists):
		return apierrors.ErrEmailExists
	case goerrors.Is(err, errUserExists):
		return apierrors.ErrUserExists
	case goerrors.Is(err, errRoleUnavailable):
		return apierrors.ErrRegistrationRoleUnavailable
	case goerrors.Is(err, errRoleConflict):
		return apierrors.ErrRegistrationRoleConflict
	default:
		return err
	}
}
