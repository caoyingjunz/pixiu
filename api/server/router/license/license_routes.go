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

// saveAndReload 原子写出 license 文件并热加载；新 license 验签失败时回滚旧文件
func (r *licenseRouter) saveAndReload(content string) error {
	path := r.path
	if path == "" {
		return ErrLicensePathNotConfigured
	}
	old, _ := os.ReadFile(path)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	if err := license.Get().Reload(); err != nil {
		// 新 license 非法，回滚旧文件，保持原授权状态
		if len(old) > 0 {
			_ = os.WriteFile(path, old, 0o644)
			_ = license.Get().Reload()
		}
		return err
	}
	return nil
}
