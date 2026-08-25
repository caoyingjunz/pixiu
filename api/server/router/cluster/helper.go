/*
Copyright 2024 The Pixiu Authors.

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

package cluster

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
)

func IsKubeProxyPath(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, kubeProxyBaseURL)
}

func IsHelmPath(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, helmBaseURL)
}

// authorizeClusterAccessByName 取当前登录用户并校验其对指定集群（按名）的访问权限。
func (cr *clusterRouter) authorizeClusterAccessByName(c *gin.Context, clusterName string) error {
	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		return err
	}
	_, err = cr.c.Cluster().AuthorizeClusterAccessByName(c, user, clusterName)
	return err
}

// authorizeClusterAccess 取当前登录用户并校验其对指定集群（按 id）的访问权限。
func (cr *clusterRouter) authorizeClusterAccess(c *gin.Context, clusterId int64) error {
	user, err := httputils.GetUserFromContext(c)
	if err != nil {
		return err
	}
	_, err = cr.c.Cluster().AuthorizeClusterAccess(c, user, clusterId)
	return err
}
