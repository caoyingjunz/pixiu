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

package cloud

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/caoyingjunz/gopixiu/pkg/util"
)

func (s *cloudRouter) createCloud(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err   error
		cloud types.Cloud
	)
	cloud.Name = c.Param("name")
	if len(cloud.Name) == 0 {
		httputils.SetFailed(c, r, fmt.Errorf("invaild empty cloud name"))
		return
	}
	if cloud.KubeConfig, err = httputils.ReadFile(c, "kubeconfig"); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Cloud().Create(context.TODO(), &cloud); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateCloud(c *gin.Context) {
	r := httputils.NewResponse()
	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteCloud(c *gin.Context) {
	r := httputils.NewResponse()
	cid, err := util.ParseInt64(c.Param("cid"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Cloud().Delete(context.TODO(), cid); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getCloud(c *gin.Context) {
	r := httputils.NewResponse()
	cid, err := util.ParseInt64(c.Param("cid"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Get(context.TODO(), cid)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) listClouds(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err        error
		pageOption types.PageOptions
	)
	if err = c.ShouldBindQuery(&pageOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = pixiu.CoreV1.Cloud().List(context.TODO(), &pageOption); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
