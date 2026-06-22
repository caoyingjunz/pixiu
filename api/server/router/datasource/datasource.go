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

package datasource

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const datasourceBaseURL = "/pixiu/datasources"

// datasourceRouter is a router to talk with the datasource controller
type datasourceRouter struct {
	c controller.PixiuInterface
}

// NewRouter initializes a new datasource router
func NewRouter(o *options.Options) {
	s := &datasourceRouter{
		c: o.Controller,
	}
	s.initRoutes(o.HttpEngine)
}

func (dr *datasourceRouter) initRoutes(ginEngine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "数据源",
		BaseURL: datasourceBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: dr.createDatasource, Description: "创建数据源"},
			{Method: "PUT", RelativePath: "/:datasourceId", Handler: dr.updateDatasource, Description: "更新数据源"},
			{Method: "DELETE", RelativePath: "/:datasourceId", Handler: dr.deleteDatasource, Description: "删除数据源"},
			{Method: "GET", RelativePath: "", Handler: dr.listDatasources, Description: "获取列表"},
			{Method: "GET", RelativePath: "/:datasourceId", Handler: dr.getDatasource, Description: "查看详情"},
		},
	}
	group.Register(ginEngine.Group(datasourceBaseURL), dr.c.APIResource())
}
