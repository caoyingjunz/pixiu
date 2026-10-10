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
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type OAuthProviderInterface interface {
	List(ctx context.Context, opts ...Options) ([]*model.OAuthProvider, error)
	GetByProvider(ctx context.Context, provider string) (*model.OAuthProvider, error)
	Create(ctx context.Context, object *model.OAuthProvider) (*model.OAuthProvider, error)
	Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
}

type oauthProvider struct {
	db *gorm.DB
}

func newOAuthProvider(db *gorm.DB) OAuthProviderInterface {
	return &oauthProvider{db: db}
}

func (o *oauthProvider) GetByProvider(ctx context.Context, provider string) (*model.OAuthProvider, error) {
	var object model.OAuthProvider
	if err := o.db.WithContext(ctx).Where("provider = ?", provider).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (o *oauthProvider) List(ctx context.Context, opts ...Options) ([]*model.OAuthProvider, error) {
	var objects []*model.OAuthProvider
	tx := o.db.WithContext(ctx).Order("id asc")
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

// Create 使用 map 写入：model 的 auto_create_user 带 default:true tag，
// GORM 在 Create 时会把结构体里的零值 false 替换为默认值 true（即使显式 Select 列也如此，
// 见 gorm callbacks/create.go 的 DefaultValueInterface 替换）。map 值会逐字落库，
// 可保证「关闭自动建号」时 auto_create_user=false 真实写入。
func (o *oauthProvider) Create(ctx context.Context, object *model.OAuthProvider) (*model.OAuthProvider, error) {
	now := time.Now()
	row := map[string]interface{}{
		"provider":         object.Provider,
		"name":             object.Name,
		"login_type":       object.LoginType,
		"enabled":          object.Enabled,
		"app_id":           object.AppID,
		"app_secret":       object.AppSecret,
		"redirect_uri":     object.RedirectURI,
		"scopes":           object.Scopes,
		"config_json":      object.ConfigJSON,
		"auto_create_user": object.AutoCreateUser,
		"match_email":      object.MatchEmail,
		"description":      object.Description,
		"resource_version": 0,
		"gmt_create":       now,
		"gmt_modified":     now,
	}
	if err := o.db.WithContext(ctx).Model(&model.OAuthProvider{}).Create(row).Error; err != nil {
		return nil, err
	}
	return o.GetByProvider(ctx, object.Provider)
}

func (o *oauthProvider) Update(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := o.db.WithContext(ctx).Model(&model.OAuthProvider{}).
		Where("id = ? and resource_version = ?", id, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotFound
	}
	return nil
}
