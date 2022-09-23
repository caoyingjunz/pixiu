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
)

func (s *cloudRouter) createKubeConfig(c *gin.Context) {
	var (
		r                 = httputils.NewResponse()
		err               error
		kubeConfigOptions = new(types.KubeConfig)
	)
	if err = c.ShouldBindJSON(kubeConfigOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	kubeConfigOptions.CloudName = c.Param("cloud_name")
	if r.Result, err = pixiu.CoreV1.Cloud().KubeConfigs(kubeConfigOptions.CloudName).Create(context.TODO(), kubeConfigOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) updateKubeConfig(c *gin.Context) {
	var (
		r                 = httputils.NewResponse()
		err               error
		kubeConfigOptions = new(types.KubeConfig)
	)
	if err = c.ShouldBindUri(kubeConfigOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = pixiu.CoreV1.Cloud().KubeConfigs(kubeConfigOptions.CloudName).Update(context.TODO(), kubeConfigOptions.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteKubeConfig(c *gin.Context) {
	var (
		r                 = httputils.NewResponse()
		kubeConfigOptions = new(types.KubeConfig)
	)
	if err := c.ShouldBindUri(kubeConfigOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err := pixiu.CoreV1.Cloud().KubeConfigs(kubeConfigOptions.CloudName).Delete(context.TODO(), kubeConfigOptions.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getKubeConfig(c *gin.Context) {
	var (
		r                 = httputils.NewResponse()
		err               error
		kubeConfigOptions = new(types.KubeConfig)
	)
	if err = c.ShouldBindUri(kubeConfigOptions); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = pixiu.CoreV1.Cloud().KubeConfigs(kubeConfigOptions.CloudName).Get(context.TODO(), kubeConfigOptions.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) listKubeConfig(c *gin.Context) {
	var (
		r         = httputils.NewResponse()
		err       error
		cloudName = c.Param("cloud_name")
	)
	if len(cloudName) == 0 {
		httputils.SetFailed(c, r, fmt.Errorf("invaild empty cloud name"))
		return
	}
	if r.Result, err = pixiu.CoreV1.Cloud().KubeConfigs(cloudName).List(context.TODO(), cloudName); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
