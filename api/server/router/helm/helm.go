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

package helm

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const (
	helmBaseURL = "/pixiu/helms"
)

type helmRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	hr := &helmRouter{
		c: o.Controller,
	}
	hr.initRoutes(o.HttpEngine)
}

func (hr *helmRouter) initRoutes(httpEngine *gin.Engine) {
	persist := false
	group := &apiregistry.Group{
		Name:    "Helm管理",
		BaseURL: helmBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "/repositories", Handler: hr.createRepository, Description: "创建仓库", Persist: &persist},
			{Method: "PUT", RelativePath: "/repositories/:id", Handler: hr.updateRepository, Description: "更新仓库", Persist: &persist},
			{Method: "DELETE", RelativePath: "/repositories/:id", Handler: hr.deleteRepository, Description: "删除仓库", Persist: &persist},
			{Method: "GET", RelativePath: "/repositories/:id", Handler: hr.getRepository, Description: "获取仓库详情", Persist: &persist},
			{Method: "GET", RelativePath: "/repositories", Handler: hr.listRepositories, Description: "获取仓库列表", Persist: &persist},
			{Method: "GET", RelativePath: "/repositories/:id/charts", Handler: hr.getRepoCharts, Description: "获取仓库Chart列表", Persist: &persist},
			{Method: "GET", RelativePath: "/repositories/charts", Handler: hr.getRepoChartsByURL, Description: "按URL获取Chart", Persist: &persist},
			{Method: "GET", RelativePath: "/repositories/values", Handler: hr.getChartValues, Description: "获取Chart Values", Persist: &persist},
			{Method: "POST", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases", Handler: hr.InstallRelease, Description: "安装Release", Persist: &persist},
			{Method: "PUT", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases", Handler: hr.UpgradeRelease, Description: "升级Release", Persist: &persist},
			{Method: "DELETE", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases/:name", Handler: hr.UninstallRelease, Description: "卸载Release", Persist: &persist},
			{Method: "GET", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases/:name", Handler: hr.GetRelease, Description: "获取Release详情", Persist: &persist},
			{Method: "GET", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases", Handler: hr.ListReleases, Description: "获取Release列表", Persist: &persist},
			{Method: "GET", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases/:name/history", Handler: hr.GetReleaseHistory, Description: "获取Release历史", Persist: &persist},
			{Method: "POST", RelativePath: "/clusters/:cluster/namespaces/:namespace/releases/:name/rollback", Handler: hr.RollbackRelease, Description: "回滚Release", Persist: &persist},
		},
	}
	group.Register(httpEngine.Group(helmBaseURL), hr.c.APIResource())
}
