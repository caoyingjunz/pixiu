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

package autoscaling

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

// autoscalingRouter is a router to talk with the autoscaling controller
type autoscalingRouter struct {
	c controller.PixiuInterface
}

// RegisterAutoscaling 将定时扩缩容（CronHPA）子模块路由注册到 extension 父路由组下，
// 完整路径为 /pixiu/extension/autoscaling/...
func RegisterAutoscaling(o *options.Options, group *apiregistry.Group) {
	ar := &autoscalingRouter{
		c: o.Controller,
	}
	group.Entries = append(group.Entries,
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/autoscaling/cronhpas", Handler: ar.createCronHpa, Description: "创建定时扩缩容规则"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/autoscaling/cronhpas", Handler: ar.listCronHpas, Description: "定时扩缩容规则列表"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/autoscaling/cronhpas/:cronHpaId", Handler: ar.getCronHpa, Description: "定时扩缩容规则详情"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/autoscaling/cronhpas/:cronHpaId", Handler: ar.updateCronHpa, Description: "更新定时扩缩容规则"},
		apiregistry.RouteEntry{Method: "DELETE", RelativePath: "/autoscaling/cronhpas/:cronHpaId", Handler: ar.deleteCronHpa, Description: "删除定时扩缩容规则"},
		apiregistry.RouteEntry{Method: "PUT", RelativePath: "/autoscaling/cronhpas/:cronHpaId/status", Handler: ar.setCronHpaStatus, Description: "暂停/恢复定时扩缩容规则"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/autoscaling/cronhpas/:cronHpaId/histories", Handler: ar.listCronHpaHistories, Description: "定时扩缩容执行历史"},
	)
}
