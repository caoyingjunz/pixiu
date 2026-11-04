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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/planrender"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
	"github.com/caoyingjunz/pixiu/pkg/util/runtime"
	pixiutpl "github.com/caoyingjunz/pixiu/template"
)

const (
	actionBootstrap = "bootstrap-servers"
	actionAddNode   = "add-node"
	actionRunner    = "runner"
	actionRender    = "render"
	actionPrechecks = "prechecks"

	taskNamePrepare   = "前置准备"
	taskNameRender    = "配置渲染"
	taskNamePrechecks = "部署预检查"
	taskNameBootstrap = "初始化部署环境"
	taskNameAddNode   = "部署Node"

	// 旧扩容日志接口（?step=）仍以 bootstrap/add-node 作为步骤标识
	stepBootstrap = "bootstrap"
	stepAddNode   = "add-node"

	defaultExpansionTimeout = 30 * time.Minute
)

// expansionTaskSpecs 扩容进度任务定义，下发时按此顺序创建、worker 按此顺序执行。
var expansionTaskSpecs = []struct {
	Name   string
	Action string
}{
	{taskNamePrepare, actionRunner},
	{taskNameRender, actionRender},
	{taskNamePrechecks, actionPrechecks},
	{taskNameBootstrap, actionBootstrap},
	{taskNameAddNode, actionAddNode},
}

var (
	expansionQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cluster-expansions")
	clusterLocks   sync.Map // map[int64]*sync.Mutex
)

func lockCluster(clusterId int64) func() {
	value, _ := clusterLocks.LoadOrStore(clusterId, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (c *controller) upsertTask(ctx context.Context, planId int64, name, action string) error {
	_, err := c.factory.Plan().Task().GetByName(ctx, planId, name)
	if err == nil {
		_, err = c.factory.Plan().Task().Update(ctx, planId, name, map[string]interface{}{
			"status":       model.UnStartPlanStatus,
			"step":         model.UnStartedPlanStep,
			"action":       action,
			"message":      "",
			"gmt_create":   time.Now(),
			"gmt_modified": time.Now(),
		})
		if err != nil {
			if errors.IsRecordNotFound(err) {
				// 任务存在但更新值未变化时 Update 可能返回 not found；二次确认存在则视为成功
				if _, getErr := c.factory.Plan().Task().GetByName(ctx, planId, name); getErr == nil {
					return nil
				}
			}
			return err
		}
		return nil
	}
	if !errors.IsRecordNotFound(err) {
		return err
	}
	_, err = c.factory.Plan().Task().Create(ctx, &model.Task{
		Name:   name,
		PlanId: planId,
		Step:   model.UnStartedPlanStep,
		Action: action,
		Status: model.UnStartPlanStatus,
	})
	return err
}

func (c *controller) updateTask(ctx context.Context, planId int64, name string, updates map[string]interface{}) {
	if _, err := c.factory.Plan().Task().Update(ctx, planId, name, updates); err != nil {
		klog.Errorf("update plan(%d) task(%s): %v", planId, name, err)
	}
}

// markTaskRunning 将任务标记为进行中；planId 为 0 时（历史无计划扩容）跳过。
func (c *controller) markTaskRunning(ctx context.Context, planId int64, name string) {
	if planId == 0 {
		return
	}
	c.updateTask(ctx, planId, name, map[string]interface{}{
		"status": model.RunningPlanStatus, "step": model.RunningPlanStep, "message": "", "gmt_create": time.Now(),
	})
}

// finishTask 按执行结果结束任务；planId 为 0 时跳过。
func (c *controller) finishTask(ctx context.Context, planId int64, name string, cause error) {
	if planId == 0 {
		return
	}
	if cause != nil {
		c.updateTask(ctx, planId, name, map[string]interface{}{
			"status": model.FailedPlanStatus, "step": model.FailedPlanStep, "message": cause.Error(), "gmt_modified": time.Now(),
		})
		return
	}
	c.updateTask(ctx, planId, name, map[string]interface{}{
		"status": model.SuccessPlanStatus, "step": model.CompletedPlanStep, "message": "执行成功", "gmt_modified": time.Now(),
	})
}

func (c *controller) updatePlanStatus(ctx context.Context, planId int64, status model.TaskStatus) {
	if err := c.factory.Plan().UpdateBy(ctx, map[string]interface{}{"status": status}, db.WithId(planId)); err != nil {
		klog.Errorf("update plan(%d) status to %s: %v", planId, status, err)
	}
}

func (c *controller) Run(ctx context.Context, workers int) error {
	klog.Infof("starting cluster expansion manager")
	if err := c.syncStatus(ctx); err != nil {
		return err
	}
	if workers <= 0 {
		workers = 3
	}
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, time.Second)
	}
	return nil
}

func (c *controller) worker(ctx context.Context) {
	for c.process(ctx) {
	}
}

func (c *controller) process(ctx context.Context) bool {
	key, quit := expansionQueue.Get()
	if quit {
		return false
	}
	defer expansionQueue.Done(key)

	c.syncHandler(ctx, key.(int64))
	return true
}

// syncStatus 进程启动时的状态恢复：未完成/运行中的扩容标记失败，添加中的节点标记失败。
func (c *controller) syncStatus(ctx context.Context) error {
	now := time.Now()
	for _, status := range []model.TaskStatus{model.UnStartPlanStatus, model.RunningPlanStatus} {
		expansions, err := c.factory.ClusterExpansion().List(ctx, db.WithTaskStatus(status))
		if err != nil {
			return err
		}
		for i := range expansions {
			if err = c.factory.ClusterExpansion().Update(ctx, expansions[i].Id, map[string]interface{}{
				"status":       model.FailedPlanStatus,
				"step":         "完成",
				"message":      "服务异常中断，请重新发起扩容",
				"completed_at": now,
			}); err != nil {
				return err
			}
			// 服务重启中断时任务可能仍停留在「未开始」，补一条失败任务用于展示原因
			c.markFirstPendingTaskFailed(ctx, expansions[i].PlanId, "服务异常中断，请重新发起扩容")
		}
	}

	nodes, err := c.factory.Plan().Node().List(ctx, db.WithNodeStatus(model.NodeStatusAdding))
	if err != nil {
		return err
	}
	for i := range nodes {
		if err = c.factory.Plan().Node().UpdateFields(ctx, nodes[i].Id, map[string]interface{}{
			"status":         model.NodeStatusFailed,
			"status_message": "服务异常中断，请重试",
		}); err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) syncHandler(ctx context.Context, expansionId int64) {
	expansion, err := c.factory.ClusterExpansion().Get(ctx, expansionId)
	if err != nil {
		klog.Errorf("get cluster expansion(%d): %v", expansionId, err)
		return
	}
	if expansion.Status != model.UnStartPlanStatus && expansion.Status != model.RunningPlanStatus {
		return
	}

	unlock := lockCluster(expansion.ClusterId)
	defer unlock()

	now := time.Now()
	_ = c.factory.ClusterExpansion().Update(ctx, expansion.Id, map[string]interface{}{
		"status":     model.RunningPlanStatus,
		"step":       "初始化",
		"message":    "",
		"started_at": now,
	})

	clusterObj, err := c.factory.Cluster().Get(ctx, expansion.ClusterId)
	if err != nil || clusterObj == nil {
		c.failExpansion(ctx, expansion.Id, nil, fmt.Errorf("集群不存在或已被删除"))
		return
	}

	nodes, err := c.loadNodes(ctx, expansion)
	if err != nil {
		c.failExpansion(ctx, expansion.Id, nil, err)
		return
	}

	var options types.ExpansionOptions
	if expansion.Options != "" {
		if err = json.Unmarshal([]byte(expansion.Options), &options); err != nil {
			c.failExpansion(ctx, expansion.Id, nodes, fmt.Errorf("解析扩容参数失败: %v", err))
			return
		}
	}

	info := c.detectCluster(ctx, clusterObj)
	if !info.Config.Supported {
		// 不支持扩容时直接失败，避免继续拉取镜像与渲染配置
		c.failExpansion(ctx, expansion.Id, nodes, fmt.Errorf("%s", info.Config.Reason))
		return
	}

	if expansion.PlanId != 0 {
		for _, spec := range expansionTaskSpecs {
			if err = c.upsertTask(ctx, expansion.PlanId, spec.Name, spec.Action); err != nil {
				c.failExpansion(ctx, expansion.Id, nodes, fmt.Errorf("初始化扩容任务失败: %v", err))
				return
			}
		}
		c.updatePlanStatus(ctx, expansion.PlanId, model.RunningPlanStatus)
	}

	timeout := time.Duration(c.cc.Worker.DeployTimeout) * time.Second
	if timeout <= 0 {
		timeout = defaultExpansionTimeout
	}

	// 1) 前置准备：解析 runner 镜像
	c.markTaskRunning(ctx, expansion.PlanId, taskNamePrepare)
	runner, err := c.resolveRunner(ctx, options.OSImage)
	if err != nil {
		c.finishTask(ctx, expansion.PlanId, taskNamePrepare, err)
		c.failExpansion(ctx, expansion.Id, nodes, err)
		return
	}
	c.finishTask(ctx, expansion.PlanId, taskNamePrepare, nil)

	// 2) 配置渲染：解析 join 信息并渲染节点配置
	c.markTaskRunning(ctx, expansion.PlanId, taskNameRender)
	join, err := c.resolveJoin(ctx, clusterObj, info, &options)
	if err != nil {
		c.finishTask(ctx, expansion.PlanId, taskNameRender, err)
		c.failExpansion(ctx, expansion.Id, nodes, err)
		return
	}

	// 与部署任务保持一致：使用 plan id 作为渲染目录；历史无 plan_id 的记录回退 expansion id
	planId := expansion.PlanId
	if planId <= 0 {
		planId = expansion.Id
	}
	dir := filepath.Join(c.cc.Worker.WorkDir, strconv.FormatInt(planId, 10))

	planNodes, err := nodesToPlanNodes(nodes)
	if err != nil {
		c.finishTask(ctx, expansion.PlanId, taskNameRender, err)
		c.failExpansion(ctx, expansion.Id, nodes, err)
		return
	}
	globals := pixiutpl.ExpandGlobals{
		KubeRelease:      join.KubeRelease,
		APIServer:        join.APIServer,
		Token:            join.Token,
		CAHash:           join.CAHash,
		NetworkInterface: options.NetworkInterface,
		SetHostname:      options.SetHostname,
		RuntimeDataDir:   options.RuntimeDataDir,
		EnableCustomRepo: options.CustomRepo != nil && options.CustomRepo.Enable,
	}
	// 重新触发扩容时清理旧渲染产物，保证 plan 目录内容为本次最新渲染结果
	if rmErr := os.RemoveAll(dir); rmErr != nil {
		renderErr := fmt.Errorf("清理扩容渲染目录失败: %v", rmErr)
		c.finishTask(ctx, expansion.PlanId, taskNameRender, renderErr)
		c.failExpansion(ctx, expansion.Id, nodes, renderErr)
		return
	}
	if err = planrender.RenderExpand(dir, planNodes, globals, options.CustomRepo); err != nil {
		renderErr := fmt.Errorf("渲染扩容配置失败: %v", err)
		c.finishTask(ctx, expansion.PlanId, taskNameRender, renderErr)
		c.failExpansion(ctx, expansion.Id, nodes, renderErr)
		return
	}
	c.finishTask(ctx, expansion.PlanId, taskNameRender, nil)

	// 3) 部署预检查：确认集群可达且支持扩容
	c.markTaskRunning(ctx, expansion.PlanId, taskNamePrechecks)
	var precheckErr error
	if info.Client == nil || !info.Config.Supported {
		reason := info.Config.Reason
		if reason == "" {
			reason = "集群不可达"
		}
		precheckErr = fmt.Errorf("%s", reason)
	}
	c.finishTask(ctx, expansion.PlanId, taskNamePrechecks, precheckErr)
	if precheckErr != nil {
		c.failExpansion(ctx, expansion.Id, nodes, precheckErr)
		return
	}

	// 4) 初始化部署环境：bootstrap-servers 为必经步骤（kubez-ansible 同名 action）
	var runErr error
	_ = c.factory.ClusterExpansion().Update(ctx, expansion.Id, map[string]interface{}{"step": taskNameBootstrap})
	c.markTaskRunning(ctx, expansion.PlanId, taskNameBootstrap)
	runErr = runRunnerContainer(ctx, fmt.Sprintf("%s-%d", actionBootstrap, expansion.PlanId), actionBootstrap, runner, dir, timeout)
	c.finishTask(ctx, expansion.PlanId, taskNameBootstrap, runErr)

	// 5) 加入集群
	joined := 0
	if runErr == nil {
		_ = c.factory.ClusterExpansion().Update(ctx, expansion.Id, map[string]interface{}{"step": taskNameAddNode})
		c.markTaskRunning(ctx, expansion.PlanId, taskNameAddNode)
		runErr = runRunnerContainer(ctx, fmt.Sprintf("%s-%d", actionAddNode, expansion.PlanId), actionAddNode, runner, dir, timeout)
		c.finishTask(ctx, expansion.PlanId, taskNameAddNode, runErr)
	}

	// 无论成败都按集群实时节点列表校正，保证已 join 的节点不误报失败
	joined, reconcileErr := c.reconcileNodes(ctx, info.Client, nodes)
	if runErr == nil {
		runErr = reconcileErr
	}

	completed := time.Now()
	if runErr != nil {
		_ = c.factory.ClusterExpansion().Update(ctx, expansion.Id, map[string]interface{}{
			"status":       model.FailedPlanStatus,
			"step":         "完成",
			"message":      runErr.Error(),
			"completed_at": completed,
		})
		if expansion.PlanId != 0 {
			c.updatePlanStatus(ctx, expansion.PlanId, model.FailedPlanStatus)
		}
		klog.Errorf("cluster(%d) expansion(%d) failed: %v", expansion.ClusterId, expansion.Id, runErr)
		return
	}
	if joined != len(nodes) {
		message := fmt.Sprintf("扩容未完全成功：%d/%d 个节点已加入集群", joined, len(nodes))
		_ = c.factory.ClusterExpansion().Update(ctx, expansion.Id, map[string]interface{}{
			"status":       model.FailedPlanStatus,
			"step":         "完成",
			"message":      message,
			"completed_at": completed,
		})
		if expansion.PlanId != 0 {
			c.updateTask(ctx, expansion.PlanId, taskNameAddNode, map[string]interface{}{
				"status": model.FailedPlanStatus, "step": model.FailedPlanStep, "message": message, "gmt_modified": time.Now(),
			})
			c.updatePlanStatus(ctx, expansion.PlanId, model.FailedPlanStatus)
		}
		return
	}

	_ = c.factory.ClusterExpansion().Update(ctx, expansion.Id, map[string]interface{}{
		"status":       model.SuccessPlanStatus,
		"step":         "完成",
		"message":      "扩容成功",
		"completed_at": completed,
	})
	if expansion.PlanId != 0 {
		c.updatePlanStatus(ctx, expansion.PlanId, model.SuccessPlanStatus)
	}
	klog.Infof("cluster(%d) expansion(%d) succeeded", expansion.ClusterId, expansion.Id)
}

func (c *controller) loadNodes(ctx context.Context, expansion *model.ClusterExpansion) ([]model.Node, error) {
	var ids []int64
	if expansion.NodeIds != "" {
		if err := json.Unmarshal([]byte(expansion.NodeIds), &ids); err != nil {
			return nil, fmt.Errorf("解析扩容节点失败: %v", err)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("扩容节点为空")
	}

	nodes, err := c.factory.Plan().Node().List(ctx, db.WithIdIn(ids))
	if err != nil {
		return nil, err
	}
	if len(nodes) != len(ids) {
		return nil, fmt.Errorf("部分扩容节点已不存在")
	}
	for i := range nodes {
		if nodes[i].ClusterId != expansion.ClusterId {
			return nil, fmt.Errorf("节点 %s 不属于该集群", nodes[i].Name)
		}
		// 扩容节点应挂在集群对应的扩容计划（type=1）下；plan_id=0 的历史节点仍按 cluster_id 判定
		if expansion.PlanId != 0 && nodes[i].PlanId != 0 && nodes[i].PlanId != expansion.PlanId {
			return nil, fmt.Errorf("节点 %s 不属于该集群的扩容计划", nodes[i].Name)
		}
	}
	return nodes, nil
}

func nodesToPlanNodes(nodes []model.Node) ([]types.PlanNode, error) {
	result := make([]types.PlanNode, 0, len(nodes))
	for i := range nodes {
		auth := types.PlanNodeAuth{}
		if err := auth.Unmarshal(nodes[i].Auth); err != nil {
			return nil, err
		}
		result = append(result, types.PlanNode{
			Name: nodes[i].Name,
			Ip:   nodes[i].Ip,
			Role: []string{model.NodeRole},
			CRI:  model.ContainerdCRI,
			Auth: auth,
		})
	}
	return result, nil
}

func runRunnerContainer(ctx context.Context, name, command, image, configDir string, timeout time.Duration) error {
	rt, err := runtime.Default()
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout+60*time.Second)
	defer cancel()

	return rt.RunContainer(runCtx, &runtime.ContainerSpec{
		Name:        name,
		Image:       image,
		Env:         []string{fmt.Sprintf("COMMAND=%s", command)},
		Binds:       []string{fmt.Sprintf("%s:/configs", configDir)},
		NetworkHost: true,
		WaitTimeout: timeout,
		Labels:      map[string]string{"author": "caoyingjunz", "pixiuExpand": name},
	})
}

// reconcileNodes 按集群实时节点列表校正扩容节点状态，返回已加入集群的节点数。
func (c *controller) reconcileNodes(ctx context.Context, cs *client.ClusterSet, nodes []model.Node) (int, error) {
	names, ips, err := clusterNodeAddresses(ctx, cs)
	if err != nil {
		for i := range nodes {
			_ = c.factory.Plan().Node().UpdateFields(ctx, nodes[i].Id, map[string]interface{}{
				"status":         model.NodeStatusFailed,
				"status_message": "无法连接集群确认节点状态，请检查日志",
			})
		}
		return 0, fmt.Errorf("查询集群节点失败: %v", err)
	}

	joined := 0
	for i := range nodes {
		node := &nodes[i]
		if nodeMatched(node.Name, node.Ip, names, ips) {
			joined++
			// 已加入集群的节点从扩容计划解绑（保留 cluster_id 供节点管理页展示）
			_ = c.factory.Plan().Node().UpdateFields(ctx, node.Id, map[string]interface{}{
				"plan_id":        int64(0),
				"status":         model.NodeStatusAdded,
				"status_message": "",
			})
			continue
		}
		_ = c.factory.Plan().Node().UpdateFields(ctx, node.Id, map[string]interface{}{
			"status":         model.NodeStatusFailed,
			"status_message": "节点未能加入集群，请查看扩容日志",
		})
	}
	return joined, nil
}

// markFirstPendingTaskFailed 在没有失败任务时把第一个「未开始」任务标记为失败，
// 保证早期失败、服务重启中断等场景下进度抽屉有失败行可展示原因。
func (c *controller) markFirstPendingTaskFailed(ctx context.Context, planId int64, message string) {
	if planId == 0 {
		return
	}
	tasks, err := c.factory.Plan().Task().List(ctx, planId)
	if err != nil {
		klog.Errorf("list plan(%d) tasks: %v", planId, err)
		return
	}
	for _, task := range tasks {
		if task.Status == model.FailedPlanStatus {
			return
		}
	}
	for _, task := range tasks {
		if task.Status == model.UnStartPlanStatus {
			c.updateTask(ctx, planId, task.Name, map[string]interface{}{
				"status": model.FailedPlanStatus, "step": model.FailedPlanStep,
				"message": message, "gmt_modified": time.Now(),
			})
			return
		}
	}
}

func (c *controller) failExpansion(ctx context.Context, expansionId int64, nodes []model.Node, cause error) {
	now := time.Now()
	_ = c.factory.ClusterExpansion().Update(ctx, expansionId, map[string]interface{}{
		"status":       model.FailedPlanStatus,
		"step":         "完成",
		"message":      cause.Error(),
		"completed_at": now,
	})

	exp, getErr := c.factory.ClusterExpansion().Get(ctx, expansionId)
	// 早期失败（节点尚未加载）时按扩容记录反查节点，避免「添加中」状态残留
	if getErr == nil && exp != nil && len(nodes) == 0 && exp.NodeIds != "" {
		var ids []int64
		if json.Unmarshal([]byte(exp.NodeIds), &ids) == nil && len(ids) > 0 {
			nodes, _ = c.factory.Plan().Node().List(ctx, db.WithIdIn(ids))
		}
	}
	for i := range nodes {
		_ = c.factory.Plan().Node().UpdateFields(ctx, nodes[i].Id, map[string]interface{}{
			"status":         model.NodeStatusFailed,
			"status_message": cause.Error(),
		})
	}
	if getErr == nil && exp != nil && exp.PlanId != 0 {
		c.updatePlanStatus(ctx, exp.PlanId, model.FailedPlanStatus)
		c.markFirstPendingTaskFailed(ctx, exp.PlanId, cause.Error())
	}
	klog.Errorf("cluster expansion(%d) failed: %v", expansionId, cause)
}
