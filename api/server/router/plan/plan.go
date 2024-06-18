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
	planRoute := ginEngine.Group("/pixiu/plans")
	{
		planRoute.POST("", t.createPlan)
		planRoute.PUT("/:planId", t.updatePlan)
		planRoute.DELETE("/:planId", t.deletePlan)
		planRoute.GET("/:planId", t.getPlan)
		planRoute.GET("", t.listPlans)

		planRoute.GET("/:planId/resources", t.getPlanWithSubResources)

		// 启动部署任务
		planRoute.POST("/:planId/start", t.startPlan)
		// 终止部署任务
		planRoute.POST("/:planId/stop", t.stopPlan)

		// 部署计划的节点API
		planRoute.POST("/:planId/nodes", t.createPlanNode)
		planRoute.PUT("/:planId/nodes/:nodeId", t.updatePlanNode)
		planRoute.DELETE("/:planId/nodes/:nodeId", t.deletePlanNode)
		planRoute.GET("/:planId/nodes/:nodeId", t.getPlanNode)
		planRoute.GET("/:planId/nodes", t.listPlanNodes)

		// 部署计划的部署配置
		planRoute.POST("/:planId/configs", t.createPlanConfig)
		planRoute.PUT("/:planId/configs/:configId", t.updatePlanConfig)
		planRoute.DELETE("/:planId/configs/:configId", t.deletePlanConfig)
		planRoute.GET("/:planId/configs", t.getPlanConfig)

		// 执行指定任务
		planRoute.POST("/:planId/tasks/:taskId", t.runTasks)
		// 查询任务列表
		planRoute.POST("/:planId/tasks", t.listTasks)
	}
}
