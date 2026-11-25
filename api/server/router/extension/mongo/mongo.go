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

package mongo

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

// mongoRouter is a router to talk with the mongo controller
type mongoRouter struct {
	c controller.PixiuInterface
}

// RegisterMongo 将 MongoDB 子模块路由注册到 extension 父路由组下，
// 完整路径为 /pixiu/extension/mongo/...
func RegisterMongo(o *options.Options, group *apiregistry.Group) {
	mr := &mongoRouter{
		c: o.Controller,
	}
	group.Entries = append(group.Entries,
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/mongo/ping", Handler: mr.pingMongoAdhoc, Description: "MongoDB 临时连通性探测"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mongo/:datasourceId/ping", Handler: mr.pingMongo, Description: "MongoDB 连接探测"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mongo/:datasourceId/info", Handler: mr.getMongoInfo, Description: "MongoDB 实例概览"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mongo/:datasourceId/replset", Handler: mr.getMongoReplicaSet, Description: "MongoDB 副本集状态"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mongo/:datasourceId/slowlog", Handler: mr.listMongoSlowLogs, Description: "MongoDB 慢日志"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mongo/:datasourceId/params", Handler: mr.listMongoParams, Description: "MongoDB 参数配置"},
	)
}
