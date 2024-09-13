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
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
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
	return os.WriteFile(filename, data, 0600)
}

func BuildWebSocketConnection(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		}}
	upgrader.Subprotocols = []string{r.Header.Get("Sec-WebSocket-Protocol")}
	return upgrader.Upgrade(w, r, nil)
}

// DeduplicateIntSlice returns a new slice with duplicated elements removed.
func DeduplicateIntSlice(s []int64) (ret []int64) {
	ret = make([]int64, 0)
	m := make(map[int64]struct{})
	for _, v := range s {
		if _, ok := m[v]; ok {
			continue
		}
		m[v] = struct{}{}
		ret = append(ret, v)
	}

	return
}

// More returns the larger one.
func More(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Less returns the smaller one.
func Less(a, b int) int {
	if a < b {
		return a
	}
	return b
}
