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
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	pixiumeta "github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	typesv2 "github.com/caoyingjunz/gopixiu/pkg/types"
	"github.com/caoyingjunz/gopixiu/pkg/util"
	"github.com/caoyingjunz/gopixiu/pkg/util/audit"
)

// buildCloud godoc
// @Summary      自建 kubernetes 集群
// @Description  自建 kubernetes 集群
// @Tags         clouds
// @Accept       json
// @Produce      json
// @Param        buildCloud body types.BuildCloud true "build a cloud"
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /clouds/build [post]
func (s *cloudRouter) buildCloud(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err   error
		cloud types.BuildCloud
	)
	if err = c.ShouldBindJSON(&cloud); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Cloud().Build(c, &cloud); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) createCloud(c *gin.Context) {
	r := httputils.NewResponse()
	var (
		err   error
		cloud types.Cloud
	)
	// 前端约定，k8s 集群的原始数据通过 clusterData 传递
	// 如果获取data失败，或者data为空，则不允许创建
	data, err := httputils.ReadFile(c, "clusterData")
	if err != nil || len(data) == 0 {
		httputils.SetFailed(c, r, fmt.Errorf("failed to get cluster raw data"))
		return
	}
	if err = json.Unmarshal(data, &cloud); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	// 获取 kubeConfig 文件
	if cloud.KubeConfig, err = httputils.ReadFile(c, "kubeconfig"); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Cloud().Create(c, &cloud); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	audit.SetAuditEvent(c, typesv2.CreateEvent, typesv2.CloudResource, "")
	httputils.SetSuccess(c, r)
}

// TODO
func (s *cloudRouter) updateCloud(c *gin.Context) {
	r := httputils.NewResponse()
	httputils.SetSuccess(c, r)
}

// deleteCloud godoc
// @Summary      Delete a cloud
// @Description  Delete by cloud ID
// @Tags         clouds
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Cloud ID"  Format(int64)
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /clouds/{id} [delete]
func (s *cloudRouter) deleteCloud(c *gin.Context) {
	r := httputils.NewResponse()
	cid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Cloud().Delete(c, cid); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// getCloud godoc
// @Summary      Get a cloud
// @Description  get string by ID
// @Tags         clouds
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Cloud ID" Format(int64)
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /clouds/{id} [get]
func (s *cloudRouter) getCloud(c *gin.Context) {
	r := httputils.NewResponse()
	cid, err := util.ParseInt64(c.Param("id"))
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result, err = pixiu.CoreV1.Cloud().Get(c, cid)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) listClouds(c *gin.Context) {
	r := httputils.NewResponse()
	var err error
	if r.Result, err = pixiu.CoreV1.Cloud().List(c, pixiumeta.ParseListSelector(c)); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}

// pingCloud godoc
// @Summary      Ping a cloud
// @Description  通过 kubeConfig 检测与 kubernetes 集群的连通性
// @Tags         clouds
// @Accept       multipart/form-data
// @Produce      json
// @Param        kubeconfig  formData  file  true  "kubernetes kubeconfig"
// @Success      200  {object}  httputils.HttpOK
// @Failure      400  {object}  httputils.HttpError
// @Router       /clouds/ping [post]
func (s *cloudRouter) pingCloud(c *gin.Context) {
	r := httputils.NewResponse()
	data, err := httputils.ReadFile(c, "kubeconfig")
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = pixiu.CoreV1.Cloud().Ping(c, data); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
