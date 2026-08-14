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

package dashboard

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	dashboardcontroller "github.com/caoyingjunz/pixiu/pkg/controller/dashboard"
)

func (r *dashboardRouter) definition(c *gin.Context) {
	response := httputils.NewResponse()
	response.Result = r.c.Dashboard().Definition()
	httputils.SetSuccess(c, response)
}

func (r *dashboardRouter) variables(c *gin.Context) {
	response := httputils.NewResponse()
	var req dashboardcontroller.VariableRequest
	if err := httputils.ShouldBindAny(c, nil, nil, &req); err != nil {
		httputils.SetFailed(c, response, err)
		return
	}
	result, err := r.c.Dashboard().Variables(c, req)
	if err != nil {
		httputils.SetFailed(c, response, err)
		return
	}
	response.Result = result
	httputils.SetSuccess(c, response)
}

func (r *dashboardRouter) query(c *gin.Context) {
	response := httputils.NewResponse()
	var req dashboardcontroller.QueryRequest
	if err := httputils.ShouldBindAny(c, &req, nil, nil); err != nil {
		httputils.SetFailed(c, response, err)
		return
	}
	result, err := r.c.Dashboard().Query(c, req)
	if err != nil {
		httputils.SetFailed(c, response, err)
		return
	}
	response.Result = result
	httputils.SetSuccess(c, response)
}
