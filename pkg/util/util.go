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
	"errors"
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

// FileExist 判断文件是否存在
func FileExist(file string) bool {
	stat, _ := os.Stat(file)
	if stat == nil {
		return false
	}
	return true
}

// CheckIsDir 判断文件类型是否是目录
func CheckIsDir(file string) (bool, error) {
	stat, _ := os.Stat(file)
	if stat == nil {
		return false, errors.New("文件不存在！")
	}
	return stat.IsDir(), nil
}

// CreateDir 创建指定目录
func CreateDir(file string) {
	err := os.MkdirAll(file, 0777)
	if err != nil {
		panic("目录创建异常" + err.Error())
	}
}
