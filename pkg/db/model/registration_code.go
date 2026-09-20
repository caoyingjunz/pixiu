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

import (
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(&RegistrationCode{})
}

// RegistrationCode 保存邮箱注册验证码的服务端状态。验证码仅保存 HMAC 摘要。
type RegistrationCode struct {
	pixiu.Model

	Email          string     `gorm:"column:email;type:varchar(128);not null;uniqueIndex:uk_registration_code_email" json:"email"`
	CodeHash       string     `gorm:"column:code_hash;type:char(64);not null" json:"-"`
	ExpiresAt      time.Time  `gorm:"column:expires_at;type:datetime;not null;index:idx_registration_code_expires" json:"expires_at"`
	UsedAt         *time.Time `gorm:"column:used_at;type:datetime" json:"used_at,omitempty"`
	FailedAttempts int        `gorm:"column:failed_attempts;not null;default:0" json:"failed_attempts"`
	SentAt         time.Time  `gorm:"column:sent_at;type:datetime;not null" json:"sent_at"`
	RequestIP      string     `gorm:"column:request_ip;type:varchar(64)" json:"request_ip"`
}

func (*RegistrationCode) TableName() string {
	return "registration_codes"
}
