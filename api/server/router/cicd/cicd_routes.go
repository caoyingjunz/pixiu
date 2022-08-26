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
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cicdRouter) runJob(c *gin.Context) {
	r := httputils.NewResponse()
	jobName := c.Param("jobName")
	_, res, err := pixiu.CoreV1.Cicd().RunJob(context.TODO(), jobName)

	r.Result = res
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) createJob(c *gin.Context) {
	r := httputils.NewResponse()
	createJob := c.Param("createJob")
	if err := pixiu.CoreV1.Cicd().CreateJob(context.TODO(), createJob); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) deleteJob(c *gin.Context) {
	r := httputils.NewResponse()
	deleteJob := c.Param("deleteJob")
	if err := pixiu.CoreV1.Cicd().DeleteJob(context.TODO(), deleteJob); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) addViewJob(c *gin.Context) {
	r := httputils.NewResponse()
	addViewJob := c.Param("addViewJob")
	if err := pixiu.CoreV1.Cicd().AddViewJob(context.TODO(), addViewJob); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) getAllJobs(c *gin.Context) {
	r := httputils.NewResponse()
	getAllJobs := c.Param("getAllJobs")
	jobs, err := pixiu.CoreV1.Cicd().GetAllJobs(context.TODO(), getAllJobs)
	r.Result = jobs
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) copyJob(c *gin.Context) {
	r := httputils.NewResponse()
	oldCopyJob := c.Param("oldCopyJob")
	newCopyJob := c.Param("newCopyJob")
	if _, err := pixiu.CoreV1.Cicd().CopyJob(context.TODO(), oldCopyJob, newCopyJob); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) renameJob(c *gin.Context) {
	r := httputils.NewResponse()
	oldRenameJob := c.Param("oldRenameJob")
	newRenameJob := c.Param("newRenameJob")
	if err := pixiu.CoreV1.Cicd().RenameJob(context.TODO(), oldRenameJob, newRenameJob); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) safeRestart(c *gin.Context) {
	r := httputils.NewResponse()
	safeRestart := c.Param("safeRestart")
	if err := pixiu.CoreV1.Cicd().SafeRestart(context.TODO(), safeRestart); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
