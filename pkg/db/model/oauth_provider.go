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

package model

import "github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"

func init() {
	register(&OAuthProvider{})
}

type OAuthProvider struct {
	pixiu.Model

	Provider       string    `gorm:"type:varchar(32);not null;uniqueIndex:uk_oauth_provider" json:"provider"`
	Name           string    `gorm:"type:varchar(64)" json:"name"`
	LoginType      string    `gorm:"column:login_type;type:varchar(32)" json:"login_type"`
	Enabled        bool      `gorm:"column:enabled" json:"enabled"`
	AppID          string    `gorm:"column:app_id;type:varchar(128)" json:"app_id"`
	AppSecret      string    `gorm:"column:app_secret;type:varchar(256)" json:"-"`
	RedirectURI    string    `gorm:"column:redirect_uri;type:varchar(512)" json:"redirect_uri"`
	Scopes         string    `gorm:"type:varchar(512)" json:"scopes"`
	ConfigJSON     string    `gorm:"column:config_json;type:text" json:"config_json"`
	AutoCreateUser bool      `gorm:"column:auto_create_user" json:"auto_create_user"`
	DefaultRole    UserLevel `gorm:"column:default_role" json:"default_role"`
	MatchEmail     bool      `gorm:"column:match_email" json:"match_email"`
	Description    string    `gorm:"type:text" json:"description"`
}

func (*OAuthProvider) TableName() string {
	return "oauth_providers"
}
