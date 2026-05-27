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

package plan

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

type planRouter struct {
	c controller.PixiuInterface
}

func NewRouter(o *options.Options) {
	router := &planRouter{
		c: o.Controller,
	}
	router.initRoutes(o.HttpEngine)
}

func (t *planRouter) initRoutes(ginEngine *gin.Engine) {
	persist := false
	group := &apiregistry.Group{
		Name:    "部署",
		BaseURL: "/pixiu/plans",
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "", Handler: t.createPlan, Description: "创建部署"},
			{Method: "PUT", RelativePath: "/:planId", Handler: t.updatePlan, Description: "更新部署2", Persist: &persist},
			{Method: "DELETE", RelativePath: "/:planId", Handler: t.deletePlan, Description: "删除部署"},
			{Method: "GET", RelativePath: "/:planId", Handler: t.getPlan, Description: "部署详情"},
			{Method: "GET", RelativePath: "", Handler: t.listPlans, Description: "部署列表"},
			{Method: "GET", RelativePath: "/:planId/resources", Handler: t.getPlanWithSubResources, Description: "更新部署"},
			{Method: "POST", RelativePath: "/:planId/start", Handler: t.startPlan, Description: "启动"},
			{Method: "POST", RelativePath: "/:planId/stop", Handler: t.stopPlan, Description: "终止"},
			{Method: "POST", RelativePath: "/:planId/destroy", Handler: t.destroyPlan, Description: "销毁"},
			{Method: "POST", RelativePath: "/:planId/nodes", Handler: t.createPlanNode, Description: "部署节点", Persist: &persist},
			{Method: "PUT", RelativePath: "/:planId/nodes/:nodeId", Handler: t.updatePlanNode, Description: "更新部署计划节点", Persist: &persist},
			{Method: "DELETE", RelativePath: "/:planId/nodes/:nodeId", Handler: t.deletePlanNode, Description: "删除部署计划节点", Persist: &persist},
			{Method: "GET", RelativePath: "/:planId/nodes/:nodeId", Handler: t.getPlanNode, Description: "获取部署计划节点详情", Persist: &persist},
			{Method: "GET", RelativePath: "/:planId/nodes", Handler: t.listPlanNodes, Description: "获取部署计划节点列表", Persist: &persist},
			{Method: "POST", RelativePath: "/:planId/configs", Handler: t.createPlanConfig, Description: "创建部署配置", Persist: &persist},
			{Method: "PUT", RelativePath: "/:planId/configs/:configId", Handler: t.updatePlanConfig, Description: "更新部署配置", Persist: &persist},
			{Method: "DELETE", RelativePath: "/:planId/configs/:configId", Handler: t.deletePlanConfig, Description: "删除部署配置", Persist: &persist},
			{Method: "GET", RelativePath: "/:planId/configs", Handler: t.getPlanConfig, Description: "配置", Persist: &persist},
			{Method: "POST", RelativePath: "/:planId/tasks/:taskId", Handler: t.runTasks, Description: "执行"},
			{Method: "GET", RelativePath: "/:planId/tasks", Handler: t.listTasks, Description: "查询任务"},
			{Method: "GET", RelativePath: "/:planId/tasks/:taskId/logs", Handler: t.watchTaskLog, Description: "部署日志"},
			{Method: "GET", RelativePath: "/distributions", Handler: t.getDistributions, Description: "获取操作系统"},
		},
	}
	group.Register(ginEngine.Group("/pixiu/plans"), t.c.APIResource())
}
