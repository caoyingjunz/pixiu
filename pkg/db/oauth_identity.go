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

type OAuthIdentityInterface interface {
	GetBySubject(ctx context.Context, provider, subjectType, subject string) (*model.OAuthIdentity, error)
	Create(ctx context.Context, object *model.OAuthIdentity) error
	DeleteByUser(ctx context.Context, userId int64) error
}

type oauthIdentity struct {
	db *gorm.DB
}

// GetBySubject 按第三方标识精确查找（对齐仓库 noRecord 范式：未命中返回 nil, nil）。
func (o *oauthIdentity) GetBySubject(ctx context.Context, provider, subjectType, subject string) (*model.OAuthIdentity, error) {
	var object model.OAuthIdentity
	if err := o.db.WithContext(ctx).
		Where("provider = ? and subject_type = ? and subject = ?", provider, subjectType, subject).
		First(&object).Error; err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (o *oauthIdentity) Create(ctx context.Context, object *model.OAuthIdentity) error {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	// 直接用 Create 触发唯一索引冲突（而非 OnConflict DoNothing），以便建号事务据此回滚。
	return o.db.WithContext(ctx).Create(object).Error
}

func (o *oauthIdentity) DeleteByUser(ctx context.Context, userId int64) error {
	return o.db.WithContext(ctx).Where("user_id = ?", userId).Delete(&model.OAuthIdentity{}).Error
}

func newOAuthIdentity(db *gorm.DB) OAuthIdentityInterface {
	return &oauthIdentity{db: db}
}
