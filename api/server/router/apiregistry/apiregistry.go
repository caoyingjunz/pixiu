/*
Copyright 2024 The Pixiu Authors.

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

package apiregistry

import (
	"context"
	"k8s.io/klog/v2"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/pkg/controller/apiresource"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// RouteEntry 路由注册项
type RouteEntry struct {
	Method       string // GET/POST/PUT/DELETE
	RelativePath string // 相对路径，如 "" 或 "/:roleId"
	Handler      gin.HandlerFunc
	Description  string // 中文描述
	SubGroup     string // 次级分类，如 Deployment/Service 等 K8s 资源类型
	Persist      *bool  // 是否持久化到数据库，nil 视为 true（默认持久化），显式设为 false 则不持久化
}

func persistEntry(entry RouteEntry) bool {
	return entry.Persist == nil || *entry.Persist
}

// Group 路由组，对应一个业务分组
type Group struct {
	Name    string // 分组名称，如 "角色管理"
	BaseURL string // 基础路径，如 "/pixiu/roles"
	Entries []RouteEntry
}

// Register 注册 Gin 路由并同步 API 元数据到数据库
func (g *Group) Register(ginGroup *gin.RouterGroup, apiSvc apiresource.Interface) {
	for _, entry := range g.Entries {
		registerGinRoute(ginGroup, entry)
		g.persistAPI(apiSvc, entry)
	}
}

// RegisterAPIs 仅将 API 元数据同步到数据库，不注册 Gin 路由（适用于 K8s 透传等单一 Handler 场景）
func (g *Group) RegisterAPIs(apiSvc apiresource.Interface) {
	for _, entry := range g.Entries {
		g.persistAPI(apiSvc, entry)
	}
}

func (g *Group) persistAPI(apiSvc apiresource.Interface, entry RouteEntry) {
	if !persistEntry(entry) {
		return
	}
	if err := registerAPI(apiSvc, g.Name, g.BaseURL, entry); err != nil {
		klog.Warning("register api %s failed %v", g.BaseURL, err)
	}
}

func registerGinRoute(ginGroup *gin.RouterGroup, entry RouteEntry) {
	switch entry.Method {
	case "GET":
		ginGroup.GET(entry.RelativePath, entry.Handler)
	case "POST":
		ginGroup.POST(entry.RelativePath, entry.Handler)
	case "PUT":
		ginGroup.PUT(entry.RelativePath, entry.Handler)
	case "DELETE":
		ginGroup.DELETE(entry.RelativePath, entry.Handler)
	case "PATCH":
		ginGroup.PATCH(entry.RelativePath, entry.Handler)
	case "ANY":
		ginGroup.Any(entry.RelativePath, entry.Handler)
	}
}

func registerAPI(apiSvc apiresource.Interface, groupName, baseURL string, entry RouteEntry) error {
	fullPath := baseURL
	if entry.RelativePath != "" {
		fullPath = baseURL + entry.RelativePath
	}
	desc := entry.Description
	req := &types.CreateAPIRequest{
		Method:      entry.Method,
		Path:        fullPath,
		Group:       &groupName,
		Description: &desc,
	}
	if entry.SubGroup != "" {
		req.SubGroup = &entry.SubGroup
	}

	return apiSvc.Register(context.Background(), req)
}
