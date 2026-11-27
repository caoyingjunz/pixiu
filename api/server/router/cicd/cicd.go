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

package cicd

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const cicdURL = "/pixiu/cicd"

type cicdRouter struct{ c controller.PixiuInterface }

func NewRouter(o *options.Options) {
	router := &cicdRouter{c: o.Controller}
	router.initRoutes(o.HttpEngine)
}

func (r *cicdRouter) initRoutes(engine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "CI/CD 流水线",
		BaseURL: cicdURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "GET", RelativePath: "/clusters/:clusterId/engine-status", Handler: r.engineStatus, Description: "获取 Tekton 引擎状态"},

			{Method: "GET", RelativePath: "/pipelines", Handler: r.listPipelines, Description: "流水线列表"},
			{Method: "POST", RelativePath: "/pipelines", Handler: r.createPipeline, Description: "创建流水线"},
			{Method: "GET", RelativePath: "/pipelines/:pipelineId", Handler: r.getPipeline, Description: "流水线详情"},
			{Method: "PUT", RelativePath: "/pipelines/:pipelineId", Handler: r.updatePipeline, Description: "更新流水线"},
			{Method: "DELETE", RelativePath: "/pipelines/:pipelineId", Handler: r.deletePipeline, Description: "删除流水线"},
			{Method: "POST", RelativePath: "/pipelines/:pipelineId/runs", Handler: r.triggerRun, Description: "手动触发运行"},

			{Method: "GET", RelativePath: "/runs", Handler: r.listRuns, Description: "运行记录列表"},
			{Method: "GET", RelativePath: "/runs/:runId", Handler: r.getRun, Description: "运行详情"},
			{Method: "GET", RelativePath: "/runs/:runId/logs", Handler: r.runLogs, Description: "运行日志 SSE"},
			{Method: "POST", RelativePath: "/runs/:runId/cancel", Handler: r.cancelRun, Description: "取消运行"},
			{Method: "POST", RelativePath: "/runs/:runId/retry", Handler: r.retryRun, Description: "重跑"},

			{Method: "GET", RelativePath: "/clusters/:clusterId/namespaces/:namespace/credentials", Handler: r.listCredentials, Description: "凭据列表"},
			{Method: "POST", RelativePath: "/clusters/:clusterId/namespaces/:namespace/credentials", Handler: r.createCredential, Description: "创建凭据"},
			{Method: "DELETE", RelativePath: "/clusters/:clusterId/namespaces/:namespace/credentials/:name", Handler: r.deleteCredential, Description: "删除凭据"},
		},
	}
	group.Register(engine.Group(cicdURL), r.c.APIResource())
}
