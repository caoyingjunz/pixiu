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
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

// redisRouter is a router to talk with the redis controller
type redisRouter struct {
	c controller.PixiuInterface
}

// RegisterRedis 将 Redis 子模块路由注册到 extension 父路由组下，
// 完整路径为 /pixiu/extension/redis/...
func RegisterRedis(o *options.Options, group *apiregistry.Group) {
	rr := &redisRouter{
		c: o.Controller,
	}
	group.Entries = append(group.Entries,
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/redis/ping", Handler: rr.pingRedisAdhoc, Description: "Redis 临时连通性探测"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/redis/:datasourceId/ping", Handler: rr.pingRedis, Description: "Redis 连接探测"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/redis/:datasourceId/info", Handler: rr.getRedisInfo, Description: "Redis 实例概览"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/redis/:datasourceId/keys", Handler: rr.scanRedisKeys, Description: "Redis Key 扫描"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/redis/:datasourceId/key", Handler: rr.getRedisKeyDetail, Description: "Redis Key 详情"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/redis/:datasourceId/key", Handler: rr.createRedisKey, Description: "Redis 新增 Key"},
		apiregistry.RouteEntry{Method: "DELETE", RelativePath: "/redis/:datasourceId/key", Handler: rr.deleteRedisKey, Description: "Redis 删除 Key"},
		apiregistry.RouteEntry{Method: "DELETE", RelativePath: "/redis/:datasourceId/keys", Handler: rr.deleteRedisKeys, Description: "Redis 批量删除 Key"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/redis/:datasourceId/key", Handler: rr.updateRedisKeyValue, Description: "Redis 修改 Key 值"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/redis/:datasourceId/key/ttl", Handler: rr.setRedisKeyTTL, Description: "Redis 修改 Key TTL"},
	)
}
