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
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/gin-gonic/gin"
)

func (s *cicdRouter) runJob(c *gin.Context) {
	r := httputils.NewResponse()
	name := c.Param("job_name")
	err := pixiu.CoreV1.Cicd().RunJob(context.TODO(), name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) createJob(c *gin.Context) {
	r := httputils.NewResponse()
	reqBody, _ := ioutil.ReadAll(c.Request.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(reqBody, &m)
	newBody, _ := json.Marshal(m)
	c.Request.Body = ioutil.NopCloser(bytes.NewReader(newBody))
	if err := pixiu.CoreV1.Cicd().CreateJob(context.TODO(), m["name"]); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) copyJob(c *gin.Context) {
	r := httputils.NewResponse()
	reqBody, _ := ioutil.ReadAll(c.Request.Body)
	var m map[string]string
	_ = json.Unmarshal(reqBody, &m)
	newBody, _ := json.Marshal(m)
	c.Request.Body = ioutil.NopCloser(bytes.NewReader(newBody))

	if _, err := pixiu.CoreV1.Cicd().CopyJob(context.TODO(), m["old_name"], m["new_name"]); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) renameJob(c *gin.Context) {
	r := httputils.NewResponse()
	reqBody, _ := ioutil.ReadAll(c.Request.Body)
	var m map[string]string
	_ = json.Unmarshal(reqBody, &m)
	newBody, _ := json.Marshal(m)
	c.Request.Body = ioutil.NopCloser(bytes.NewReader(newBody))
	if err := pixiu.CoreV1.Cicd().RenameJob(context.TODO(), m["old_name"], m["new_name"]); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) getAllJobs(c *gin.Context) {
	r := httputils.NewResponse()
	//get_all_jobs := c.Param("get_all_jobs")
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
	name := c.Param("name")
	if err := pixiu.CoreV1.Cicd().DeleteJob(context.TODO(), name); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cicdRouter) addViewJob(c *gin.Context) {
	r := httputils.NewResponse()
	add_view_job := c.Param("add_view_job")
	if err := pixiu.CoreV1.Cicd().AddViewJob(context.TODO(), add_view_job); err != nil {
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
