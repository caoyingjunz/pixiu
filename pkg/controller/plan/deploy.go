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

func (b Deploy) Name() string { return "部署Master" }

// Run 以容器的形式执行 BootStrap 任务，如果存在旧的容器，则先删除在执行
func (b Deploy) Run() error {
	cli, err := container.NewContainer("deploy", b.GetPlanId(), b.dir)
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	// 启动执行容器
	if err = cli.StartAndWaitForContainer(ctx, b.runner); err != nil {
		return err
	}

	return nil
}

type DeployNode struct {
	handlerTask
}

func (b DeployNode) Name() string { return "部署Node" }

// Run 以容器的形式执行 BootStrap 任务，如果存在旧的容器，则先删除在执行
func (b DeployNode) Run() error {
	return nil
}

type DeployChart struct {
	handlerTask
}

func (b DeployChart) Name() string         { return "部署基础组件" }
func (b DeployChart) Step() model.PlanStep { return model.CompletedPlanStep }

// Run 以容器的形式执行 BootStrap 任务，如果存在旧的容器，则先删除在执行
func (b DeployChart) Run() error {
	return nil
}
