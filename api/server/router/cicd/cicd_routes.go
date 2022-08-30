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
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/gin-gonic/gin"
)

func (s *cicdRouter) runJob(c *gin.Context) {
	r := httputils.NewResponse()
	p := struct {
		Name string `json:"name,omitempty"`
	}{}
	_ = c.ShouldBindJSON(&p)
	err := pixiu.CoreV1.Cicd().RunJob(context.TODO(), p.Name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) createJob(c *gin.Context) {
	r := httputils.NewResponse()
	p := struct {
		Name string `json:"name,omitempty"`
	}{}
	_ = c.ShouldBindJSON(&p)
	if err := pixiu.CoreV1.Cicd().CreateJob(context.TODO(), p.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) copyJob(c *gin.Context) {
	r := httputils.NewResponse()
	p := struct {
		O_Name string `json:"o_name,omitempty"`
		N_Name string `json:"n_name,omitempty"`
	}{}
	_ = c.ShouldBindJSON(&p)

	if _, err := pixiu.CoreV1.Cicd().CopyJob(context.TODO(), p.O_Name, p.N_Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) renameJob(c *gin.Context) {
	r := httputils.NewResponse()
	p := struct {
		O_Name string `json:"o_name,omitempty"`
		N_Name string `json:"n_name,omitempty"`
	}{}
	_ = c.ShouldBindJSON(&p)
	if err := pixiu.CoreV1.Cicd().RenameJob(context.TODO(), p.O_Name, p.N_Name); err != nil {
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

func (s *cicdRouter) deleteJob(c *gin.Context) {
	r := httputils.NewResponse()
	p := struct {
		Name string `json:"name,omitempty"`
	}{}
	_ = c.ShouldBindJSON(&p)
	if err := pixiu.CoreV1.Cicd().DeleteJob(context.TODO(), p.Name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) addViewJob(c *gin.Context) {
	r := httputils.NewResponse()
	p := struct {
		View_Name string `json:"view_name,omitempty"`
		Job       string `json:"job,omitempty"`
	}{}
	_ = c.ShouldBindJSON(&p)
	if err := pixiu.CoreV1.Cicd().AddViewJob(context.TODO(), p.View_Name, p.Job); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) safeRestart(c *gin.Context) {
	r := httputils.NewResponse()
	if err := pixiu.CoreV1.Cicd().SafeRestart(context.TODO()); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
