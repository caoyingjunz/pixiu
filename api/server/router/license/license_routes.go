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

package license

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/license"
)

// ErrLicensePathNotConfigured 服务端未配置 license 存储路径
var ErrLicensePathNotConfigured = errors.New("服务端未配置 license 存储路径（config.yaml license.path）")

// updateLicenseRequest 上传 license 请求体
type updateLicenseRequest struct {
	// Content license.key 文件内容（文本）
	Content string `json:"content" validate:"required"`
}

// get 返回当前 license 概要（状态/版本/功能/配额/过期时间）
func (r *licenseRouter) get(c *gin.Context) {
	result := httputils.NewResponse()
	result.Result = license.Get().Summary()
	httputils.SetSuccess(c, result)
}

// update 上传并立即生效新的 license（仅超级管理员）
func (r *licenseRouter) update(c *gin.Context) {
	result := httputils.NewResponse()
	var req updateLicenseRequest
	var err error
	if err = httputils.ShouldBindAny(c, &req, nil, nil); err == nil {
		// 仅超级管理员可更新 license
		if _, err = httputils.CheckRootUser(c.Request.Context()); err == nil {
			err = r.saveAndReload(req.Content)
		}
	}
	if err != nil {
		httputils.SetFailed(c, result, err)
		return
	}
	result.Result = license.Get().Summary()
	httputils.SetSuccess(c, result)
}

// saveAndReload 预校验新 license 后原子写出并热加载。
// 先验签再落盘：非法内容不会覆盖现有文件，无需回滚；写入用临时文件 + fsync + rename，
// 避免进程崩溃时留下截断的 license 文件。
func (r *licenseRouter) saveAndReload(content string) error {
	path := r.path
	if path == "" {
		return ErrLicensePathNotConfigured
	}
	data := []byte(content)

	// 落盘前先验签 + 校验有效期/指纹，非法内容直接拒绝，不动现有文件
	if err := license.VerifyContent(data); err != nil {
		return fmt.Errorf("license 校验失败: %w", err)
	}
	if err := writeFileAtomic(path, data, 0o644); err != nil {
		return err
	}
	return license.Get().Reload()
}

// writeFileAtomic 原子写文件：同目录临时文件写满并 fsync 后 rename 覆盖目标。
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".license-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后路径不存在，remove 无副作用

	if err = tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmpName, path); err != nil {
		return err
	}
	// 尽力 fsync 目录，确保 rename 持久化（部分平台不支持时忽略）
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
