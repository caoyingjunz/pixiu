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
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) createKubeConfig(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err       error
		cloudMeta types.CloudMeta
		opts      types.KubeConfigOptions
	)
	if err = c.ShouldBindUri(&cloudMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = c.ShouldBindJSON(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	opts.CloudName = cloudMeta.CloudName
	if r.Result, err = pixiu.CoreV1.Cloud().KubeConfigs(opts.CloudName).Create(context.TODO(), &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateKubeConfig(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts types.KubeConfigOptions
	)
	if err = c.ShouldBindUri(&opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = pixiu.CoreV1.Cloud().KubeConfigs(opts.CloudName).Update(context.TODO(), opts.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteKubeConfig(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err         error
		cloudIdMeta types.CloudIdMeta
	)
	if err = c.ShouldBindUri(&cloudIdMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Cloud().KubeConfigs(cloudIdMeta.CloudName).Delete(context.TODO(), cloudIdMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getKubeConfig(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err         error
		cloudIdMeta types.CloudIdMeta
	)
	if err = c.ShouldBindUri(&cloudIdMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = pixiu.CoreV1.Cloud().KubeConfigs(cloudIdMeta.CloudName).Get(context.TODO(), cloudIdMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) listKubeConfig(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts types.KubeConfigOptions
	)
	opts.CloudName = c.Param("cloud_name")
	if r.Result, err = pixiu.CoreV1.Cloud().KubeConfigs(opts.CloudName).List(context.TODO()); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
