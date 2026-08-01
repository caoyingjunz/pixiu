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

type ClusterAccessTokenInterface interface {
	Create(ctx context.Context, object *model.ClusterAccessToken) (*model.ClusterAccessToken, error)
	GetByJTI(ctx context.Context, jti string) (*model.ClusterAccessToken, error)
	GetByTokenHash(ctx context.Context, hash string) (*model.ClusterAccessToken, error)
	RevokeByJTI(ctx context.Context, jti string, userId int64) error
	TouchLastUsed(ctx context.Context, id int64) error
	List(ctx context.Context, opts ...Options) ([]model.ClusterAccessToken, error)
}

type clusterAccessToken struct {
	db *gorm.DB
}

func newClusterAccessToken(db *gorm.DB) ClusterAccessTokenInterface {
	return &clusterAccessToken{db: db}
}

func (a *clusterAccessToken) Create(ctx context.Context, object *model.ClusterAccessToken) (*model.ClusterAccessToken, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *clusterAccessToken) GetByJTI(ctx context.Context, jti string) (*model.ClusterAccessToken, error) {
	var object model.ClusterAccessToken
	if err := a.db.WithContext(ctx).Where("jti = ?", jti).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *clusterAccessToken) GetByTokenHash(ctx context.Context, hash string) (*model.ClusterAccessToken, error) {
	var object model.ClusterAccessToken
	if err := a.db.WithContext(ctx).Where("token_hash = ?", hash).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *clusterAccessToken) RevokeByJTI(ctx context.Context, jti string, userId int64) error {
	now := time.Now()
	f := a.db.WithContext(ctx).Model(&model.ClusterAccessToken{}).
		Where("jti = ? AND user_id = ? AND revoked_at IS NULL", jti, userId).
		Updates(map[string]interface{}{
			"revoked_at":   now,
			"gmt_modified": now,
		})
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (a *clusterAccessToken) TouchLastUsed(ctx context.Context, id int64) error {
	now := time.Now()
	return a.db.WithContext(ctx).Model(&model.ClusterAccessToken{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_used_at": now,
			"gmt_modified": now,
		}).Error
}

func (a *clusterAccessToken) List(ctx context.Context, opts ...Options) ([]model.ClusterAccessToken, error) {
	var objects []model.ClusterAccessToken
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}
