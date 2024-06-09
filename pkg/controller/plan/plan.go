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

	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type PlanGetter interface {
	Plan() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreatePlanRequest) error
	Update(ctx context.Context, pid int64, req *types.UpdatePlanRequest) error
	Delete(ctx context.Context, pid int64) error
	Get(ctx context.Context, pid int64) (*types.Plan, error)
	List(ctx context.Context) ([]types.Plan, error)

	// Start 启动部署任务
	Start(ctx context.Context, pid int64) error
	// Stop 终止部署任务
	Stop(ctx context.Context, pid int64) error

	CreateNode(ctx context.Context, pid int64, req *types.CreatePlanNodeRequest) error
	UpdateNode(ctx context.Context, pid int64, nodeId int64, req *types.UpdatePlanNodeRequest) error
	DeleteNode(ctx context.Context, pid int64, nodeId int64) error
	GetNode(ctx context.Context, pid int64, nodeId int64) (*types.PlanNode, error)
	ListNodes(ctx context.Context, pid int64) ([]types.PlanNode, error)

	CreateConfig(ctx context.Context, pid int64, req *types.CreatePlanConfigRequest) error
	UpdateConfig(ctx context.Context, pid int64, cfgId int64, req *types.UpdatePlanConfigRequest) error
	DeleteConfig(ctx context.Context, pid int64, cfgId int64) error
	GetConfig(ctx context.Context, pid int64, cfgId int64) (*types.PlanConfig, error)

	// Run 启动 worker 处理协程
	Run(ctx context.Context, workers int) error

	RunTask(ctx context.Context, planId int64, taskId int64) error
	ListTasks(ctx context.Context, planId int64) ([]types.PlanTask, error)
}

var taskQueue workqueue.RateLimitingInterface

func init() {
	taskQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "tasks")
}

type plan struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

// Create
// 1. 创建部署计划
// 2. 创建部署任务
// 3. 创建部署配置
func (p *plan) Create(ctx context.Context, req *types.CreatePlanRequest) error {
	_, err := p.factory.Plan().Create(ctx, &model.Plan{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		klog.Errorf("failed to create plan %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}

	// TODO: 创建部署配置
	return nil
}

func (p *plan) Update(ctx context.Context, pid int64, req *types.UpdatePlanRequest) error {
	updates := make(map[string]interface{})

	if err := p.factory.Plan().Update(ctx, pid, req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update plan %d: %v", pid, err)
		return errors.ErrServerInternal
	}
	return nil
}

// Delete
// TODO: 删除前校验
func (p *plan) Delete(ctx context.Context, pid int64) error {
	_, err := p.factory.Plan().Delete(ctx, pid)
	if err != nil {
		klog.Errorf("failed to delete plan %d: %v", pid, err)
		return errors.ErrServerInternal
	}

	// 删除部署计划后，同步删除任务，删除任务失败时，可直接忽略
	err = p.factory.Plan().DeleteTask(ctx, pid)
	if err != nil {
		klog.Errorf("failed to delete plan(%d) task: %v", pid, err)
	}
	return nil
}

func (p *plan) Get(ctx context.Context, pid int64) (*types.Plan, error) {
	object, err := p.factory.Plan().Get(ctx, pid)
	if err != nil {
		klog.Errorf("failed to get plan %d: %v", pid, err)
		return nil, errors.ErrServerInternal
	}

	return p.model2Type(object)
}

func (p *plan) List(ctx context.Context) ([]types.Plan, error) {
	objects, err := p.factory.Plan().List(ctx)
	if err != nil {
		klog.Errorf("failed to get plans: %v", err)
		return nil, errors.ErrServerInternal
	}

	var ps []types.Plan
	for _, object := range objects {
		no, err := p.model2Type(&object)
		if err != nil {
			return nil, err
		}
		ps = append(ps, *no)
	}
	return ps, nil
}

func (p *plan) Start(ctx context.Context, pid int64) error {
	tasks, err := p.factory.Plan().ListTasks(ctx, pid)
	if err != nil {
		klog.Errorf("failed to get tasks of plan %d: %v", pid, err)
		return errors.ErrServerInternal
	}

	for _, task := range tasks {
		if task.Status == model.RunningPlanStatus {
			klog.Warningf("task %d of plan %d is already running", task.Id, pid)
			return errors.ErrNotAcceptable
		}
	}

	taskQueue.Add(pid)
	return nil
}

func (p *plan) Stop(ctx context.Context, pid int64) error {
	return nil
}

func (p *plan) model2Type(o *model.Plan) (*types.Plan, error) {
	step := model.UnStartedPlanStep

	// 尝试获取最新的任务状态
	// 获取失败也不中断返回
	newestTask, err := p.factory.Plan().GetNewestTask(context.TODO(), o.Id)
	if err == nil {
		step = newestTask.Step
	}

	return &types.Plan{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:        o.Name,
		Description: o.Description,
		Step:        step,
	}, nil
}

func NewPlan(cfg config.Config, f db.ShareDaoFactory) *plan {
	return &plan{
		cc:      cfg,
		factory: f,
	}
}
