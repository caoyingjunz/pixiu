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
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

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
func ValidateUserPassword(old, new string) error {
	return bcrypt.CompareHashAndPassword([]byte(old), []byte(new))
}

// ValidateStrongPassword validates the password is strong enough.
func ValidateStrongPassword(password string) bool {
	if len(password) < 8 {
		return false
	}

	var oneUpper bool
	var oneLower bool
	var oneNumber bool
	for _, l := range password {
		if oneUpper && oneLower && oneNumber {
			return true
		}

		if !oneUpper && l >= 'A' && l <= 'Z' {
			oneUpper = true
			continue
		}
		if !oneLower && l >= 'a' && l <= 'z' {
			oneLower = true
			continue
		}
		if !oneNumber && l >= '0' && l <= '9' {
			oneNumber = true
			continue
		}
	}
	return oneUpper && oneLower && oneNumber
}

// GenerateRequestID return a request ID string with random suffix.
func GenerateRequestID() string {
	return fmt.Sprintf("%s-%06d", time.Now().Format("20060102150405"), rand.Intn(1000000))
}

func IsEmptyS(s string) bool {
	return len(s) != 0
}

func IsDirectoryExists(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}

	if stat.IsDir() {
		return true
	}
	return false
}

func IsFileExists(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}

	if stat.IsDir() {
		return false
	}
	return true
}

func EnsureDirectoryExists(path string) error {
	if !IsDirectoryExists(path) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}

	return nil
}

func WriteToFile(filename string, data []byte) error {
	return ioutil.WriteFile(filename, data, 0644)
}
