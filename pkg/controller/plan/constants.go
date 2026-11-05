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

package plan

// 计划任务展示名，部署计划与集群扩容进度共用。
const (
	TaskNamePrepare    = "前置准备"
	TaskNameRender     = "配置渲染"
	TaskNamePrechecks  = "部署预检查"
	TaskNameBootstrap  = "初始化部署环境"
	TaskNameDeployNode = "部署Node"
	TaskNameAddNode    = "新增Node"
)
