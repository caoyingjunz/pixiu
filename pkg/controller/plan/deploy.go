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
	"fmt"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/runtime"
)

type DeployMaster struct {
	handlerTask

	dir         string
	runner      string
	waitTimeout time.Duration
}

// runRunnerContainer 以宿主容器运行时拉起 runner 任务容器并等待退出。
// waitTimeout 为等待容器退出的最长时长，来自 worker.deploy_timeout 配置(秒)；
// <=0 时表示不设内部上限（等待容器退出或 ctx 结束）。
// 日志路径由运行时按容器名自行决定（固定 /var/lib/pixiu/runner-logs/<name>.log），调用方不再传入。
func runRunnerContainer(ctx context.Context, action string, planId int64, dir, image string, waitTimeout time.Duration) error {
	name := fmt.Sprintf("%s-%d", action, planId)
	rt, err := runtime.Default()
	if err != nil {
		return err
	}
	return rt.RunContainer(ctx, &runtime.ContainerSpec{
		Name:        name,
		Image:       image,
		Env:         []string{fmt.Sprintf("COMMAND=%s", action)},
		Binds:       []string{fmt.Sprintf("%s/%d:/configs", dir, planId)},
		NetworkHost: true,
		WaitTimeout: waitTimeout,
		Labels:      map[string]string{"author": "caoyingjunz", "pixiuName": name},
	})
}

func (b DeployMaster) Name() string      { return "部署Master" }
func (b DeployMaster) GetAction() string { return "deploy-master" }
func (b DeployMaster) Run() error {
	// ctx 比容器等待超时多留 60s 余量，避免 ctx 先过期导致无意义报错
	ctx, cancel := context.WithTimeout(context.Background(), b.waitTimeout+60*time.Second)
	defer cancel()

	// 启动执行容器
	return runRunnerContainer(ctx, b.GetAction(), b.GetPlanId(), b.dir, b.runner, b.waitTimeout)
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

	dir         string
	runner      string
	waitTimeout time.Duration
}

func (b DeployNode) Name() string      { return "部署Node" }
func (b DeployNode) GetAction() string { return "deploy-node" }
func (b DeployNode) Run() error {
	// ctx 比容器等待超时多留 60s 余量，避免 ctx 先过期导致无意义报错
	ctx, cancel := context.WithTimeout(context.Background(), b.waitTimeout+60*time.Second)
	defer cancel()

	// 启动执行容器
	return runRunnerContainer(ctx, b.GetAction(), b.GetPlanId(), b.dir, b.runner, b.waitTimeout)
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

	dir         string
	runner      string
	waitTimeout time.Duration
}

func (b DeployChart) Name() string         { return "部署基础组件" }
func (b DeployChart) GetAction() string    { return "apply" }
func (b DeployChart) Step() model.PlanStep { return model.RunningPlanStep }
func (b DeployChart) Run() error {
	// ctx 比容器等待超时多留 60s 余量，避免 ctx 先过期导致无意义报错
	ctx, cancel := context.WithTimeout(context.Background(), b.waitTimeout+60*time.Second)
	defer cancel()

	// 启动执行容器
	return runRunnerContainer(ctx, b.GetAction(), b.GetPlanId(), b.dir, b.runner, b.waitTimeout)
}
