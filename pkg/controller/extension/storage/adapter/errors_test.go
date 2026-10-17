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
	"testing"

	"github.com/minio/minio-go/v7"
)

// 错误归一化三态：404 + 历史空态码 → 未配置；404 + NoSuchBucket → 真实错误；403 → 真实错误。
func TestAbsentCodesNormalization(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		codes []string
		want  bool
	}{
		{
			name:  "404 且命中空态码视为未配置",
			err:   wrapError(minio.ErrorResponse{Code: "NoSuchBucketPolicy", StatusCode: http.StatusNotFound, Message: "no policy"}),
			codes: []string{"NoSuchBucketPolicy"},
			want:  true,
		},
		{
			name:  "404 且 NoSuchBucket 是真实错误",
			err:   wrapError(minio.ErrorResponse{Code: "NoSuchBucket", StatusCode: http.StatusNotFound}),
			codes: []string{"NoSuchBucketPolicy"},
			want:  false,
		},
		{
			name:  "403 是真实错误",
			err:   wrapError(minio.ErrorResponse{Code: "AccessDenied", StatusCode: http.StatusForbidden, Message: "Access Denied."}),
			codes: []string{"NoSuchBucketPolicy"},
			want:  false,
		},
		{
			name:  "404 未知码按新规则归一化为未配置",
			err:   wrapError(minio.ErrorResponse{StatusCode: http.StatusNotFound}),
			codes: []string{"NoSuchBucketPolicy"},
			want:  true,
		},
		{
			name:  "非 404 但命中历史空态码（兼容非常规实现）",
			err:   wrapError(minio.ErrorResponse{Code: "NoSuchTagSet", StatusCode: http.StatusOK}),
			codes: []string{"NoSuchTagSet"},
			want:  true,
		},
		{
			name:  "无错误不算未配置",
			err:   nil,
			codes: []string{"NoSuchTagSet"},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := absentCodes(tt.err, tt.codes...); got != tt.want {
				t.Errorf("absentCodes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	notFoundPolicy := wrapError(minio.ErrorResponse{Code: "NoSuchBucketPolicy", StatusCode: http.StatusNotFound})
	if !IsNotFound(notFoundPolicy) {
		t.Error("404+NoSuchBucketPolicy 应为 not found（未配置）")
	}
	notFoundBucket := wrapError(minio.ErrorResponse{Code: "NoSuchBucket", StatusCode: http.StatusNotFound})
	if IsNotFound(notFoundBucket) {
		t.Error("404+NoSuchBucket 不应为 not found")
	}
	forbidden := wrapError(minio.ErrorResponse{Code: "AccessDenied", StatusCode: http.StatusForbidden})
	if IsNotFound(forbidden) {
		t.Error("403 不应为 not found")
	}
	if IsNotFound(nil) || IsNotFound(errors.New("plain")) {
		t.Error("nil / 普通错误不应为 not found")
	}
}

func TestErrorCode(t *testing.T) {
	err := wrapError(minio.ErrorResponse{Code: "NoSuchBucket", StatusCode: http.StatusNotFound})
	if got := ErrorCode(err); got != "NoSuchBucket" {
		t.Errorf("ErrorCode() = %q, want NoSuchBucket", got)
	}
	// 经 fmt.Errorf("%w") 包装后仍可提取
	if got := ErrorCode(fmt.Errorf("get bucket policy failed: %w", err)); got != "NoSuchBucket" {
		t.Errorf("包装后 ErrorCode() = %q, want NoSuchBucket", got)
	}
	if got := ErrorCode(nil); got != "" {
		t.Errorf("nil 的 ErrorCode() = %q, want 空串", got)
	}
	if got := ErrorCode(errors.New("plain")); got != "" {
		t.Errorf("普通错误的 ErrorCode() = %q, want 空串", got)
	}
}

// StorageError 的文本委托原始错误：经 %w 包装后的文案与改造前直接使用 SDK 错误时逐字一致。
func TestStorageErrorTextDelegation(t *testing.T) {
	inner := minio.ErrorResponse{Code: "NoSuchBucket", StatusCode: http.StatusNotFound, Message: "The specified bucket does not exist"}
	err := wrapError(inner)
	if err.Error() != inner.Error() {
		t.Fatalf("StorageError.Error() = %q, want %q", err.Error(), inner.Error())
	}
	wrapped := fmt.Errorf("get bucket policy failed: %w", err)
	want := "get bucket policy failed: " + inner.Error()
	if wrapped.Error() != want {
		t.Fatalf("包装后错误文案 = %q, want %q", wrapped.Error(), want)
	}
	// errors.Is/As 链完整：可穿透包装定位到底层 SDK 错误
	var er minio.ErrorResponse
	if !errors.As(wrapped, &er) || er.Code != "NoSuchBucket" {
		t.Fatalf("errors.As 未能穿透包装定位 minio.ErrorResponse: %+v", er)
	}
}

// 本地判定错误（如标签校验）带显式 Message 时优先输出 Message，Code 供控制层做 400 映射。
func TestStorageErrorLocalCode(t *testing.T) {
	err := &StorageError{Code: CodeInvalidTag, Status: http.StatusBadRequest, Err: errors.New("invalid tag key")}
	if got := ErrorCode(err); got != CodeInvalidTag {
		t.Errorf("ErrorCode() = %q, want %q", got, CodeInvalidTag)
	}
	if err.Error() != "invalid tag key" {
		t.Errorf("Error() = %q, want 原始错误文本", err.Error())
	}
	if IsNotFound(err) {
		t.Error("400 不应为 not found")
	}
}
