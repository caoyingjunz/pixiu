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
	"io/ioutil"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func readConfig(c *gin.Context) ([]byte, error) {
	config, err := c.FormFile("kubeconfig")
	if err != nil {
		return nil, err
	}
	file, err := config.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ioutil.ReadAll(file)
}

func (s *cloudRouter) createCloud(c *gin.Context) {
	r := httputils.NewResponse()
	var cloud types.Cloud
	var err error
	if err = c.ShouldBindJSON(&cloud); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	cloud.KubeConfig, err = readConfig(c)
	if err != nil {
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
	cid := c.Param("cid")

	r.Result = cid

	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) getCloud(c *gin.Context) {}

func (s *cloudRouter) listClouds(c *gin.Context) {}

func (s *cloudRouter) listDeployments(c *gin.Context) {
	r := httputils.NewResponse()
	clusterName := c.Param("cluster_name")
	if len(clusterName) == 0 {
		httputils.SetFailed(c, r, fmt.Errorf("参数为空"))
		return
	}
	deployments, err := pixiu.CoreV1.Cloud().ListDeployments(context.TODO(), clusterName)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = deployments.Items
	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteDeployment(c *gin.Context) {
	r := httputils.NewResponse()

	httputils.SetSuccess(c, r)
}
