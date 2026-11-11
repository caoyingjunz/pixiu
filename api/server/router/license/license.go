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

// Package license 提供 license 查询与更新 API：
//
//	GET  /pixiu/license          当前 license 概要（所有登录用户可读，前端门控依赖）
//	POST /pixiu/license          上传/更新 license 文件（仅超级管理员）
package license

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
)

const licenseURL = "/pixiu/license"

type licenseRouter struct {
	// path license 文件存储路径（来自 config.yaml license.path）
	path string
}

func NewRouter(o *options.Options) {
	router := &licenseRouter{path: o.ComponentConfig.License.Path}
	router.initRoutes(o.HttpEngine)
}

func (r *licenseRouter) initRoutes(engine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "商业授权",
		BaseURL: licenseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "GET", RelativePath: "", Handler: r.get, Description: "获取当前 license 概要", Persist: boolPtr(false)},
			{Method: "POST", RelativePath: "", Handler: r.update, Description: "上传/更新 license 文件", Persist: boolPtr(false)},
		},
	}
	// license 路由不持久化到 API 权限目录：GET 需要所有登录用户可达（前端门控依赖），
	// POST 已单独做超管校验，避免影响常规 RBAC 流程
	group.Register(engine.Group(licenseURL), nil)
}

func boolPtr(v bool) *bool { return &v }
