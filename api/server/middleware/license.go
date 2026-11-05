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

package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/license"
)

// featurePrefixes 付费功能 → API URL 前缀映射。
// 新增付费模块时在此登记前缀，中间件即自动生效。
var featurePrefixes = []struct {
	feature  license.Feature
	prefixes []string
}{
	{license.FeatureBackup, []string{"/pixiu/backup"}},
	{license.FeatureAlert, []string{"/pixiu/alerts"}},
	{license.FeatureExtension, []string{"/pixiu/extension"}},
	{license.FeatureDistribution, []string{"/pixiu/distributions"}},
	{license.FeatureApps, []string{"/pixiu/apps"}},
	{license.FeatureAssistant, []string{"/pixiu/assistant"}},
}

// License 商业授权中间件：
// 命中付费功能 URL 前缀时校验 license 授权，未授权返回 403（前端据此弹升级引导）。
// debug 模式全放通，与认证中间件行为保持一致。
func License(o *options.Options) gin.HandlerFunc {
	return func(c *gin.Context) {
		if o.ComponentConfig.Default.Mode.InDebug() {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		for _, item := range featurePrefixes {
			if !matchPrefix(path, item.prefixes) {
				continue
			}
			if !license.Get().HasFeature(item.feature) {
				httputils.AbortFailedWithCode(c, http.StatusForbidden,
					license.ErrLicenseRequired)
				return
			}
			break
		}
		c.Next()
	}
}

// matchPrefix 判断请求路径是否命中任一前缀
func matchPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
