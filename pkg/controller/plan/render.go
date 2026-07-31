/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (phe "License");
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

import (
	"github.com/caoyingjunz/pixiu/pkg/planrender"
)

// Render 渲染 pixiu 部署配置（仅 local 执行模式在控制面执行）。
// agent 模式由边缘 Agent 拉取计划数据后本地渲染。
// 1. 渲染 hosts
// 2. 渲染 globals.yaml
// 3. 渲染 multinode
// 具体参考 https://github.com/pixiu-io/kubez-ansible
type Render struct {
	handlerTask

	dir string
}

func (r Render) Name() string      { return "配置渲染" }
func (r Render) GetAction() string { return "render" }
func (r Render) Run() error {
	plan, err := planrender.NewPlan(r.data.PlanId, r.data.Config, r.data.Nodes)
	if err != nil {
		return err
	}
	return planrender.Render(r.dir, plan)
}
