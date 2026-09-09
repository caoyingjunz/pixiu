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
	List(ctx context.Context) ([]*model.OAuthProvider, error)
	GetByProvider(ctx context.Context, provider string) (*model.OAuthProvider, error)
	Save(ctx context.Context, object *model.OAuthProvider) (*model.OAuthProvider, error)
}

type oauthProvider struct {
	db *gorm.DB
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

func (o *oauthProvider) List(ctx context.Context) ([]*model.OAuthProvider, error) {
	var objects []*model.OAuthProvider
	if err := o.db.WithContext(ctx).Order("id asc").Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (o *oauthProvider) Save(ctx context.Context, object *model.OAuthProvider) (*model.OAuthProvider, error) {
	now := time.Now()
	existing, err := o.GetByProvider(ctx, object.Provider)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		object.GmtCreate = now
		object.GmtModified = now
		if err := o.db.WithContext(ctx).Create(object).Error; err != nil {
			return nil, err
		}
		return object, nil
	}

	updates := map[string]interface{}{
		"name":             object.Name,
		"login_type":       object.LoginType,
		"enabled":          object.Enabled,
		"app_id":           object.AppID,
		"app_secret":       object.AppSecret,
		"redirect_uri":     object.RedirectURI,
		"scopes":           object.Scopes,
		"config_json":      object.ConfigJSON,
		"auto_create_user": object.AutoCreateUser,
		"default_role":     object.DefaultRole,
		"match_email":      object.MatchEmail,
		"description":      object.Description,
		"gmt_modified":     now,
		"resource_version": existing.ResourceVersion + 1,
	}
	if err := o.db.WithContext(ctx).Model(&model.OAuthProvider{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return o.GetByProvider(ctx, object.Provider)
}

func newOAuthProvider(db *gorm.DB) *oauthProvider {
	return &oauthProvider{db: db}
}
