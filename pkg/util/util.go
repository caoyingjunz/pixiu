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
	"os"
	"strconv"
)

// ParseInt64 将字符串转换为 int64
func ParseInt64(s string) (int64, error) {
	if len(s) == 0 {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

// IsDirectoryExists 判断目录是否存在
func IsDirectoryExists(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

// IsFileExists 判断文件是否存在
func IsFileExists(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !stat.IsDir()
}

// EnsureDirectoryExists 不存在则创建指定目录
func EnsureDirectoryExists(path string) error {
	if !IsDirectoryExists(path) {
		return os.MkdirAll(path, 0755)
	}

	return nil
}
