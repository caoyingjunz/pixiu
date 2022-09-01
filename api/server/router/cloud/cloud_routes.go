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
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
)

func (s *cloudRouter) listDeployments(c *gin.Context) {
	r := httputils.NewResponse()
	namespace := c.Param("namespace")
	deployments, err := pixiu.CoreV1.Cloud().ListDeployments(context.TODO(), namespace)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	r.Result = deployments.Items
	httputils.SetSuccess(c, r)
}

func (s *cloudRouter) deleteDeployments(c *gin.Context) {
	r := httputils.NewResponse()
	name := c.Param("name")
	namespace := c.Param("namespace")

	err := pixiu.CoreV1.Cloud().DeleteDeployments(context.TODO(), namespace, name)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}
