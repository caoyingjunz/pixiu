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

package k8s

import (
	"context"
	"fmt"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/pixiu"
	"github.com/gin-gonic/gin"
)

func (s *k8sRouter) createCluster(c *gin.Context) {
	r := new(httputils.Response)
	req := new(types.K8sClusterCreate)
	name := c.PostForm("name")
	config := c.PostForm("config")
	if len(name) == 0 || len(config) == 0 {
		httputils.SetFailed(c, r, fmt.Errorf("参数为空"))
		return
	}
	req.Name, req.Config = name, config
	if err := pixiu.CoreV1.K8s().ClusterCreate(context.TODO(), req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	httputils.SetSuccess(c, r)
}
