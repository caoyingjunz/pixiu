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
	"github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
	"github.com/caoyingjunz/pixiu/pkg/util/token"
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
	List(ctx context.Context, listOption types.ListOptions) (interface{}, error)

	GetWithSubResources(ctx context.Context, planId int64) (*types.Plan, error)

	// Start 启动部署任务
	Start(ctx context.Context, pid int64) error
	// Destroy 销毁k8s集群
	Destroy(ctx context.Context, pid int64, restart bool) error

	ListNodes(ctx context.Context, pid int64) ([]types.PlanNode, error)

	// Run 启动 plan worker 处理协程
	Run(ctx context.Context, workers int) error

	ListTasks(ctx context.Context, planId int64) ([]types.PlanTask, error)
	WatchTasks(ctx context.Context, planId int64, w http.ResponseWriter, r *http.Request)
	WatchTaskLog(ctx context.Context, planId int64, taskId int64, w http.ResponseWriter, r *http.Request) error

	// Config 计划配置子接口
	Config() ConfigInterface
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

func (p *plan) preCreate(ctx context.Context, uid int64, req *types.CreatePlanRequest) error {
	plans, err := p.factory.Plan().List(ctx, db.WithUser(uid))
	if err != nil {
		klog.Errorf("list plans err: %v", err)
		return err
	}

	for _, pp := range plans {
		if pp.Name == req.Name {
			return fmt.Errorf("部署集群 %s 已存在", pp.Name)
		}
	}

	if req.ExecMode == model.PlanExecModeAgent {
		if req.DeployAgentId == 0 {
			return fmt.Errorf("agent 模式必须指定 deploy_agent_id")
		}
		agent, err := p.factory.Agent().Get(ctx, req.DeployAgentId)
		if err != nil {
			return err
		}
		if agent == nil {
			return fmt.Errorf("deploy agent %d 不存在", req.DeployAgentId)
		}
	}

	return nil
}

// Create
// 1. 创建部署计划
// 2. 创建部署配置
// 3. 创建节点列表
// 4. 创建扩展组件
// 5. 创建容器服务
func (p *plan) Create(ctx context.Context, req *types.CreatePlanRequest) error {
	if err := p.preCreate(ctx, req.UserId, req); err != nil {
		return err
	}

	// 为节点增加用户 id 属性
	for i := range req.Nodes {
		req.Nodes[i].UserId = req.UserId
	}

	planModel := &model.Plan{
		Name:              req.Name,
		UserId:            req.UserId,
		Description:       req.Description,
		ExecMode:          req.ExecMode,
		DeployAgentId:     req.DeployAgentId,
		KubernetesVersion: req.Config.Kubernetes.KubernetesVersion,
		NodeCount:         len(req.Nodes),
		Status:            model.UnStartPlanStatus,
	}
	if planModel.ExecMode == "" {
		planModel.ExecMode = model.PlanExecModeLocal
	}

	createdPlan, err := p.factory.Plan().Create(ctx, planModel, p.createPlanSubResources(ctx, req))
	if err != nil {
		klog.Errorf("failed to create plan %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}
	planId := createdPlan.Id

	// 如果启用pixiu注册功能，则创建容器服务
	kubeNode := types.KubeNode{Ready: []string{}, NotReady: []string{}}
	nodes, _ := kubeNode.Marshal()
	obj := &model.Cluster{
		Name:          uuid.NewRandName(8),
		AliasName:     req.Name,
		ClusterType:   model.ClusterTypeCustom,
		PlanId:        planId,
		UserId:        req.UserId,
		ClusterStatus: model.ClusterStatusUnStart,
		Protected:     true,
		Nodes:         nodes,
	}

	// agent 模式（单向网络）使用反向隧道，而非直连 apiServer
	if planModel.ExecMode == model.PlanExecModeAgent {
		agentToken, tokenErr := token.Generate()
		if tokenErr != nil {
			klog.Errorf("failed to generate agent token for plan %s: %v", req.Name, tokenErr)
			_ = p.Delete(ctx, planId)
			return errors.ErrServerInternal
		}
		obj.AgentToken = agentToken
		obj.ConnectMode = model.ConnectModeTunnel
	}
	_, err = p.factory.Cluster().Create(ctx, obj)
	if err != nil {
		klog.Errorf("failed to register cluster for plan: %v", err)
		_ = p.Delete(ctx, planId)
		return errors.ErrServerInternal
	}
	return nil
}

func (p *plan) createPlanSubResources(ctx context.Context, req *types.CreatePlanRequest) db.CreatePlanOption {
	return func(planModel *model.Plan, tx *gorm.DB) (*gorm.DB, error) {
		if err := p.preCreateConfig(ctx, planModel.Id, &req.Config); err != nil {
			return nil, err
		}

		planConfig, err := p.buildPlanConfig(ctx, &req.Config)
		if err != nil {
			return nil, err
		}
		planConfig.PlanId = planModel.Id

		if err := p.factory.Plan().TxCreateConfig(ctx, tx, planConfig); err != nil {
			klog.Errorf("failed to create plan(%d) config: %v", planModel.Id, err)
			return nil, err
		}

		for i := range req.Nodes {
			nodeReq := &req.Nodes[i]
			node, err := p.buildNodeFromRequest(ctx, planModel.Id, nodeReq)
			if err != nil {
				klog.Errorf("failed to build plan(%d) node from request: %v", planModel.Id, err)
				return nil, err
			}
			if err := p.factory.Plan().TxCreateNode(ctx, tx, node); err != nil {
				klog.Errorf("failed to create node(%s): %v", nodeReq.Name, err)
				return nil, err
			}
		}

		return tx, nil
	}
}

// 更新前置检查：资源存在 + 非超级管理员只能更新自己的部署计划
func (p *plan) preUpdate(ctx context.Context, planId int64) (*model.Plan, error) {
	oldPlan, err := p.factory.Plan().Get(ctx, planId)
	if err != nil {
		klog.Errorf("failed to get plan(%d): %v", planId, err)
		return nil, errors.ErrServerInternal
	}
	if oldPlan == nil {
		return nil, errors.ErrServerInternal
	}
	if err := util.CheckResourceAccess(ctx, p.factory, oldPlan.UserId, types.ResourceTypePlan, planId); err != nil {
		return nil, err
	}
	return oldPlan, nil
}

// Update
// 更新部署计划
// TODO: 实现太过丑陋，后续优化
func (p *plan) Update(ctx context.Context, planId int64, req *types.UpdatePlanRequest) error {
	oldPlan, err := p.preUpdate(ctx, planId)
	if err != nil {
		klog.Errorf("pre-update check failed for plan(%d): %v", planId, err)
		return err
	}

	execMode := req.ExecMode
	if execMode == "" {
		execMode = model.PlanExecModeLocal
	}
	deployAgentId := req.DeployAgentId
	if execMode == model.PlanExecModeLocal {
		deployAgentId = 0
	}
	if execMode == model.PlanExecModeAgent {
		if deployAgentId == 0 {
			return fmt.Errorf("agent 模式必须指定 deploy_agent_id")
		}
		agent, err := p.factory.Agent().Get(ctx, deployAgentId)
		if err != nil {
			return err
		}
		if agent == nil {
			return fmt.Errorf("deploy agent %d 不存在", deployAgentId)
		}
	}

	for i := range req.Nodes {
		req.Nodes[i].UserId = oldPlan.UserId
	}

	updates := make(map[string]interface{})
	// 必要时更新 plan
	if oldPlan.Description != req.Description {
		updates["description"] = req.Description
	}
	if oldPlan.Name != req.Name {
		updates["name"] = req.Name
	}
	if oldPlan.ExecMode != execMode {
		updates["exec_mode"] = execMode
	}
	if oldPlan.DeployAgentId != deployAgentId {
		updates["deploy_agent_id"] = deployAgentId
	}
	// 同步冗余字段：k8s 版本来自配置，节点数来自本次节点列表
	if oldPlan.KubernetesVersion != req.Config.Kubernetes.KubernetesVersion {
		updates["kubernetes_version"] = req.Config.Kubernetes.KubernetesVersion
	}
	if oldPlan.NodeCount != len(req.Nodes) {
		updates["node_count"] = len(req.Nodes)
	}
	if len(updates) != 0 {
		if err = p.factory.Plan().Update(ctx, planId, *req.ResourceVersion, updates); err != nil {
			klog.Errorf("failed to update plan %d: %v", planId, err)
			return errors.ErrServerInternal
		}
	}

	// 切换为 agent 模式时，同步关联集群为隧道连接
	if oldPlan.ExecMode != execMode && execMode == model.PlanExecModeAgent {
		if err = p.syncClusterToTunnel(ctx, planId); err != nil {
			klog.Errorf("failed to sync cluster connect mode for plan(%d): %v", planId, err)
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
// 1. 资源存在（不存在返回 404）
// 2. 非超级管理员只能删除自己的部署计划（403）
// 3. 有正在运行中的任务则不允许删除（409）
func (p *plan) preDelete(ctx context.Context, planId int64) error {
	planObj, err := p.factory.Plan().Get(ctx, planId)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return errors.ErrPlanNotFound
		}
		klog.Errorf("failed to get plan(%d): %v", planId, err)
		return errors.ErrServerInternal
	}
	if planObj == nil {
		return errors.ErrPlanNotFound
	}
	if err = util.CheckResourceAccess(ctx, p.factory, planObj.UserId, types.ResourceTypePlan, planId); err != nil {
		return err
	}

	// 有正在运行中的任务则不允许删除
	isRunning, err := p.IsRunning(ctx, planId)
	if err != nil {
		return errors.ErrServerInternal
	}
	if isRunning {
		return errors.ErrConflict
	}
	return nil
}

// Delete
// 1. 删除部署计划
// 2. 删除关联任务
// 3. 删除关联配置
// 4. 删除关联节点
func (p *plan) Delete(ctx context.Context, planId int64) error {
	// 删除前校验：存在性 + owner + 运行中任务
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
	if object == nil {
		return nil, errors.ErrServerInternal
	}

	// 非超级管理员只能查看自己的部署计划或被 scope 授权的部署计划
	if err := util.CheckResourceAccess(ctx, p.factory, object.UserId, types.ResourceTypePlan, pid); err != nil {
		return nil, err
	}

	return p.model2Type(object)
}

// GetWithSubResources
// 获取 plan
// 获取 configs
// 获取 nodes
func (p *plan) GetWithSubResources(ctx context.Context, planId int64) (*types.Plan, error) {
	// 内部组装（免 owner 校验）：调用方 agent 已通过 agent token 鉴权，
	// API 层入口 getPlanWithSubResources 已先经 Get 完成 owner 校验。
	object, err := p.factory.Plan().Get(ctx, planId)
	if err != nil {
		klog.Errorf("failed to get plan %d: %v", planId, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrServerInternal
	}
	result, err := p.model2Type(object)
	if err != nil {
		return nil, err
	}

	// 追加配置
	cfg, err := p.Config().Get(ctx, planId)
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

func (p *plan) List(ctx context.Context, listOption types.ListOptions) (interface{}, error) {
	listOption.SetDefaultPageOption()

	pageResult := types.PageResult{
		PageRequest: types.PageRequest{
			Page:  listOption.Page,
			Limit: listOption.Limit,
		},
	}

	// 资源级授权：非超管用户叠加 scope 授权的 plan id（超管走 listOption.UserId 现状即可）
	authorizedPlanIDs, err := util.AuthorizedResourceIDs(ctx, p.factory, types.ResourceTypePlan)
	if err != nil {
		return nil, err
	}

	opts := []db.Options{
		db.WithUserOrResourceIDs(listOption.UserId, authorizedPlanIDs),
		db.WithNameLike(listOption.NameSelector),
	}

	pageResult.Total, err = p.factory.Plan().Count(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to count plans: %v", err)
		pageResult.Message = err.Error()
	}

	offset := (listOption.Page - 1) * listOption.Limit
	opts = append(opts, []db.Options{
		db.WithModifyOrderByDesc(),
		db.WithOffset(offset),
		db.WithLimit(listOption.Limit),
	}...)

	objects, err := p.factory.Plan().List(ctx, opts...)
	if err != nil {
		klog.Errorf("failed to get plans: %v", err)
		pageResult.Message = err.Error()
		return nil, errors.ErrServerInternal
	}

	ps := make([]types.Plan, 0, len(objects))
	for _, object := range objects {
		no, err := p.model2Type(&object)
		if err != nil {
			return nil, err
		}
		ps = append(ps, *no)
	}
	pageResult.Items = ps

	return pageResult, nil
}

// listAll 内部使用，返回所有计划（不带分页和过滤）
func (p *plan) listAll(ctx context.Context) ([]types.Plan, error) {
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
	plans, err := p.listAll(ctx)
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
	// 4. 校验运行任务
	isRunning, err := p.IsRunning(ctx, pid)
	if err != nil {
		return errors.ErrServerInternal
	}
	if isRunning {
		return errors.ErrNotAcceptable
	}

	// 1. 校验配置
	cfg, err := p.Config().Get(ctx, pid)
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
	runner, err := p.GetRunner(ctx, cfg.OSImage)
	if err != nil {
		return fmt.Errorf("获取 runner(%s) 失败 %v", cfg.OSImage, err)
	}
	klog.Infof("plan(%d) runner is %s", pid, runner)

	planObj, err := p.factory.Plan().Get(ctx, pid)
	if err != nil {
		return err
	}
	if planObj != nil && planObj.ExecMode == model.PlanExecModeAgent {
		if planObj.DeployAgentId == 0 {
			return fmt.Errorf("agent 模式未绑定执行 Agent")
		}
		agent, err := p.factory.Agent().Get(ctx, planObj.DeployAgentId)
		if err != nil {
			return err
		}
		if agent == nil {
			return fmt.Errorf("执行 Agent 不存在，可能已被删除")
		}
		if agent.Status == model.AgentStatusOffline {
			return fmt.Errorf("执行 Agent 已离线")
		}
	}
	return nil
}

// IsRunning
// 校验是否有任务正在运行
func (p *plan) IsRunning(ctx context.Context, planId int64) (bool, error) {
	object, err := p.factory.Plan().Get(ctx, planId)
	if err != nil {
		klog.Errorf("failed to get plan %d: %v", planId, err)
		return false, errors.ErrServerInternal
	}
	if object == nil {
		return false, errors.ErrServerInternal
	}

	switch object.Status {
	case model.RunningPlanStatus, model.DestroyingPlanStatus, model.StoppingPlanStatus:
		klog.Warningf("plan %d is running with status %s", planId, object.Status)
		return true, nil
	default:
		return false, nil
	}
}

// checkPlanAccess 校验当前用户是否有权操作该部署计划：超管放行 → owner 放行 → scope 命中放行。
func (p *plan) checkPlanAccess(ctx context.Context, pid int64) error {
	object, err := p.factory.Plan().Get(ctx, pid)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return errors.ErrPlanNotFound
		}
		klog.Errorf("get plan %d: %v", pid, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrPlanNotFound
	}
	return util.CheckResourceAccess(ctx, p.factory, object.UserId, types.ResourceTypePlan, pid)
}

func (p *plan) Start(ctx context.Context, pid int64) error {
	// 非超级管理员只能启动自己的部署计划或被 scope 授权的计划
	if err := p.checkPlanAccess(ctx, pid); err != nil {
		klog.Errorf("start plan(%d) access check failed: %v", pid, err)
		return err
	}
	// 启动前校验
	if err := p.preStart(ctx, pid); err != nil {
		klog.Errorf("pre-start check failed: %v", err)
		return err
	}

	taskQueue.Add(pid)
	return nil
}

// Destroy 全部设置成销毁
func (p *plan) Destroy(ctx context.Context, pid int64, restart bool) error {
	// 非超级管理员只能销毁自己的部署计划或被 scope 授权的计划
	if err := p.checkPlanAccess(ctx, pid); err != nil {
		klog.Errorf("destroy plan(%d) access check failed: %v", pid, err)
		return err
	}

	return nil
}

func (p *plan) model2Type(o *model.Plan) (*types.Plan, error) {
	return &types.Plan{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Name:              o.Name,
		Description:       o.Description,
		Step:              o.Status,
		KubernetesVersion: o.KubernetesVersion,
		NodeCount:         o.NodeCount,
		ExecMode:          o.ExecMode,
		DeployAgentId:     o.DeployAgentId,
	}, nil
}

// syncClusterToTunnel 将 plan 关联集群切换为隧道模式，并在缺少 token 时生成。
// TODO: 暂不实现隧道切换直连
func (p *plan) syncClusterToTunnel(ctx context.Context, planId int64) error {
	obj, err := p.factory.Cluster().GetBy(ctx, db.WithPlan(planId))
	if err != nil {
		return err
	}
	if obj == nil {
		return nil
	}

	updates := map[string]interface{}{"connect_mode": model.ConnectModeTunnel}
	if obj.AgentToken == "" {
		agentToken, tokenErr := token.Generate()
		if tokenErr != nil {
			return tokenErr
		}
		updates["agent_token"] = agentToken
	}
	return p.factory.Cluster().UpdateByPlan(ctx, planId, updates)
}

func NewPlan(cfg config.Config, f db.ShareDaoFactory) *plan {
	return &plan{
		cc:      cfg,
		factory: f,
	}
}
