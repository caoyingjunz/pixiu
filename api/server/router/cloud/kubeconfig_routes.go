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
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

// createKubeConfig godoc
// @Summary      Create a cloud custom kubeConfig
// @Description  Create by cloud kubeConfig
// @Tags         kubeConfigs
// @Accept       json
// @Produce      json
// @Param        cloud_name  path string  true  "cloud name"  Format(string)
// @Param        data body types.KubeConfigOptions true "service_account, cluster_role"
// @Success      200  {object}  httputils.Response{result=types.KubeConfigOptions}
// @Failure      400  {object}  httputils.Response
// @Router       /clouds/v1/{cloud_name}/kubeconfigs [post]
func (s *cloudRouter) createKubeConfig(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err       error
		cloudMeta types.CloudUriMeta
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
	if r.Result, err = pixiu.CoreV1.Cloud().KubeConfigs(opts.CloudName).Create(c, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// updateKubeConfig godoc
// @Summary      Update a cloud custom kubeConfig
// @Description  Update by cloud kubeConfig
// @Tags         kubeConfigs
// @Accept       json
// @Produce      json
// @Param        cloud_name  path string  true  "cloud name"  Format(string)
// @Param        id   path      int  true  "Cloud ID"  Format(int64)
// @Success      200  {object}  httputils.Response{result=types.KubeConfigOptions}
// @Failure      400  {object}  httputils.Response
// @Router       /clouds/v1/{cloud_name}/kubeconfigs [put]
func (s *cloudRouter) updateKubeConfig(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err         error
		cloudIdMeta types.CloudUriMeta
	)
	if err = c.ShouldBindUri(&cloudIdMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = pixiu.CoreV1.Cloud().KubeConfigs(cloudIdMeta.CloudName).Update(c, cloudIdMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// deleteKubeConfig godoc
// @Summary      Delete a cloud custom kubeConfig
// @Description  Delete by cloud kubeConfig ID
// @Tags         kubeConfigs
// @Accept       json
// @Produce      json
// @Param        cloud_name  path string  true  "cloud name"  Format(string)
// @Param        id   path      int  true  "Cloud ID"  Format(int64)
// @Success      200  {object}  httputils.Response
// @Failure      400  {object}  httputils.Response
// @Router       /clouds/v1/{cloud_name}/kubeconfigs/{id} [delete]
func (s *cloudRouter) deleteKubeConfig(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err         error
		cloudIdMeta types.CloudUriMeta
	)
	if err = c.ShouldBindUri(&cloudIdMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Cloud().KubeConfigs(cloudIdMeta.CloudName).Delete(c, cloudIdMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// getKubeConfig godoc
// @Summary      get a cloud custom kubeConfig
// @Description  get by cloud kubeConfig ID
// @Tags         kubeConfigs
// @Accept       json
// @Produce      json
// @Param        cloud_name  path string  true  "cloud name"  Format(string)
// @Param        id   path      int  true  "kubeConfig ID"  Format(int64)
// @Success      200  {object}  httputils.Response{result=types.KubeConfigOptions}
// @Failure      400  {object}  httputils.Response
// @Router       /clouds/v1/{cloud_name}/kubeconfigs/{id} [get]
func (s *cloudRouter) getKubeConfig(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err         error
		cloudIdMeta types.CloudUriMeta
	)
	if err = c.ShouldBindUri(&cloudIdMeta); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = pixiu.CoreV1.Cloud().KubeConfigs(cloudIdMeta.CloudName).Get(c, cloudIdMeta.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// listKubeConfig godoc
// @Summary      get a cloud custom kubeConfig
// @Description  get by cloud kubeConfig ID
// @Tags         kubeConfigs
// @Accept       json
// @Produce      json
// @Param        cloud_name  path string  true  "cloud name"  Format(string)
// @Success      200  {object}  httputils.Response{result=[]types.KubeConfigOptions}
// @Failure      400  {object}  httputils.Response
// @Router       /clouds/v1/{cloud_name}/kubeconfigs [get]
func (s *cloudRouter) listKubeConfig(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err  error
		opts types.KubeConfigOptions
	)
	opts.CloudName = c.Param("cloud_name")
	if r.Result, err = pixiu.CoreV1.Cloud().KubeConfigs(opts.CloudName).List(c); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
