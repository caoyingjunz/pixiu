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

package cicd

import (
	"context"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"

	"github.com/gin-gonic/gin"
)

func (s *cicdRouter) runJob(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	err := pixiu.CoreV1.Cicd().RunJob(context.TODO(), cicd.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) createJob(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err := pixiu.CoreV1.Cicd().CreateJob(context.TODO(), cicd.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) copyJob(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if _, err := pixiu.CoreV1.Cicd().CopyJob(context.TODO(), cicd.OldName, cicd.NewName); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) renameJob(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.Cicd().RenameJob(context.TODO(), cicd.OldName, cicd.NewName); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) getAllJobs(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetAllJobs(context.TODO())
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) getAllViews(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetAllViews(context.TODO())
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) getLastFailedBuild(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetLastFailedBuild(context.TODO(), cicd.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) getLastSuccessfulBuild(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetLastSuccessfulBuild(context.TODO(), cicd.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) details(c *gin.Context) {
	r := httputils.NewResponse()
	name := c.Param("name")
	r.Result = pixiu.CoreV1.Cicd().Details(context.TODO(), name)
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) getAllNodes(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetAllNodes(context.TODO())
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) deleteJob(c *gin.Context) {
	r := httputils.NewResponse()
	name := c.Param("name")
	if err := pixiu.CoreV1.Cicd().DeleteJob(context.TODO(), name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) deleteViewJob(c *gin.Context) {
	r := httputils.NewResponse()
	parm := map[string]interface{}{"name": c.Param("name"), "viewname": c.Param("viewname")}
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().DeleteViewJob(context.TODO(), parm["name"].(string), parm["viewname"].(string))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) deleteNode(c *gin.Context) {
	r := httputils.NewResponse()
	nodename := c.Param("nodename")
	if err := pixiu.CoreV1.Cicd().DeleteNode(context.TODO(), nodename); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) addViewJob(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.Cicd().AddViewJob(context.TODO(), cicd.ViewName, cicd.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) restart(c *gin.Context) {
	r := httputils.NewResponse()
	if err := pixiu.CoreV1.Cicd().Restart(context.TODO()); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) disable(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if _, err := pixiu.CoreV1.Cicd().Disable(context.TODO(), cicd.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) enable(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if _, err := pixiu.CoreV1.Cicd().Enable(context.TODO(), cicd.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) stop(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if _, err := pixiu.CoreV1.Cicd().Stop(context.TODO(), cicd.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) config(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().Config(context.TODO(), cicd.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) updateConfig(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.Cicd().UpdateConfig(context.TODO(), cicd.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) history(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().History(context.TODO(), cicd.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
