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

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Handler interface {
	GetPlanId() int64

	Name() string         // 检查项名称
	Step() model.PlanStep // 未开始，运行中，异常和完成
	Run() error           // 执行
}

type handlerTask struct {
	data TaskData
}

func (t handlerTask) GetPlanId() int64     { return t.data.PlanId }
func (t handlerTask) Step() model.PlanStep { return model.RunningPlanStep }

func newHandlerTask(data TaskData) handlerTask {
	return handlerTask{data: data}
}

func (p *plan) Run(ctx context.Context, workers int) error {
	// 进程启动时，尝试同步任务状态
	klog.Infof("starting to sync task manager")
	if err := p.SyncTaskStatus(ctx); err != nil {
		return err
	}

	// 启动部署计划控制器
	klog.Infof("starting plan manager")
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, p.worker, time.Second)
	}
	return nil
}

func (p *plan) worker(ctx context.Context) {
	for p.process(ctx) {
	}
}

func (p *plan) process(ctx context.Context) bool {
	key, quit := taskQueue.Get()
	if quit {
		return false
	}
	defer taskQueue.Done(key)

	p.syncHandler(ctx, key.(int64))
	return true
}

type TaskData struct {
	PlanId int64
	Config *model.Config
	Nodes  []model.Node
}

func (t TaskData) validate() error {
	return nil
}

func (p *plan) getTaskData(ctx context.Context, planId int64) (TaskData, error) {
	nodes, err := p.factory.Plan().ListNodes(ctx, planId)
	if err != nil {
		return TaskData{}, err
	}
	cfg, err := p.factory.Plan().GetConfigByPlan(ctx, planId)
	if err != nil {
		return TaskData{}, err
	}

	return TaskData{
		PlanId: planId,
		Config: cfg,
		Nodes:  nodes,
	}, nil
}

// 实际处理函数
// 处理步骤:
// 1. 检查部署参数是否符合要求
// 2. 渲染环境
// 3. 执行部署
// 4. 部署后环境清理
func (p *plan) syncHandler(ctx context.Context, planId int64) {
	klog.Infof("starting plan(%d) task", planId)
	defer klog.Infof("completed plan(%d) task", planId)

	taskData, err := p.getTaskData(ctx, planId)
	if err != nil {
		klog.Errorf("failed to get task data: %v", err)
		return
	}
	runner, err := p.GetRunner(taskData.Config.OSImage)
	if err != nil {
		klog.Errorf("failed to get image(%s) for worker: %v", taskData.Config.OSImage, err)
		return
	}
	// Runner的工作目录
	dir := p.WorkDir()

	task := newHandlerTask(taskData)
	handlers := []Handler{
		Check{handlerTask: task},
		Render{handlerTask: task, dir: dir},
		BootStrap{handlerTask: task, dir: dir, runner: runner},
		Deploy{handlerTask: task, dir: dir, runner: runner},
		DeployNode{handlerTask: task},
		Register{handlerTask: task, factory: p.factory},
		DeployChart{handlerTask: task},
	}
	if err = p.syncTasks(handlers...); err != nil {
		klog.Errorf("failed to sync task: %v", err)
	}
}

func (p *plan) createPlanTasksIfNotExist(tasks ...Handler) error {
	for _, task := range tasks {
		planId := task.GetPlanId()
		name := task.Name()
		step := task.Step()

		_, err := p.factory.Plan().GetTaskByName(context.TODO(), planId, name)
		// 存在则直接返回
		if err == nil {
			return nil
		}

		// 非不存在报错则报异常
		if !errors.IsRecordNotFound(err) {
			klog.Infof("failed to get plan(%d) tasks(%s) for first created: %v", planId, name, err)
			return err
		}

		// 不存在记录则新建
		if _, err = p.factory.Plan().CreateTask(context.TODO(), &model.Task{
			Name:   name,
			PlanId: planId,
			Step:   step,
			Status: model.UnStartPlanStatus,
		}); err != nil {
			klog.Errorf("failed to init plan(%d) task(%s): %v", planId, name, err)
			return err
		}
	}

	return nil
}

func (p *plan) WorkDir() string {
	return p.cc.Worker.WorkDir
}

func (p *plan) GetRunner(osImage string) (string, error) {
	engines := p.cc.Worker.Engines
	for _, engine := range engines {
		for _, os := range engine.OSSupported {
			if os == osImage {
				return engine.Image, nil
			}
		}
	}
	return "", fmt.Errorf("osImage(%s) runner not found", osImage)
}

// 同步任务状态
// 任务启动时设置为运行中，结束时同步为结束状态(成功或者失败)
// TODO: 后续优化，判断对应部署容器是否在运行，根据容器的运行结果同步状态
func (p *plan) syncStatus(ctx context.Context, planId int64) error {
	tasks, err := p.factory.Plan().ListTasks(ctx, planId)
	if err != nil {
		return err
	}

	for _, task := range tasks {
		if task.Status != model.RunningPlanStatus {
			continue
		}
		if _, err = p.factory.Plan().UpdateTask(ctx, planId, task.Name, map[string]interface{}{
			"status": model.FailedPlanStatus, "step": model.FailedPlanStep, "message": "服务异常修正，请重新启动部署计划", "gmt_modified": time.Now(),
		}); err != nil {
			klog.Errorf("failed to update plan(%d) status: %v", planId, err)
			return err
		}
	}
	return nil
}

func (p *plan) syncTasks(tasks ...Handler) error {
	// 初始化记录
	if err := p.createPlanTasksIfNotExist(tasks...); err != nil {
		return err
	}

	// 执行任务并更新状态
	for _, task := range tasks {
		planId := task.GetPlanId()
		name := task.Name()
		klog.Infof("starting plan(%d) task(%s)", planId, name)

		// TODO: 通过闭包方式优化
		start, err := p.factory.Plan().UpdateTask(context.TODO(), planId, name, map[string]interface{}{
			"status": model.RunningPlanStatus, "message": "", "gmt_create": time.Now(),
		})
		if err != nil {
			klog.Errorf("failed to update plan(%d) status before run task(%s): %v", planId, name, err)
			return err
		}
		taskC.SetByTask(planId, *start)

		status := model.SuccessPlanStatus
		step := task.Step()
		message := ""

		// 执行检查
		runErr := task.Run()
		if runErr != nil {
			status = model.FailedPlanStatus
			step = model.FailedPlanStep
			message = runErr.Error()
		}

		// 执行完成之后更新状态
		end, err := p.factory.Plan().UpdateTask(context.TODO(), planId, name, map[string]interface{}{
			"status": status, "message": message, "step": step, "gmt_modified": time.Now(),
		})
		if err != nil {
			klog.Errorf("failed to update plan(%d) status after run task(%s): %v", planId, name, err)
			return err
		}
		taskC.SetByTask(planId, *end)

		klog.Infof("completed plan(%d) task(%s)", planId, name)
		if runErr != nil {
			return runErr
		}
	}

	return nil
}
