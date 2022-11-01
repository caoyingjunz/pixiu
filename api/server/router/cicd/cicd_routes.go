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

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

// runJob godoc
// @Summary      runJob jenkins job
// @Description  runJob jenkins job
// @Tags         runJob
// @Accept       json
// @Produce      json
// @Success      200  {object}  httputils.Response
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs/run [post]
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

// createJob godoc
// @Summary      create jenkins job
// @Description  create jenkins job
// @Tags         create
// @Accept       json
// @Produce      json
// @Param        data body types.Cicd true "git"
// @Success      200  {object}  httputils.Response
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs/createJob [post]
func (s *cicdRouter) createJob(c *gin.Context) {
	r := httputils.NewResponse()
	var cicd types.Cicd
	if err := c.ShouldBindJSON(&cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	if err := pixiu.CoreV1.Cicd().CreateJob(context.TODO(), cicd); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// copyJob godoc
// @Summary      copy jenkins job
// @Description  copy jenkins job
// @Tags         copy
// @Accept       json
// @Produce      json
// @Param        data body types.Cicd true "oldName, newName"
// @Success      200  {object}  httputils.Response
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs/copy [post]
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

// renameJob godoc
// @Summary      rename jenkins job
// @Description  rename jenkins job
// @Tags         rename
// @Accept       json
// @Produce      json
// @Param        data body types.Cicd true "oldName, newName"
// @Success      200  {object}  httputils.Response
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs/rename [post]
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

// getAllJobs godoc
// @Summary      get jenkins allJob
// @Description  get jenkins allJob
// @Tags         getAllJobs
// @Accept       json
// @Produce      json
// @Success      200  {object}  httputils.Response
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs [get]
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

// getAllViews godoc
// @Summary      get jenkins allView
// @Description  get jenkins allView
// @Tags         getAllViews
// @Accept       json
// @Produce      json
// @Success      200  {object}  httputils.Response{result=[]string}
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/view [get]
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

// getLastFailedBuild godoc
// @Summary      get jenkins LastFailedBuild
// @Description  get jenkins LastFailedBuild
// @Tags         getLastFailedBuild
// @Accept       json
// @Produce      json
// @Param        data body types.Cicd true "name"
// @Success      200  {object}  httputils.Response{result=[]string}
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs/failed [post]
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

// getLastFailedBuild godoc
// @Summary      get jenkins LastFailedBuild
// @Description  get jenkins LastFailedBuild
// @Tags         getLastFailedBuild
// @Accept       json
// @Produce      json
// @Param        data body types.Cicd true "name"
// @Success      200  {object}  httputils.Response{result=[]string}
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs/failed [post]
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

// deleteJob godoc
// @Summary      delete jenkins job
// @Description  delete jenkins job
// @Tags         deleteJob
// @Accept       json
// @Produce      json
// @Param        name
// @Success      200  {object}  httputils.Response{result=[]string}
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs/failed [post]
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

// restart godoc
// @Summary      restart jenkins server
// @Description  restart jenkins server
// @Tags         restart
// @Accept       json
// @Produce      json
// @Success      200  {object}  httputils.Response
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/restart [post]
func (s *cicdRouter) restart(c *gin.Context) {
	r := httputils.NewResponse()
	if err := pixiu.CoreV1.Cicd().Restart(context.TODO()); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// disable godoc
// @Summary      disable jenkins job
// @Description  disable jenkins job
// @Tags         disable
// @Accept       json
// @Produce      json
// @Param        cicd.Name  path string  true  "name"  Format(string)
// @Success      200
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs/disable [post]
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

// enable godoc
// @Summary      enable jenkins job
// @Description  enable jenkins job
// @Tags         enable
// @Accept       json
// @Produce      json
// @Param        cicd.Name  path string  true  "name"  Format(string)
// @Success      200
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs/enable [post]
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

// stop godoc
// @Summary      stop jenkins job
// @Description  stop jenkins job
// @Tags         stop
// @Accept       json
// @Produce      json
// @Param        cicd.Name  path string  true  "name"  Format(string)
// @Success      200
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs/stop [post]
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

// config godoc
// @Summary      config jenkins job
// @Description  config jenkins job
// @Tags         config
// @Accept       json
// @Produce      json
// @Param        cicd.Name  path string  true  "name"  Format(string)
// @Success      200  {object}  httputils.Response{result=config}
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs/config [post]
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

// history godoc
// @Summary      history jenkins job
// @Description  history jenkins job
// @Tags         history
// @Accept       json
// @Produce      json
// @Param        cicd.Name  path string  true  "name"  Format(string)
// @Success      200  {object}  httputils.Response{result=[]*gojenkins.History}
// @Failure      400  {object}  httputils.Response
// @Router       /cicd/jobs/history [post]
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
