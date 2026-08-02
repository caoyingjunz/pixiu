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
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type AccessTokenInterface interface {
	Create(ctx context.Context, object *model.ClusterAccessToken) (*model.ClusterAccessToken, error)
	List(ctx context.Context, opts ...Options) ([]model.ClusterAccessToken, error)
	Delete(ctx context.Context, opts ...Options) error

	GetBy(ctx context.Context, opts ...Options) (*model.ClusterAccessToken, error)
	InternalUpdate(ctx context.Context, id int64, updates map[string]interface{}) error
	TouchLastUsed(ctx context.Context, id int64) error
}

type accessToken struct {
	db *gorm.DB
}

func newAccessToken(db *gorm.DB) AccessTokenInterface {
	return &accessToken{db: db}
}

func (a *accessToken) Create(ctx context.Context, object *model.ClusterAccessToken) (*model.ClusterAccessToken, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *accessToken) GetBy(ctx context.Context, opts ...Options) (*model.ClusterAccessToken, error) {
	var object model.ClusterAccessToken
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (a *accessToken) Delete(ctx context.Context, opts ...Options) error {
	if len(opts) == 0 {
		return fmt.Errorf("delete access token requires at least one filter option")
	}
	tx := a.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	f := tx.Delete(&model.ClusterAccessToken{})
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}

func (a *accessToken) TouchLastUsed(ctx context.Context, id int64) error {
	now := time.Now()
	return a.db.WithContext(ctx).Model(&model.ClusterAccessToken{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_used_at": now,
			"gmt_modified": now,
		}).Error
}

func (a *accessToken) List(ctx context.Context, opts ...Options) ([]model.ClusterAccessToken, error) {
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

func (a *accessToken) InternalUpdate(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	return a.db.WithContext(ctx).Model(&model.ClusterAccessToken{}).Where("id = ?", id).Updates(updates).Error
}
