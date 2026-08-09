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
	register(&Email{})
}

// Email 系统邮件 SMTP 配置（支持多条，is_default 标记默认配置）。
// Password 明文落库，对外接口不回显明文（仅 PasswordSet）。
type Email struct {
	pixiu.Model
	Name        string `gorm:"column:name;type:varchar(128);not null" json:"name"`
	SmtpHost    string `gorm:"column:smtp_host;type:varchar(255);not null" json:"smtp_host"`
	SmtpPort    int    `gorm:"column:smtp_port;not null" json:"smtp_port"`
	Username    string `gorm:"column:username;type:varchar(255)" json:"username"`
	Password    string `gorm:"column:password;type:varchar(512)" json:"-"`
	FromEmail   string `gorm:"column:from_email;type:varchar(255);not null" json:"from_email"`
	FromName    string `gorm:"column:from_name;type:varchar(128)" json:"from_name"`
	Encryption  string `gorm:"column:encryption;type:varchar(16);default:'none'" json:"encryption"`
	Enabled     bool   `gorm:"column:enabled;default:true;not null" json:"enabled"`
	IsDefault   bool   `gorm:"column:is_default;default:false;not null" json:"is_default"`
	Description string `gorm:"column:description;type:text" json:"description"`
	CreatedBy   int64  `gorm:"column:created_by;not null" json:"created_by"`
}

func (*Email) TableName() string {
	return "emails"
}
