/*
Copyright 2021 The Pixiu Authors.

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

package util

import (
	"golang.org/x/crypto/bcrypt"
)

// EncryptUserPassword 生成加密密码
// 前端传的密码为明文，需要加密存储
// TODO: 后续确认是否有必要在前端加密
func EncryptUserPassword(origin string) (string, error) {
	pwd, err := bcrypt.GenerateFromPassword([]byte(origin), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(pwd), nil
}

// ValidateUserPassword 验证用户的密码是否正确
func ValidateUserPassword(new, old string) error {
	return bcrypt.CompareHashAndPassword([]byte(new), []byte(old))
}
