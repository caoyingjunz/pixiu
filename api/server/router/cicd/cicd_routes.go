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
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cicdRouter) runJob(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	err := pixiu.CoreV1.Cicd().RunJob(c, cicd.Name)
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

	if err := pixiu.CoreV1.Cicd().CreateJob(c, cicd); err != nil {
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
	if _, err := pixiu.CoreV1.Cicd().CopyJob(c, cicd.OldName, cicd.NewName); err != nil {
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
	if err := pixiu.CoreV1.Cicd().RenameJob(c, cicd.OldName, cicd.NewName); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) getAllJobs(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetAllJobs(c)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) getAllViews(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetAllViews(c)
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
	r.Result, err = pixiu.CoreV1.Cicd().GetLastFailedBuild(c, cicd.Name)
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
	r.Result, err = pixiu.CoreV1.Cicd().GetLastSuccessfulBuild(c, cicd.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) details(c *gin.Context) {
	r := httputils.NewResponse()
	name := c.Param("name")
	r.Result = pixiu.CoreV1.Cicd().Details(c, name)
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) getAllNodes(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().GetAllNodes(c)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) deleteJob(c *gin.Context) {
	r := httputils.NewResponse()
	name := c.Param("name")
	if err := pixiu.CoreV1.Cicd().DeleteJob(c, name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) deleteViewJob(c *gin.Context) {
	r := httputils.NewResponse()
	parm := map[string]interface{}{"name": c.Param("name"), "viewname": c.Param("viewname")}
	var err error
	r.Result, err = pixiu.CoreV1.Cicd().DeleteViewJob(c, parm["name"].(string), parm["viewname"].(string))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) deleteNode(c *gin.Context) {
	r := httputils.NewResponse()
	nodename := c.Param("nodename")
	if err := pixiu.CoreV1.Cicd().DeleteNode(c, nodename); err != nil {
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
	if err := pixiu.CoreV1.Cicd().AddViewJob(c, cicd.ViewName, cicd.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) restart(c *gin.Context) {
	r := httputils.NewResponse()
	if err := pixiu.CoreV1.Cicd().Restart(c); err != nil {
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
	if _, err := pixiu.CoreV1.Cicd().Disable(c, cicd.Name); err != nil {
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
	if _, err := pixiu.CoreV1.Cicd().Enable(c, cicd.Name); err != nil {
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
	if _, err := pixiu.CoreV1.Cicd().Stop(c, cicd.Name); err != nil {
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
	r.Result, err = pixiu.CoreV1.Cicd().Config(c, cicd.Name)
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
	if err := pixiu.CoreV1.Cicd().UpdateConfig(c, cicd.Name); err != nil {
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
	r.Result, err = pixiu.CoreV1.Cicd().History(c, cicd.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
