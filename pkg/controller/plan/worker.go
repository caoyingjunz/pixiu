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
	klog.Infof("Starting Plan Manager")
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
		DeployChart{handlerTask: task},
	}
	quit := make(chan struct{}, len(handlers))
	taskCache.SetQuitQueue(planId, quit)
	err = p.createPlanTasksIfNotExist(handlers...)
	if err != nil {
		klog.Errorf("failed to create plan(%d) tasks: %v", planId, err)
		return
	}

	taskQueue := make(chan interface{})
	taskCache.SetTaskQueue(planId, taskQueue)
	go func() {
		for _, handler := range handlers {
			taskQueue <- handler
		}
		close(taskQueue)
	}()
	go p.syncTasks(planId)
}

func (p *plan) createPlanTasksIfNotExist(tasks ...Handler) error {
	for _, task := range tasks {
		planId := task.GetPlanId()
		name := task.Name()
		step := task.Step()
		lastTask, err := p.factory.Plan().GetTaskByName(context.TODO(), planId, name)
		// 存在则直接返回
		if err == nil && lastTask != nil {
			// 从数据库获取历史状态并初始化缓存
			taskCache.SetTaskResults(planId, lastTask)
			continue
		}
		if err != nil {
			// 非不存在报错则报异常
			if !errors.IsRecordNotFound(err) {
				klog.Infof("failed to get plan(%d) tasks(%s) for first created: %v", planId, name, err)
				return err
			}
		}

		// 不存在记录则新建
		if newTask, err := p.factory.Plan().CreatTask(context.TODO(), &model.Task{
			Name:   name,
			PlanId: planId,
			Step:   step,
			Status: model.UnStartPlanStatus,
		}); err != nil {
			klog.Errorf("failed to init plan(%d) task(%s): %v", planId, name, err)
			return err
		} else {
			taskCache.SetTaskResults(planId, newTask)
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
func (p *plan) syncStatus(task *model.Task) error {
	if err := p.factory.Plan().UpdateTask(context.TODO(), task.PlanId, task.Name, map[string]interface{}{
		"status": task.Status, "message": task.Message,
		"start_at": task.StartAt, "end_at": task.EndAt,
	}); err != nil {
		return err
	}
	return nil
}

func (p *plan) syncTasks(planId int64) {
	quit, ok := taskCache.GetQuitQueue(planId)
	if !ok {
		return
	}
	taskQueue, ok := taskCache.GetTaskQueue(planId)
	if !ok {
		return
	}
	for {
		res, ok := <-taskQueue
		if !ok {
			klog.Infof("plan(%d) tasks all completed", planId)
			taskCache.ClearPlanResults(planId)
			quit <- struct{}{}
			taskCache.CloseQuitQueue(planId)
			break
		}

		task := res.(Handler)
		if err := p.handlerTask(task); err != nil {
			taskCache.ClearPlanResults(planId)
			klog.Errorf("%d 计划 failed to handle task(%s): %v", planId, task.Name(), err)
			quit <- struct{}{}
			taskCache.CloseQuitQueue(planId)
			break
		}
	}
}

func (p *plan) handlerTask(task Handler) error {
	planId := task.GetPlanId()
	name := task.Name()
	// 获取当前任务缓存信息
	taskResult, ok := taskCache.GetTaskResults(planId, name)
	if !ok {
		return fmt.Errorf("failed to get plan(%d) task(%s) cache", planId, name)
	}
	// 设置启动任务时间
	taskResult.StartAt = time.Now()
	taskResult.Status = model.RunningPlanStatus
	step := task.Step()
	klog.Infof("starting plan(%d) task(%s)", planId, name)
	if err := p.syncStatus(taskResult); err != nil {
		return err
	}

	runErr := task.Run()
	if runErr != nil {
		step = model.FailedPlanStep
		taskResult.Message = runErr.Error()
		taskResult.EndAt = time.Now()
		taskResult.Status = model.FailedPlanStatus
		taskResult.Step = step
		klog.Errorf("failed plan(%d) task(%s),result: %v", planId, name, taskResult)
		if err := p.syncStatus(taskResult); err != nil {
			return err
		}
		return runErr
	}

	taskResult.EndAt = time.Now()
	taskResult.Status = model.SuccessPlanStatus
	taskResult.Step = step
	klog.Infof("completed plan(%d) task(%s),result: %v", planId, name, taskResult)
	if err := p.syncStatus(taskResult); err != nil {
		return err
	}
	return nil
}
