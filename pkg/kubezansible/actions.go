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

// Package kubezansible 定义 kubez-ansible CLI 的 action 名称，属于外部工具契约。
// 参考 https://github.com/pixiu-io/kubez-ansible/blob/master/tools/kubez-ansible
package kubezansible

const (
	Runner    = "runner"
	Render    = "render"
	Prechecks = "prechecks"

	Bootstrap = "bootstrap-servers"

	DeployMaster = "deploy-master"
	AddMaster    = "add-master"
	DeployNode   = "deploy-node"

	AddNode = "add-node"

	Apply    = "apply"
	Register = "register"
)
