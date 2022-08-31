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
	jobs, err := pixiu.CoreV1.Cicd().GetAllJobs(context.TODO())
	r.Result = jobs
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) getAllViews(c *gin.Context) {
	r := httputils.NewResponse()
	allviews, err := pixiu.CoreV1.Cicd().GetAllViews(context.TODO())
	r.Result = allviews
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) getAllNodes(c *gin.Context) {
	r := httputils.NewResponse()
	node, err := pixiu.CoreV1.Cicd().GetAllNodes(context.TODO())
	r.Result = node
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
