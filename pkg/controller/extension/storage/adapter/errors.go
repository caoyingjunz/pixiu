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

package adapter

import (
	"errors"
	"fmt"
	"net/http"
)

// CodeInvalidTag 适配层本地可判定的「标签输入不合法」错误码（用户输入问题，控制层应映射为 400）。
const CodeInvalidTag = "InvalidTag"

// StorageError 是适配层的统一错误：保留原始错误与（服务端）错误码、HTTP 状态码。
// Error() 优先使用显式 Message，否则委托原始错误的文本——保证经 fmt.Errorf("%w") 包装后
// 的错误文案与改造前直接使用 SDK 错误时逐字一致。
type StorageError struct {
	Code    string // 服务端/适配层错误码
	Status  int    // HTTP 状态码（0 表示未知）
	Message string // 显式消息；为空时取 Err 的文本
	Err     error  // 原始错误（Unwrap 链）
}

func (e *StorageError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("storage error %s (status %d)", e.Code, e.Status)
}

// Unwrap 保留 errors.Is/As 链（含归一化判定所需的底层错误）。
func (e *StorageError) Unwrap() error { return e.Err }

// ErrorCode 返回错误链中 StorageError 的错误码；无则返回空串。
func ErrorCode(err error) string {
	code, _ := errorInfo(err)
	return code
}

// IsNotFound 判断错误是否为「服务端 404 且非 NoSuchBucket」——
// 即目标不存在（未配置）而非真实失败。
func IsNotFound(err error) bool {
	code, status := errorInfo(err)
	return status == http.StatusNotFound && code != "NoSuchBucket"
}

// errorInfo 提取错误链中的（错误码, HTTP 状态码）。
func errorInfo(err error) (string, int) {
	var se *StorageError
	if errors.As(err, &se) {
		return se.Code, se.Status
	}
	return "", 0
}

// absentCodes 判断错误是否属于「配置未设置」：
//   - 错误码命中调用方给出的历史空态码（部分 S3 兼容实现不用 404 返回它们）；或
//   - 服务端 404 且错误码不是 NoSuchBucket（桶本身不存在是真实错误）。
func absentCodes(err error, codes ...string) bool {
	code, status := errorInfo(err)
	for _, c := range codes {
		if code == c {
			return true
		}
	}
	return status == http.StatusNotFound && code != "NoSuchBucket"
}
