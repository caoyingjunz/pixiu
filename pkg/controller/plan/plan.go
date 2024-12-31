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
	"net/http"
	"sync"

	"gorm.io/gorm"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/uuid"
)

type PlanGetter interface {
	Plan() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreatePlanRequest) error
	Update(ctx context.Context, planID int64, req *types.UpdatePlanRequest) error
	Delete(ctx context.Context, pid int64) error
	Get(ctx context.Context, pid int64) (*types.Plan, error)
	List(ctx context.Context) ([]types.Plan, error)

	GetWithSubResources(ctx context.Context, planId int64) (*types.Plan, error)

	// Start 启动部署任务
	Start(ctx context.Context, pid int64) error
	// Stop 终止部署任务
	Stop(ctx context.Context, pid int64) error

	CreateNode(ctx context.Context, pid int64, req *types.CreatePlanNodeRequest) error
	UpdateNode(ctx context.Context, pid int64, nodeId int64, req *types.UpdatePlanNodeRequest) error
	DeleteNode(ctx context.Context, pid int64, nodeId int64) error
	GetNode(ctx context.Context, pid int64, nodeId int64) (*types.PlanNode, error)
	ListNodes(ctx context.Context, pid int64) ([]types.PlanNode, error)

	CreateConfig(ctx context.Context, planId int64, req *types.CreatePlanConfigRequest) error
	UpdateConfig(ctx context.Context, pid int64, cfgId int64, req *types.UpdatePlanConfigRequest) error
	DeleteConfig(ctx context.Context, pid int64, cfgId int64) error
	GetConfig(ctx context.Context, planId int64) (*types.PlanConfig, error)

	// Run 启动 plan worker 处理协程
	Run(ctx context.Context, workers int) error

	RunTask(ctx context.Context, planId int64, taskId int64) error
	ListTasks(ctx context.Context, planId int64) ([]types.PlanTask, error)
	WatchTasks(ctx context.Context, planId int64, w http.ResponseWriter, r *http.Request)
	WatchTaskLog(ctx context.Context, planId int64, taskId int64, w http.ResponseWriter, r *http.Request) error
}

var taskQueue workqueue.RateLimitingInterface
var taskC *client.Task

func init() {
	taskQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "tasks")
	taskC = client.NewTaskCache()
}

type plan struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

// Create
// 1. 创建部署计划
// 2. 创建部署配置
// 3. 创建节点列表
// 4. 创建扩展组件
// 5. 创建容器服务
func (p *plan) Create(ctx context.Context, req *types.CreatePlanRequest) error {
	plan := &model.Plan{
		Name:        req.Name,
		Description: req.Description,
	}

	withConfig := func(plan *model.Plan, tx *gorm.DB) (*gorm.DB, error) {
		if err := p.preCreateConfig(ctx, plan.Id, &req.Config); err != nil {
			return nil, err
		}

		planConfig, err := p.buildPlanConfig(ctx, &req.Config)
		if err != nil {
			return nil, err
		}
		planConfig.PlanId = plan.Id

		if err := p.factory.Plan().TxCreateConfig(ctx, tx, planConfig); err != nil {
			klog.Errorf("failed to create plan(%s) config: %v", plan.Id, err)
			return nil, err
		}
		return tx, nil
	}

	withNodes := func(plan *model.Plan, tx *gorm.DB) (*gorm.DB, error) {
		for _, r := range req.Nodes {
			node, err := p.buildNodeFromRequest(plan.Id, &r)
			if err != nil {
				klog.Errorf("failed to build plan(%d) node from request: %v", plan.Id, err)
				return nil, err
			}

			if err := p.factory.Plan().TxCreateNode(ctx, tx, node); err != nil {
				klog.Errorf("failed to create node(%s): %v", r.Name, err)
				return nil, err
			}
		}
		return tx, nil
	}

	if _, err := p.factory.Plan().Create(ctx, plan, withConfig, withNodes); err != nil {
		klog.Errorf("failed to create plan %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}

	// 如果启用pixiu注册功能，则创建容器服务
	if req.Config.Kubernetes.Register {
		kubeNode := types.KubeNode{Ready: []string{}, NotReady: []string{}}
		nodes, _ := kubeNode.Marshal()
		_, err := p.factory.Cluster().Create(ctx, &model.Cluster{
			Name:        uuid.NewRandName(8),
			AliasName:   req.Name,
			Description: req.Description,
			ClusterType: model.ClusterTypeCustom,
			PlanId:      planId,
			Protected:   true,
			Nodes:       nodes,
		})
		if err != nil {
			klog.Errorf("failed to register cluster for plan: %v", err)
			_ = p.Delete(ctx, planId)
			return errors.ErrServerInternal
		}
	}
	return nil
}

// Update
// 更新部署计划
func (p *plan) Update(ctx context.Context, planId int64, req *types.UpdatePlanRequest) error {
	oldPlan, err := p.factory.Plan().Get(ctx, planId)
	if err != nil {
		klog.Errorf("failed to get plan(%d) %v", planId, err)
		return errors.ErrServerInternal
	}
	// 必要时更新 plan
	if oldPlan.Description != req.Description {
		if err := p.factory.Plan().Update(ctx, planId, *req.ResourceVersion, map[string]interface{}{"description": req.Description}); err != nil {
			klog.Errorf("failed to update plan %d: %v", planId, err)
			return errors.ErrServerInternal
		}
	}

	// 必要时更新部署计划配置
	if err = p.UpdateConfigIfNeeded(ctx, planId, req); err != nil {
		klog.Errorf("failed to update plan(%d) config: %v", planId, err)
		return errors.ErrServerInternal
	}

	// 必要时更新部署计划 nodes
	if err = p.updateNodesIfNeeded(ctx, planId, req); err != nil {
		klog.Errorf("failed to update plan(%d) nodes: %v", planId, err)
		return errors.ErrServerInternal
	}

	return nil
}

// 删除前检查
// 有正在运行中的任务则不允许删除
func (p *plan) preDelete(ctx context.Context, planId int64) error {
	isRunning, err := p.TaskIsRunning(ctx, planId)
	if err != nil {
		return errors.ErrServerInternal
	}
	if isRunning {
		return errors.ErrNotAcceptable
	}
	return nil
}

// Delete
// 1. 删除部署计划
// 2. 删除关联任务
// 3. 删除关联配置
// 4. 删除关联节点
func (p *plan) Delete(ctx context.Context, planId int64) error {
	// 删除前校验
	if err := p.preDelete(ctx, planId); err != nil {
		return err
	}

	// 执行实际的删除逻辑
	_, err := p.factory.Plan().Delete(ctx, planId)
	if err != nil {
		klog.Errorf("failed to delete plan %d: %v", planId, err)
		return errors.ErrServerInternal
	}
	// 删除 plan 关联资源
	// 2. 删除部署计划后，同步删除任务，删除任务失败时，可直接忽略
	if err = p.factory.Plan().DeleteTask(ctx, planId); err != nil {
		klog.Errorf("failed to delete plan(%d) task: %v", planId, err)
		return err
	}
	// 3. 删除关联配置
	if err = p.factory.Plan().DeleteConfigByPlan(ctx, planId); err != nil {
		klog.Errorf("failed to delete plan(%d) config: %v", planId, err)
		return err
	}
	// 4. 删除关联nodes
	if err = p.factory.Plan().DeleteNodesByPlan(ctx, planId); err != nil {
		klog.Errorf("failed to delete plan(%d) nodes: %v", planId, err)
		return err
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

// GetWithSubResources
// 获取 plan
// 获取 configs
// 获取 nodes
func (p *plan) GetWithSubResources(ctx context.Context, planId int64) (*types.Plan, error) {
	result, err := p.Get(ctx, planId)
	if err != nil {
		return nil, err
	}

	// 追加配置
	cfg, err := p.GetConfig(ctx, planId)
	if err != nil {
		return nil, err
	}
	result.Config = *cfg

	// 追加节点
	result.Nodes, err = p.ListNodes(ctx, planId)
	if err != nil {
		return nil, err
	}
	return result, nil
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

func (p *plan) SyncTaskStatus(ctx context.Context) error {
	plans, err := p.List(ctx)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(plans))
	for _, planP := range plans {
		wg.Add(1)
		go func(planId int64) {
			defer wg.Done()
			if err = p.syncStatus(ctx, planId); err != nil {
				errChan <- err
			}
		}(planP.Id)
	}
	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
	}

	return nil
}

// 启动前校验
// 1. 配置
// 2. 节点
// 3. 校验runner
// 3. 运行任务
func (p *plan) preStart(ctx context.Context, pid int64) error {
	// 1. 校验配置
	cfg, err := p.GetConfig(ctx, pid)
	if err != nil {
		return fmt.Errorf("failed to get plan(%d) config %v", pid, err)
	}
	// TODO: 根据具体情况对参数

	// 2. 校验节点
	nodes, err := p.ListNodes(ctx, pid)
	if err != nil {
		return fmt.Errorf("failed to get plan(%d) nodes %v", pid, err)
	}
	if len(nodes) == 0 {
		return fmt.Errorf("部署计划暂无关联节点")
	}

	// 3. 校验runner
	runner, err := p.GetRunner(cfg.OSImage)
	if err != nil {
		return err
	}
	klog.Infof("plan(%d) runner is %s", pid, runner)

	// 4. 校验运行任务
	isRunning, err := p.TaskIsRunning(ctx, pid)
	if err != nil {
		return errors.ErrServerInternal
	}
	if isRunning {
		return errors.ErrNotAcceptable
	}

	return nil
}

// TaskIsRunning
// 校验是否有任务正在运行
func (p *plan) TaskIsRunning(ctx context.Context, planId int64) (bool, error) {
	tasks, err := p.factory.Plan().ListTasks(ctx, planId)
	if err != nil {
		klog.Errorf("failed to get tasks of plan %d: %v", planId, err)
		return false, errors.ErrServerInternal
	}

	for _, task := range tasks {
		if task.Status == model.RunningPlanStatus {
			klog.Warningf("task %d of plan %d is running", task.Id, planId)
			return true, nil
		}
	}

	return false, nil
}

func (p *plan) Start(ctx context.Context, pid int64) error {
	// 启动前校验
	if err := p.preStart(ctx, pid); err != nil {
		return err
	}

	taskQueue.Add(pid)
	return nil
}

func (p *plan) Stop(ctx context.Context, pid int64) error {
	return nil
}

func (p *plan) model2Type(o *model.Plan) (*types.Plan, error) {
	status := model.SuccessPlanStatus

	// 尝试获取最新的任务状态
	// 获取失败也不中断返回
	if tasks, err := p.factory.Plan().ListTasks(context.TODO(), o.Id); err == nil {
		if len(tasks) == 0 {
			status = model.UnStartPlanStatus
		} else {
			for _, task := range tasks {
				if task.Status != model.SuccessPlanStatus {
					status = task.Status
					break
				}
			}
		}
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
		Step:        status,
	}, nil
}

func NewPlan(cfg config.Config, f db.ShareDaoFactory) *plan {
	return &plan{
		cc:      cfg,
		factory: f,
	}
}
