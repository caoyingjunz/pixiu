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
	defer klog.Infof("completed plan(%d) task", planId)

	taskData, err := p.getTaskData(ctx, planId)
	if err != nil {
		klog.Errorf("failed to get task data: %v", err)
		return
	}

	task := newHandlerTask(taskData)
	handlers := []Handler{
		Check{handlerTask: task},
		Render{handlerTask: task},
		BootStrap{handlerTask: task},
		Deploy{handlerTask: task},
	}
	if err = p.syncTasks(handlers...); err != nil {
		klog.Errorf("failed to sync task: %v", err)
	}
}

func (p *plan) initTasks(tasks ...Handler) error {
	for _, task := range tasks {
		planId := task.GetPlanId()
		name := task.Name()
		step := task.Step()

		object, err := p.factory.Plan().GetTaskByName(context.TODO(), planId, name)
		if err != nil {
			if !errors.IsRecordNotFound(err) {
				return err
			}

			// 不存在记录则新建
			object, err = p.factory.Plan().CreatTask(context.TODO(), &model.Task{
				Name:   name,
				PlanId: planId,
				Step:   step,
				Status: model.RunningPlanStatus,
			})
			if err != nil {
				klog.Errorf("failed to init plan(%d) task(%s): %v", object.PlanId, name, err)
				return err
			}
		} else {
			// 存在的情况下，如果状态不是运行中，则更新成运行状态
			if object.Status != model.RunningPlanStatus {
				// 如果对象已经存在，则更新状态为运行中
				if err = p.factory.Plan().UpdateTask(context.TODO(), object.PlanId, object.ResourceVersion, map[string]interface{}{
					"status":  model.RunningPlanStatus,
					"message": "",
				}); err != nil {
					klog.Errorf("failed to update init plan(%d) task(%s): %v", object.PlanId, name, err)
					return err
				}
			}
		}
	}

	return nil
}

func (p *plan) syncTasks(tasks ...Handler) error {
	// 初始化记录
	if err := p.initTasks(tasks...); err != nil {
		return err
	}

	// 执行任务并更新状态
	for _, task := range tasks {
		planId := task.GetPlanId()
		name := task.Name()

		object, err := p.factory.Plan().GetTaskByName(context.TODO(), planId, name)
		if err != nil {
			return err
		}

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
		if err = p.factory.Plan().UpdateTask(context.TODO(), object.PlanId, object.ResourceVersion, map[string]interface{}{
			"status":  status,
			"message": message,
			"step":    step,
		}); err != nil {
			klog.Errorf("failed to update plan(%d) task(%s): %v", object.PlanId, name, err)
			return err
		}

		if runErr != nil {
			return runErr
		}
	}

	return nil
}
