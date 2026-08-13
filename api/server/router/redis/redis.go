/*
Copyright 2026 The Pixiu Authors.

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

package redis

import (
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const redisBaseURL = "/pixiu/redis"

// redisRouter is a router to talk with the redis controller
type redisRouter struct {
	c controller.PixiuInterface
}

// NewRouter initializes a new redis router
func NewRouter(o *options.Options) {
	s := &redisRouter{
		c: o.Controller,
	}
	s.initRoutes(o.HttpEngine)
}

func (rr *redisRouter) initRoutes(ginEngine *gin.Engine) {
	group := &apiregistry.Group{
		Name:    "Redis 管理",
		BaseURL: redisBaseURL,
		Entries: []apiregistry.RouteEntry{
			{Method: "POST", RelativePath: "/ping", Handler: rr.pingRedisAdhoc, Description: "Redis 临时连通性探测"},
			{Method: "GET", RelativePath: "/:datasourceId/ping", Handler: rr.pingRedis, Description: "Redis 连接探测"},
			{Method: "GET", RelativePath: "/:datasourceId/info", Handler: rr.getRedisInfo, Description: "Redis 实例概览"},
			{Method: "GET", RelativePath: "/:datasourceId/keys", Handler: rr.scanRedisKeys, Description: "Redis Key 扫描"},
			{Method: "GET", RelativePath: "/:datasourceId/key", Handler: rr.getRedisKeyDetail, Description: "Redis Key 详情"},
			{Method: "POST", RelativePath: "/:datasourceId/key", Handler: rr.createRedisKey, Description: "Redis 新增 Key"},
			{Method: "DELETE", RelativePath: "/:datasourceId/key", Handler: rr.deleteRedisKey, Description: "Redis 删除 Key"},
			{Method: "DELETE", RelativePath: "/:datasourceId/keys", Handler: rr.deleteRedisKeys, Description: "Redis 批量删除 Key"},
			{Method: "PUT", RelativePath: "/:datasourceId/key", Handler: rr.updateRedisKeyValue, Description: "Redis 修改 Key 值"},
			{Method: "POST", RelativePath: "/:datasourceId/key/ttl", Handler: rr.setRedisKeyTTL, Description: "Redis 修改 Key TTL"},
		},
	}
	group.Register(ginEngine.Group(redisBaseURL), rr.c.APIResource())
}
