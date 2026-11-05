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

package clusterexpand

import (
	"context"
	"fmt"
	"os"
	"time"

	planctrl "github.com/caoyingjunz/pixiu/pkg/controller/plan"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/kubezansible"
	"github.com/caoyingjunz/pixiu/pkg/planrender"
	"github.com/caoyingjunz/pixiu/pkg/types"
	pixiutpl "github.com/caoyingjunz/pixiu/template"
)

// expandTask 扩容执行的共享上下文（对应部署计划的 handlerTask）。
type expandTask struct {
	plan    *model.Plan
	cluster *model.Cluster
	info    *clusterInfo
	options *types.ExpansionOptions
	nodes   []model.Node

	// 运行期产物
	dir    string
	runner string
}

// expandHandler 扩容步骤接口，执行循环见 syncExpansionTasks。
type expandHandler interface {
	Name() string
	Run(ctx context.Context, task *expandTask) error
}

// syncExpansionTasks 顺序执行扩容步骤：任一步失败即中断并收口整个扩容计划。
func (c *controller) syncExpansionTasks(ctx context.Context, planId int64, task *expandTask, handlers ...expandHandler) error {
	for _, handler := range handlers {
		c.markTaskRunning(ctx, planId, handler.Name())
		err := handler.Run(ctx, task)
		c.finishTask(ctx, planId, handler.Name(), err)
		if err != nil {
			c.failExpansionPlan(ctx, planId, err)
			return err
		}
	}
	return nil
}

// taskPrepare 前置准备：解析 runner 镜像。
type taskPrepare struct{ c *controller }

func (h taskPrepare) Name() string { return planctrl.TaskNamePrepare }
func (h taskPrepare) Run(ctx context.Context, task *expandTask) error {
	runner, err := h.c.resolveRunner(ctx, task.options.OSImage)
	if err != nil {
		return err
	}
	task.runner = runner
	return nil
}

// taskRender 配置渲染：解析 join 信息并渲染节点配置。
type taskRender struct{ c *controller }

func (h taskRender) Name() string { return planctrl.TaskNameRender }
func (h taskRender) Run(ctx context.Context, task *expandTask) error {
	join, err := h.c.resolveJoin(ctx, task.cluster, task.info, task.options)
	if err != nil {
		return err
	}

	planNodes, err := nodesToPlanNodes(task.nodes)
	if err != nil {
		return err
	}
	globals := pixiutpl.ExpandGlobals{
		KubeRelease:      join.KubeRelease,
		APIServer:        join.APIServer,
		Token:            join.Token,
		CAHash:           join.CAHash,
		NetworkInterface: task.options.NetworkInterface,
		SetHostname:      task.options.SetHostname,
		RuntimeDataDir:   task.options.RuntimeDataDir,
		EnableCustomRepo: task.options.CustomRepo != nil && task.options.CustomRepo.Enable,
	}
	// 重新触发扩容时清理旧渲染产物，保证 plan 目录内容为本次最新渲染结果
	if err = os.RemoveAll(task.dir); err != nil {
		return fmt.Errorf("清理扩容渲染目录失败: %v", err)
	}
	if err = planrender.RenderExpand(task.dir, planNodes, globals, task.options.CustomRepo); err != nil {
		return fmt.Errorf("渲染扩容配置失败: %v", err)
	}
	return nil
}

// taskPrechecks 部署预检查：确认集群可达且支持扩容。
type taskPrechecks struct{}

func (h taskPrechecks) Name() string { return planctrl.TaskNamePrechecks }
func (h taskPrechecks) Run(_ context.Context, task *expandTask) error {
	if task.info.Client == nil || !task.info.Capabilities.Supported {
		reason := task.info.Capabilities.Reason
		if reason == "" {
			reason = "集群不可达"
		}
		return fmt.Errorf("%s", reason)
	}
	return nil
}

// taskBootstrap 初始化部署环境：bootstrap-servers 为必经步骤。
type taskBootstrap struct {
	c       *controller
	timeout time.Duration
}

func (h taskBootstrap) Name() string { return planctrl.TaskNameBootstrap }
func (h taskBootstrap) Run(ctx context.Context, task *expandTask) error {
	return runRunnerContainer(ctx, fmt.Sprintf("%s-%d", kubezansible.Bootstrap, task.plan.Id), kubezansible.Bootstrap, task.runner, task.dir, h.timeout)
}

// taskAddNode 加入集群。
type taskAddNode struct {
	c       *controller
	timeout time.Duration
}

func (h taskAddNode) Name() string { return planctrl.TaskNameDeployNode }
func (h taskAddNode) Run(ctx context.Context, task *expandTask) error {
	return runRunnerContainer(ctx, fmt.Sprintf("%s-%d", kubezansible.AddNode, task.plan.Id), kubezansible.AddNode, task.runner, task.dir, h.timeout)
}
