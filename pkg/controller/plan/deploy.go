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
	"context"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/container"
)

type Deploy struct {
	handlerTask

	dir    string
	runner string
}

func (b Deploy) Name() string      { return "部署Master" }
func (b Deploy) GetAction() string { return "deploy" }
func (b Deploy) Run() error {
	cli, err := container.NewContainer(b.GetAction(), b.GetPlanId(), b.dir)
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	// 启动执行容器
	return cli.StartAndWaitForContainer(ctx, b.runner)
}

type AddMaster struct {
	handlerTask
}

func (b AddMaster) Name() string      { return "新增Master" }
func (b AddMaster) GetAction() string { return "add-master" }
func (b AddMaster) Run() error {
	return nil
}

type DeployNode struct {
	handlerTask
}

func (b DeployNode) Name() string      { return "部署Node" }
func (b DeployNode) GetAction() string { return "deploy-node" }
func (b DeployNode) Run() error {
	return nil
}

type AddNode struct {
	handlerTask
}

func (b AddNode) Name() string      { return "新增Node" }
func (b AddNode) GetAction() string { return "add-node" }
func (b AddNode) Run() error {
	return nil
}

type DeployChart struct {
	handlerTask
}

func (b DeployChart) Name() string         { return "部署基础组件" }
func (b DeployChart) GetAction() string    { return "deploy-chart" }
func (b DeployChart) Step() model.PlanStep { return model.CompletedPlanStep }
func (b DeployChart) Run() error {
	return nil
}
